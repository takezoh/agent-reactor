// Package lineargql implements the §10.5 linear_graphql client-side agent tool.
// It forwards raw GraphQL queries from the agent to the Linear API and maps
// responses to the §10.5 success/errors shape, keeping the API key out of logs.
package lineargql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 30 * time.Second

// Result holds the §10.5 output for a linear_graphql tool invocation.
type Result struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Errors  json.RawMessage `json:"errors,omitempty"`
}

// Client forwards raw GraphQL queries to the Linear API on the agent's behalf.
// It is distinct from the tracker adapter (platform/tracker/linear) which is
// dispatch-specific; this client is a general passthrough per SPEC §10.5.
type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// New returns a Client for the given Linear endpoint and API key.
// The key is transmitted only in the Authorization header and is never logged.
func New(endpoint, apiKey string) *Client {
	return newClient(endpoint, apiKey, &http.Client{Timeout: defaultTimeout})
}

func newClient(endpoint, apiKey string, hc *http.Client) *Client {
	return &Client{endpoint: endpoint, apiKey: apiKey, http: hc}
}

// Execute sends a raw GraphQL query and variables to Linear and returns a §10.5
// Result. Invalid input and auth failures are encoded in Result.Success=false;
// only unexpected encoding errors are returned as a Go error.
func (c *Client) Execute(ctx context.Context, query string, variables json.RawMessage) (*Result, error) {
	if query == "" {
		return errResult("query must not be empty"), nil
	}
	if err := validateSingleOperation(query); err != nil {
		return errResult(err.Error()), nil
	}
	if err := validateVariablesObject(variables); err != nil {
		return errResult(err.Error()), nil
	}
	if c.apiKey == "" {
		return errResult("linear API key not configured"), nil
	}
	if len(variables) == 0 {
		variables = json.RawMessage("null")
	}

	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, fmt.Errorf("lineargql: encode payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return errResult("failed to build request"), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey) // key never appears in slog output

	resp, err := c.http.Do(req)
	if err != nil {
		return errResult("transport error"), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errResult(fmt.Sprintf("HTTP %d", resp.StatusCode)), nil
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return errResult("response decode error"), nil
	}

	if len(envelope.Errors) > 0 && string(envelope.Errors) != "null" {
		return &Result{Success: false, Data: envelope.Data, Errors: envelope.Errors}, nil
	}
	return &Result{Success: true, Data: envelope.Data}, nil
}

func errResult(msg string) *Result {
	errs, _ := json.Marshal([]map[string]string{{"message": msg}})
	return &Result{Success: false, Errors: errs}
}

// validateSingleOperation returns an error if the document contains more than
// one GraphQL operation definition (SPEC §10.5 MUST: single operation).
// Fragment definitions do not count as operations.
func validateSingleOperation(query string) error {
	if countOperations(query) > 1 {
		return errors.New("query must contain exactly one GraphQL operation")
	}
	return nil
}

// validateVariablesObject returns an error if variables is present (non-null,
// non-empty) and is not a JSON object (SPEC §10.5 MUST: variables-object).
func validateVariablesObject(vars json.RawMessage) error {
	trimmed := bytes.TrimSpace(vars)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] != '{' {
		return errors.New("variables must be a JSON object")
	}
	return nil
}

// gqlScanState is the lexer mode for handling strings and comments.
type gqlScanState int

const (
	gqlssNormal      gqlScanState = iota
	gqlssString                   // inside "…"
	gqlssBlockString              // inside """…"""
	gqlssComment                  // inside #…\n
)

// gqlDocState tracks where we are in the GraphQL document structure.
type gqlDocState int

const (
	gqldsIdle      gqlDocState = iota
	gqldsOperation             // saw query/mutation/subscription; awaiting {
	gqldsFragment              // saw fragment; awaiting {
	gqldsBody                  // inside { } at depth ≥ 1
)

// gqlScanner is a lightweight GraphQL document scanner used by countOperations.
type gqlScanner struct {
	src         string
	i           int
	ss          gqlScanState
	ds          gqlDocState
	bodyDepth   int // brace depth inside gqldsBody
	headerDepth int // combined ( ) { } nesting in operation/fragment header
	count       int // operation definitions found so far
}

