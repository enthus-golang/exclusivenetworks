package exclusivenetworks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Client is an Exclusive Networks AccessNow GraphQL API client. It is safe
// for concurrent use by multiple goroutines.
type Client struct {
	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	scope        string
	customerID   string

	httpClient  *http.Client
	minLimiter  *rate.Limiter
	hourLimiter *rate.Limiter

	maxAttempts int
	baseBackoff time.Duration
	logger      Logger

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

// New creates an Exclusive Networks AccessNow API client.
//
// baseURL is the GraphQL endpoint. tokenURL is the OAuth2 token endpoint
// that issues client_credentials tokens. clientID, clientSecret, and
// scope are the OAuth2 credentials provisioned by Exclusive Networks.
// All four URLs/credentials are issued by Exclusive Networks on approval.
//
// All five arguments are required; this constructor does not validate them
// — instead, the first call needing them surfaces any errors.
func New(baseURL, tokenURL, clientID, clientSecret, scope string, opts ...Option) *Client {
	c := &Client{
		baseURL:      baseURL,
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		scope:        scope,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		maxAttempts:  3,
		baseBackoff:  500 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// graphqlRequest is the wire shape for a single GraphQL HTTP POST body.
type graphqlRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
}

// graphqlResponse is the wire shape for a GraphQL response. Data is parsed
// later by the caller after we strip the envelope.
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// do issues an authenticated GraphQL POST. The query is sent as
// {"query": query, "operationName": opName, "variables": variables}.
// When the client has a configured customerID, it is injected into
// variables as OIC_AUTHENTICATED_CUSTOMER (unless the caller already
// supplied it). The data envelope is decoded into out; any GraphQL errors
// surface as *GraphQLErrors.
func (c *Client) do(ctx context.Context, opName, query string, variables map[string]any, out any) error {
	if c.minLimiter != nil {
		if err := c.minLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("exclusivenetworks: rate limit wait: %w", err)
		}
	}
	if c.hourLimiter != nil {
		if err := c.hourLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("exclusivenetworks: rate limit wait: %w", err)
		}
	}

	if err := c.authenticate(ctx); err != nil {
		return err
	}

	if c.customerID != "" {
		if variables == nil {
			variables = make(map[string]any, 1)
		}
		if _, present := variables["OIC_AUTHENTICATED_CUSTOMER"]; !present {
			variables["OIC_AUTHENTICATED_CUSTOMER"] = c.customerID
		}
	}

	body, err := json.Marshal(graphqlRequest{
		Query:         query,
		OperationName: opName,
		Variables:     variables,
	})
	if err != nil {
		return fmt.Errorf("exclusivenetworks: marshal request: %w", err)
	}

	authRetried := false
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("exclusivenetworks: build request: %w", err)
		}
		c.tokenMu.Lock()
		req.Header.Set("Authorization", "Bearer "+c.token)
		c.tokenMu.Unlock()
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			lastErr = fmt.Errorf("exclusivenetworks: request: %w", doErr)
			if !c.shouldRetry(attempt, 0, true) {
				return lastErr
			}
			c.sleepBeforeRetry(ctx, attempt, nil)
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized && !authRetried {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			authRetried = true
			c.invalidateToken()
			if err := c.authenticate(ctx); err != nil {
				return err
			}
			attempt-- // do not count the reauth toward maxAttempts
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			apiErr := &APIError{StatusCode: resp.StatusCode, Body: string(body)}
			lastErr = apiErr
			if !c.shouldRetry(attempt, resp.StatusCode, false) {
				return apiErr
			}
			c.sleepBeforeRetry(ctx, attempt, resp)
			continue
		}

		return c.decodeGraphQL(resp, out)
	}
	return lastErr
}

// decodeGraphQL reads a 2xx GraphQL response, unwraps the envelope, and
// decodes the "data" field into out. A non-empty "errors" array surfaces
// as *GraphQLErrors regardless of HTTP status.
func (c *Client) decodeGraphQL(resp *http.Response, out any) error {
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("exclusivenetworks: read response: %w", err)
	}

	var env graphqlResponse
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("exclusivenetworks: decode envelope: %w", err)
	}
	if len(env.Errors) > 0 {
		return &GraphQLErrors{Errors: env.Errors}
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("exclusivenetworks: decode data: %w", err)
	}
	return nil
}

func (c *Client) shouldRetry(attempt, statusCode int, networkErr bool) bool {
	if attempt >= c.maxAttempts {
		return false
	}
	if networkErr {
		return true
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode >= 500 && statusCode <= 599 {
		return true
	}
	return false
}

func (c *Client) sleepBeforeRetry(ctx context.Context, attempt int, resp *http.Response) {
	d := c.computeBackoff(attempt)
	if resp != nil {
		if v := resp.Header.Get("Retry-After"); v != "" {
			if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
				d = time.Duration(secs) * time.Second
			}
		}
	}
	if c.logger != nil {
		c.logger.Printf("exclusivenetworks: retry attempt %d after %s", attempt, d)
	}
	c.sleepFor(ctx, d)
}

func (c *Client) computeBackoff(attempt int) time.Duration {
	const cap = 30 * time.Second
	if c.baseBackoff <= 0 {
		return 0
	}
	d := c.baseBackoff << uint(attempt-1)
	if d <= 0 || d > cap {
		d = cap
	}
	jitter := time.Duration(rand.Int64N(int64(d)/2+1)) - d/4
	return d + jitter
}

func (c *Client) sleepFor(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
