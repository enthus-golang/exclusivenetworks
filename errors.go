package exclusivenetworks

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Sentinel errors. Match with errors.Is.
var (
	// ErrQuoteNotFound is returned by GetQuoteByNumber when the search
	// yields no quote (or no latest-version quote) for the requested
	// quote number.
	ErrQuoteNotFound = errors.New("exclusivenetworks: quote not found")

	// ErrAmbiguousQuoteNumber is returned by GetQuoteByNumber when the
	// search yields more than one row with IsLatestVersion == true for
	// the same quoteNumber. This indicates upstream data inconsistency.
	ErrAmbiguousQuoteNumber = errors.New("exclusivenetworks: ambiguous quote number")

	// ErrUnauthorized matches HTTP 401/403 responses via errors.Is.
	ErrUnauthorized = errors.New("exclusivenetworks: unauthorized")

	// ErrNotFound matches HTTP 404 responses via errors.Is.
	ErrNotFound = errors.New("exclusivenetworks: not found")
)

// APIError carries the HTTP status code and raw response body for an
// unsuccessful HTTP-level API call.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("exclusivenetworks: HTTP %d: %s", e.StatusCode, e.Body)
}

// Is allows callers to match ErrUnauthorized / ErrNotFound via errors.Is.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	}
	return false
}

// GraphQLError is a single entry from a GraphQL response's "errors" array.
type GraphQLError struct {
	Message    string         `json:"message"`
	Path       []any          `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// GraphQLErrors aggregates one or more GraphQL errors returned alongside a
// 200 OK response. The first message is surfaced for brevity; the rest are
// available via Errors.
type GraphQLErrors struct {
	Errors []GraphQLError
}

func (e *GraphQLErrors) Error() string {
	if len(e.Errors) == 0 {
		return "exclusivenetworks: empty GraphQL error envelope"
	}
	msgs := make([]string, len(e.Errors))
	for i, ge := range e.Errors {
		msgs[i] = ge.Message
	}
	return "exclusivenetworks: GraphQL: " + strings.Join(msgs, "; ")
}
