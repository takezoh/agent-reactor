package linear

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

// errRoundTripper always fails, exercising the transport-error path with no I/O.
type errRoundTripper struct{}

func (errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial failed")
}

// New binds activeStates onto the client (used by FetchCandidateIssues).
func TestNew_BindsActiveStates(t *testing.T) {
	active := []string{"Todo", "Doing"}
	c := New("http://linear.test", "key", "slug", active)
	if !reflect.DeepEqual(c.activeStates, active) {
		t.Errorf("activeStates = %v, want %v", c.activeStates, active)
	}
}

// Transport failures map to ErrAPIRequest. Uses an injected failing transport
// so the test is instant instead of waiting on a real dial/timeout.
func TestErrorMapping_RequestError(t *testing.T) {
	c := newClient("http://linear.test", "key", "slug", []string{"Todo"},
		&http.Client{Transport: errRoundTripper{}})
	_, err := c.FetchCandidateIssues(context.Background())
	if !errors.Is(err, ErrAPIRequest) {
		t.Errorf("want ErrAPIRequest, got %v", err)
	}
}
