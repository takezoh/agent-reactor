package lineargql_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/orchestrator/lineargql"
)

// captureServer starts an httptest server that captures the last request and
// returns the given JSON body with status 200.
func captureServer(t *testing.T, respBody string) (url string, lastReq func() *http.Request) {
	t.Helper()
	var last *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cp := r.Clone(context.Background())
		cp.Body = io.NopCloser(strings.NewReader(string(body)))
		last = cp
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, respBody) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() *http.Request { return last }
}

func TestExecute_successResponse(t *testing.T) {
	url, lastReq := captureServer(t, `{"data":{"issue":{"id":"abc"}}}`)
	c := lineargql.New(url, "test-key")

	result, err := c.Execute(context.Background(), "{ issue { id } }", nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.JSONEq(t, `{"issue":{"id":"abc"}}`, string(result.Data))
	assert.Empty(t, result.Errors)

	req := lastReq()
	require.NotNil(t, req)
	assert.Equal(t, "test-key", req.Header.Get("Authorization"))
}

func TestExecute_querySentWithVariables(t *testing.T) {
	url, lastReq := captureServer(t, `{"data":{}}`)
	c := lineargql.New(url, "key")
	vars := json.RawMessage(`{"teamId":"T1"}`)

	_, err := c.Execute(context.Background(), "query Q($teamId:String!){team(id:$teamId){name}}", vars)
	require.NoError(t, err)

	req := lastReq()
	var body map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
	assert.Equal(t, `"query Q($teamId:String!){team(id:$teamId){name}}"`, string(body["query"]))
	assert.JSONEq(t, `{"teamId":"T1"}`, string(body["variables"]))
}

func TestExecute_graphqlErrorsReturnSuccessFalse(t *testing.T) {
	body := `{"data":null,"errors":[{"message":"not found"},{"message":"forbidden"}]}`
	url, _ := captureServer(t, body)
	c := lineargql.New(url, "key")

	result, err := c.Execute(context.Background(), "{ issue { id } }", nil)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Errors)
	// full errors array must be preserved for debugging
	assert.Contains(t, string(result.Errors), "not found")
	assert.Contains(t, string(result.Errors), "forbidden")
}

func TestExecute_httpErrorReturnSuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	c := lineargql.New(srv.URL, "bad-key")

	result, err := c.Execute(context.Background(), "{ viewer { id } }", nil)
	require.NoError(t, err)
	assert.False(t, result.Success)
}

func TestExecute_transportFailureReturnSuccessFalse(t *testing.T) {
	c := lineargql.New("http://127.0.0.1:1", "key") // nothing listening

	result, err := c.Execute(context.Background(), "{ viewer { id } }", nil)
	require.NoError(t, err)
	assert.False(t, result.Success)
}

func TestExecute_emptyQueryReturnSuccessFalse(t *testing.T) {
	c := lineargql.New("http://unused", "key")

	result, err := c.Execute(context.Background(), "", nil)
	require.NoError(t, err)
	assert.False(t, result.Success)
}

func TestExecute_missingAPIKeyReturnSuccessFalse(t *testing.T) {
	c := lineargql.New("http://unused", "")

	result, err := c.Execute(context.Background(), "{ viewer { id } }", nil)
	require.NoError(t, err)
	assert.False(t, result.Success)
}

func TestExecute_multipleOperationsRejected(t *testing.T) {
	// Two named operations in one document must be rejected without hitting the
	// network (no server is started here).
	c := lineargql.New("http://unused", "key")
	src := `query A { viewer { id } } query B { teams { nodes { id } } }`

	result, err := c.Execute(context.Background(), src, nil)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, string(result.Errors), "exactly one")
}

func TestExecute_multipleOperationsNotSentToServer(t *testing.T) {
	// Confirm no HTTP request is made when the document has multiple operations.
	_, lastReq := captureServer(t, `{"data":{}}`)
	c := lineargql.New("http://unused", "key") // wrong URL — would fail if reached
	src := `query A { viewer { id } } query B { teams { nodes { id } } }`

	result, err := c.Execute(context.Background(), src, nil)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Nil(t, lastReq(), "no request should reach the server")
}

func TestExecute_arrayVariablesRejected(t *testing.T) {
	c := lineargql.New("http://unused", "key")

	result, err := c.Execute(context.Background(), "{ viewer { id } }", []byte(`["a","b"]`))
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, string(result.Errors), "JSON object")
}

func TestExecute_stringVariablesRejected(t *testing.T) {
	c := lineargql.New("http://unused", "key")

	result, err := c.Execute(context.Background(), "{ viewer { id } }", []byte(`"hello"`))
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, string(result.Errors), "JSON object")
}

func TestExecute_nullVariablesAllowed(t *testing.T) {
	url, _ := captureServer(t, `{"data":{}}`)
	c := lineargql.New(url, "key")

	result, err := c.Execute(context.Background(), "{ viewer { id } }", []byte("null"))
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestExecute_objectVariablesAllowed(t *testing.T) {
	url, _ := captureServer(t, `{"data":{}}`)
	c := lineargql.New(url, "key")

	result, err := c.Execute(context.Background(), "{ viewer { id } }", []byte(`{"key":"val"}`))
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestExecute_tokenNotLogged(t *testing.T) {
	secret := "super-secret-api-key-12345"
	url, _ := captureServer(t, `{"data":{}}`)
	c := lineargql.New(url, secret)

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(slog.Default()) })

	_, err := c.Execute(context.Background(), "{ viewer { id } }", nil)
	require.NoError(t, err)
	assert.NotContains(t, logBuf.String(), secret, "API key must not appear in log output")
}
