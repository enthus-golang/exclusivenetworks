package exclusivenetworks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// tokenRefreshLeeway is how far before the cached token's expiry we
// proactively refresh.
const tokenRefreshLeeway = 60 * time.Second

// authenticate ensures c.token is valid for at least tokenRefreshLeeway from
// now. It POSTs an OAuth2 client_credentials request when the cached token
// is missing or near expiry. Concurrent callers serialize on c.tokenMu —
// only one HTTP exchange happens at a time.
func (c *Client) authenticate(ctx context.Context) error {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && time.Now().Add(tokenRefreshLeeway).Before(c.tokenExpiry) {
		return nil
	}

	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "client_credentials")
	if c.scope != "" {
		form.Set("scope", c.scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("exclusivenetworks: build auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("exclusivenetworks: auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var authResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("exclusivenetworks: decode auth response: %w", err)
	}
	if authResp.AccessToken == "" {
		return fmt.Errorf("exclusivenetworks: auth response missing access_token")
	}
	if authResp.ExpiresIn <= 0 {
		return fmt.Errorf("exclusivenetworks: auth response missing or invalid expires_in")
	}

	c.token = authResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)
	return nil
}

func (c *Client) invalidateToken() {
	c.tokenMu.Lock()
	c.token = ""
	c.tokenExpiry = time.Time{}
	c.tokenMu.Unlock()
}
