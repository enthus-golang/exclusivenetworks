package exclusivenetworks

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// tokenHandler responds to the OAuth2 token endpoint with a fixed
// access token + expires_in when the form credentials match. tokenCalls,
// if non-nil, is incremented on every invocation (handy for asserting
// auth-call counts).
func tokenHandler(t *testing.T, clientID, clientSecret, scope, accessToken string, expiresIn int64, tokenCalls *int) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if tokenCalls != nil {
			*tokenCalls++
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("client_id") != clientID || r.PostForm.Get("client_secret") != clientSecret {
			http.Error(w, "bad creds", http.StatusUnauthorized)
			return
		}
		if r.PostForm.Get("grant_type") != "client_credentials" {
			http.Error(w, "bad grant_type", http.StatusBadRequest)
			return
		}
		if scope != "" && r.PostForm.Get("scope") != scope {
			http.Error(w, "bad scope", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": accessToken,
			"token_type":   "Bearer",
			"expires_in":   expiresIn,
		})
	}
}

// startServer wires the given mux onto an httptest.Server with cleanup
// registered against t.
func startServer(t *testing.T, mux *http.ServeMux) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// readGraphQLRequest parses the JSON body of an incoming GraphQL POST.
func readGraphQLRequest(t *testing.T, r *http.Request) graphqlRequest {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var req graphqlRequest
	require.NoError(t, json.Unmarshal(body, &req))
	return req
}
