package exclusivenetworks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGraphQLHandler returns the given response body verbatim for any POST
// to the GraphQL endpoint. It records the last request seen for assertions.
type fakeGraphQLHandler struct {
	lastRequest graphqlRequest
	response    string
	statusCode  int
}

func (f *fakeGraphQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Authorization") == "" {
		http.Error(w, "missing auth", http.StatusUnauthorized)
		return
	}
	body, _ := json.Marshal(struct {
		Q string         `json:"query"`
		O string         `json:"operationName,omitempty"`
		V map[string]any `json:"variables,omitempty"`
	}{})
	_ = body // suppress unused
	f.lastRequest = readGraphQLRequestFrom(r)
	status := f.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(f.response))
}

func readGraphQLRequestFrom(r *http.Request) graphqlRequest {
	defer func() { _ = r.Body.Close() }()
	var req graphqlRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req
}

func TestGetQuoteByNumberHappyPath(t *testing.T) {
	handler := &fakeGraphQLHandler{
		response: `{
			"data": {
				"salesQuotes": [
					{
						"id": "Q1",
						"quoteNumber": "QPL010006170",
						"version": 1,
						"isLatestVersion": false,
						"status": "Closed",
						"lines": []
					},
					{
						"id": "Q2",
						"quoteNumber": "QPL010006170",
						"version": 2,
						"isLatestVersion": true,
						"status": "Open",
						"vendor": "Palo Alto Networks",
						"lines": [
							{
								"id": "QL1",
								"itemName": "PA-220",
								"vendorPartNumber": "PA-220-LIC",
								"vendor": "Palo Alto Networks",
								"serialNumberSupported": "SN-001",
								"contractStartDate": "2026-01-01",
								"contractEndDate": "2027-12-31",
								"itemType": "Hardware"
							},
							{
								"id": "QL2",
								"itemName": "Description",
								"vendorPartNumber": "Description",
								"description": "Annotation"
							}
						]
					}
				]
			}
		}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.Handle("/graphql", handler)
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "",
		WithCustomerID("CUST-99"))
	quote, err := client.GetQuoteByNumber(context.Background(), "QPL010006170")
	require.NoError(t, err)
	require.NotNil(t, quote)

	assert.Equal(t, "Q2", quote.ID)
	assert.True(t, quote.IsLatestVersion)
	assert.Equal(t, 2, quote.Version)
	require.Len(t, quote.Lines, 2)
	assert.Equal(t, "PA-220", quote.Lines[0].ItemName)
	assert.Equal(t, "SN-001", quote.Lines[0].SerialNumberSupported)
	assert.Equal(t, 2026, quote.Lines[0].ContractStartDate.Year())
	assert.Equal(t, 2027, quote.Lines[0].ContractEndDate.Year())

	// Customer ID injected as variable.
	assert.Equal(t, "CUST-99", handler.lastRequest.Variables["OIC_AUTHENTICATED_CUSTOMER"])
	assert.Equal(t, "QPL010006170", handler.lastRequest.Variables["number"])
	assert.Equal(t, "FindQuoteByNumber", handler.lastRequest.OperationName)
}

func TestGetQuoteByNumberNotFound(t *testing.T) {
	handler := &fakeGraphQLHandler{
		response: `{"data": {"salesQuotes": []}}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.Handle("/graphql", handler)
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	_, err := client.GetQuoteByNumber(context.Background(), "Q-MISSING")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrQuoteNotFound), "want ErrQuoteNotFound, got %v", err)
}

func TestGetQuoteByNumberOnlyOldVersionsTreatedAsNotFound(t *testing.T) {
	handler := &fakeGraphQLHandler{
		response: `{
			"data": {
				"salesQuotes": [
					{"id": "Q1", "quoteNumber": "Q1", "version": 1, "isLatestVersion": false, "lines": []},
					{"id": "Q2", "quoteNumber": "Q1", "version": 2, "isLatestVersion": false, "lines": []}
				]
			}
		}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.Handle("/graphql", handler)
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	_, err := client.GetQuoteByNumber(context.Background(), "Q1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrQuoteNotFound))
}

func TestGetQuoteByNumberAmbiguous(t *testing.T) {
	handler := &fakeGraphQLHandler{
		response: `{
			"data": {
				"salesQuotes": [
					{"id": "Q1", "quoteNumber": "Q1", "version": 1, "isLatestVersion": true, "lines": []},
					{"id": "Q1B", "quoteNumber": "Q1", "version": 1, "isLatestVersion": true, "lines": []}
				]
			}
		}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.Handle("/graphql", handler)
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	_, err := client.GetQuoteByNumber(context.Background(), "Q1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAmbiguousQuoteNumber))
}

func TestGetQuoteByNumberGraphQLError(t *testing.T) {
	handler := &fakeGraphQLHandler{
		response: `{
			"errors": [
				{"message": "field 'salesQuotes' not found"},
				{"message": "permission denied"}
			]
		}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.Handle("/graphql", handler)
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "")
	_, err := client.GetQuoteByNumber(context.Background(), "anything")
	require.Error(t, err)
	var gqlErr *GraphQLErrors
	require.True(t, errors.As(err, &gqlErr), "want *GraphQLErrors, got %T", err)
	require.Len(t, gqlErr.Errors, 2)
	assert.Contains(t, gqlErr.Error(), "field 'salesQuotes' not found")
	assert.Contains(t, gqlErr.Error(), "permission denied")
}

func TestGetQuoteByNumberHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/token", tokenHandler(t, "id", "secret", "", "tok", 3600, nil))
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	server := startServer(t, mux)

	client := New(server.URL+"/graphql", server.URL+"/token", "id", "secret", "",
		WithRetry(1, 0)) // disable retries so we see the error fast
	_, err := client.GetQuoteByNumber(context.Background(), "anything")
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok, "want *APIError, got %T", err)
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
}
