package tracker

import "errors"

// Sentinel errors for §11.4 config/kind-level failures.
// 009 (orchestrator/tracker) uses these when constructing an adapter from wfconfig.
var (
	ErrUnsupportedTrackerKind    = errors.New("unsupported_tracker_kind")
	ErrMissingTrackerAPIKey      = errors.New("missing_tracker_api_key")
	ErrMissingTrackerProjectSlug = errors.New("missing_tracker_project_slug")
)
