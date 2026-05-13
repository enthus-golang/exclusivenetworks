package exclusivenetworks

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticateSuccess(t *testing.T) {
	const (
		clientID     = "id"
		clientSecret = "secret"
		scope        = "sales:read"
		accessToken  = "token-abc"
	)
	var tokenCalls int
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, clientID, clientSecret, scope, accessToken, 3600, &tokenCalls))
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", clientID, clientSecret, scope)
	require.NoError(t, client.authenticate(context.Background()))
	assert.Equal(t, accessToken, client.token)
	assert.WithinDuration(t, time.Now().Add(time.Hour), client.tokenExpiry, 5*time.Second)
	assert.Equal(t, 1, tokenCalls)

	// Second call within leeway window must reuse the cached token.
	require.NoError(t, client.authenticate(context.Background()))
	assert.Equal(t, 1, tokenCalls, "second call should reuse cached token")
}

func TestAuthenticateRefreshesNearExpiry(t *testing.T) {
	var tokenCalls int
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "token", 30, &tokenCalls)) // 30s TTL, well below leeway
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	require.NoError(t, client.authenticate(context.Background()))
	assert.Equal(t, 1, tokenCalls)

	// 30s expiry is within tokenRefreshLeeway (60s), so the next call refreshes.
	require.NoError(t, client.authenticate(context.Background()))
	assert.Equal(t, 2, tokenCalls)
}

func TestAuthenticateBadCredentials(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "token", 3600, nil))
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "wrong-id", "secret", "")
	err := client.authenticate(context.Background())
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok, "want *APIError, got %T", err)
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
}

func TestAuthenticateMissingAccessToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token": "", "token_type": "Bearer", "expires_in": 3600}`))
	})
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	err := client.authenticate(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing access_token")
}

func TestAuthenticateMissingExpiresIn(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token": "t", "token_type": "Bearer", "expires_in": 0}`))
	})
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	err := client.authenticate(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expires_in")
}

func TestInvalidateToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "token", 3600, nil))
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	require.NoError(t, client.authenticate(context.Background()))
	assert.NotEmpty(t, client.token)
	client.invalidateToken()
	assert.Empty(t, client.token)
	assert.True(t, client.tokenExpiry.IsZero())
}
