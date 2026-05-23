package linear

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/takezoh/agent-roost/platform/tracker"
)

// errRoundTripper always fails, exercising the transport-error path with no I/O.
type errRoundTripper struct{}

func (errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial failed")
}

func TestNormalizeBlockers_CaseInsensitiveAndTrim(t *testing.T) {
	ref := rawIssueRef{ID: "id1", Identifier: "DEV-1", State: rawState{Name: "Todo"}}
	want := []tracker.Blocker{{ID: "id1", Identifier: "DEV-1", State: "Todo"}}

	cases := []struct {
		name    string
		relType string
		wantLen int
	}{
		{"exact lowercase", "blocks", 1},
		{"uppercase B", "Blocks", 1},
		{"all caps", "BLOCKS", 1},
		{"leading space", " blocks", 1},
		{"trailing space", "blocks ", 1},
		{"padded mixed case", "  Blocks  ", 1},
		{"non-blocks type", "related", 0},
		{"empty type", "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := []rawRelNode{{Type: tc.relType, Issue: ref}}
			got := normalizeBlockers(nodes)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d (type=%q)", len(got), tc.wantLen, tc.relType)
			}
			if tc.wantLen == 0 && got != nil {
				t.Errorf("want nil slice, got %+v", got)
			}
			if tc.wantLen > 0 && !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
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
