// Package exclusivenetworks is a Go client library for the Exclusive Networks
// AccessNow GraphQL API.
//
// The API is authenticated via OAuth 2.0 client_credentials: a client ID,
// secret, and scope are exchanged at the token endpoint for a short-lived
// bearer token. The client caches the token internally and refreshes it
// proactively before expiry.
//
// All public methods accept a context.Context for cancellation and timeout
// control. Non-2xx HTTP responses and GraphQL errors surface as typed errors
// that wrap the upstream status code, body, and (for GraphQL) the error
// envelope.
package exclusivenetworks
