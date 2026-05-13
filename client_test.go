package exclusivenetworks

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientReauthsOn401 covers the do() path that refreshes a stale token
// when the server returns 401 despite our cached-expiry check.
func TestClientReauthsOn401(t *testing.T) {
	const (
		validToken   = "valid-token"
		expiredToken = "expired-token"
	)
	var tokenCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		count := tokenCalls.Add(1)
		token := expiredToken
		if count > 1 {
			token = validToken
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token": "` + token + `", "token_type": "Bearer", "expires_in": 3600}`))
	})
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer "+expiredToken {
			http.Error(w, "stale", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"salesQuotes": []}}`))
	})
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	_, err := client.GetQuoteByNumber(context.Background(), "anything")
	require.ErrorIs(t, err, ErrQuoteNotFound) // empty array → not found, but the reauth path was exercised
	assert.GreaterOrEqual(t, tokenCalls.Load(), int32(2), "expected token refresh after 401")
}

// TestClientRetriesOn5xx covers the retry-with-backoff path. WithRetry(2, …)
// gives one attempt + one retry.
func TestClientRetriesOn5xx(t *testing.T) {
	var graphqlCalls atomic.Int32

	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, _ *http.Request) {
		n := graphqlCalls.Add(1)
		if n == 1 {
			http.Error(w, "transient", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"salesQuotes": []}}`))
	})
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "",
		WithRetry(2, 1*time.Millisecond))
	_, err := client.GetQuoteByNumber(context.Background(), "anything")
	require.ErrorIs(t, err, ErrQuoteNotFound)
	assert.Equal(t, int32(2), graphqlCalls.Load())
}

// TestClientHonorsRetryAfter checks that the Retry-After header from a 429
// response overrides the exponential backoff.
func TestClientHonorsRetryAfter(t *testing.T) {
	var graphqlCalls atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, _ *http.Request) {
		n := graphqlCalls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"salesQuotes": []}}`))
	})
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "",
		WithRetry(2, 30*time.Second)) // base backoff would normally be 30s; Retry-After 0 should override
	start := time.Now()
	_, err := client.GetQuoteByNumber(context.Background(), "anything")
	require.ErrorIs(t, err, ErrQuoteNotFound)
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 5*time.Second, "Retry-After: 0 should override the 30s backoff (got %s)", elapsed)
	assert.Equal(t, int32(2), graphqlCalls.Load())
}

// TestClientGivesUpAfterMaxAttempts ensures non-retryable + retry exhaustion
// surface the last APIError.
func TestClientGivesUpAfterMaxAttempts(t *testing.T) {
	var graphqlCalls atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, _ *http.Request) {
		graphqlCalls.Add(1)
		http.Error(w, "boom", http.StatusBadGateway)
	})
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "",
		WithRetry(3, 1*time.Millisecond))
	_, err := client.GetQuoteByNumber(context.Background(), "anything")
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok, "want *APIError, got %T", err)
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Equal(t, int32(3), graphqlCalls.Load())
}
