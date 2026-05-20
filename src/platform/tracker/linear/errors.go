package linear

import "errors"

// Sentinel errors for §11.4 Linear-specific failures.
var (
	ErrAPIRequest       = errors.New("linear_api_request")
	ErrAPIStatus        = errors.New("linear_api_status")
	ErrGraphQLErrors    = errors.New("linear_graphql_errors")
	ErrUnknownPayload   = errors.New("linear_unknown_payload")
	ErrMissingEndCursor = errors.New("linear_missing_end_cursor")
)