// countOperations counts GraphQL operation definitions in src. It respects
// string literals (including block strings) and comments so that keywords
// inside those constructs are correctly ignored.
//
// Named operations begin with "query", "mutation", or "subscription".
// Anonymous operations begin with "{" at document depth 0.
// "fragment" definitions are skipped and do not contribute to the count.
func countOperations(src string) int {
	sc := &gqlScanner{src: src}
	n := len(src)
	for sc.i < n {
		if !sc.advanceLexer() {
			sc.advanceDoc()
		}
	}
	return sc.count
}

// advanceLexer handles the current position when the scanner is inside a string
// or comment. Returns true when a character was consumed, false when the caller
// should proceed with document-level scanning.
func (sc *gqlScanner) advanceLexer() bool {
	ch := sc.src[sc.i]
	switch sc.ss {
	case gqlssComment:
		if ch == '\n' {
			sc.ss = gqlssNormal
		}
		sc.i++
		return true
	case gqlssString:
		if ch == '\\' {
			sc.i += 2
		} else {
			if ch == '"' {
				sc.ss = gqlssNormal
			}
			sc.i++
		}
		return true
	case gqlssBlockString:
		if sc.i+2 < len(sc.src) && sc.src[sc.i] == '"' && sc.src[sc.i+1] == '"' && sc.src[sc.i+2] == '"' {
			sc.ss = gqlssNormal
			sc.i += 3
		} else {
			sc.i++
		}
		return true
	}
	return false
}

// advanceDoc handles one document-level character in normal (non-string,
// non-comment) scan mode.
func (sc *gqlScanner) advanceDoc() { //nolint:cyclop
	ch := sc.src[sc.i]
	n := len(sc.src)
	switch {
	case ch == '#':
		sc.ss = gqlssComment
		sc.i++
	case sc.i+2 < n && sc.src[sc.i] == '"' && sc.src[sc.i+1] == '"' && sc.src[sc.i+2] == '"':
		sc.ss = gqlssBlockString
		sc.i += 3
	case ch == '"':
		sc.ss = gqlssString
		sc.i++
	case ch == '(' && (sc.ds == gqldsOperation || sc.ds == gqldsFragment):
		sc.headerDepth++
		sc.i++
	case ch == ')' && (sc.ds == gqldsOperation || sc.ds == gqldsFragment):
		if sc.headerDepth > 0 {
			sc.headerDepth--
		}
		sc.i++
	case ch == '{':
		sc.openBrace()
	case ch == '}':
		sc.closeBrace()
	case sc.ds == gqldsIdle && isGQLIdentRune(ch):
		sc.consumeIdent()
	default:
		sc.i++
	}
}

func (sc *gqlScanner) openBrace() {
	switch sc.ds {
	case gqldsIdle:
		sc.count++
		sc.ds = gqldsBody
		sc.bodyDepth = 1
	case gqldsOperation, gqldsFragment:
		if sc.headerDepth > 0 {
			// Inside variable-definition parens; this is an input object literal.
			sc.headerDepth++
		} else {
			sc.ds = gqldsBody
			sc.bodyDepth = 1
			sc.headerDepth = 0
		}
	case gqldsBody:
		sc.bodyDepth++
	}
	sc.i++
}

func (sc *gqlScanner) closeBrace() {
	switch sc.ds {
	case gqldsOperation, gqldsFragment:
		if sc.headerDepth > 0 {
			sc.headerDepth--
		}
	case gqldsBody:
		sc.bodyDepth--
		if sc.bodyDepth == 0 {
			sc.ds = gqldsIdle
		}
	}
	sc.i++
}

func (sc *gqlScanner) consumeIdent() {
	j := sc.i
	n := len(sc.src)
	for j < n && isGQLIdentRune(sc.src[j]) {
		j++
	}
	switch sc.src[sc.i:j] {
	case "query", "mutation", "subscription":
		sc.count++
		sc.ds = gqldsOperation
		sc.headerDepth = 0
	case "fragment":
		sc.ds = gqldsFragment
		sc.headerDepth = 0
	}
	sc.i = j
}

func isGQLIdentRune(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
