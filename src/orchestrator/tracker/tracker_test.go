package tracker

import (
	"context"
	"errors"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	ptracker "github.com/takezoh/agent-roost/platform/tracker"
)

// fakeAdapter records calls for assertions.
type fakeAdapter struct {
	candidateCalled bool
	refreshIDs      []string
	byStateNames    []string
	stubIssues      []ptracker.Issue
	stubErr         error
}

func (f *fakeAdapter) FetchCandidateIssues(_ context.Context) ([]ptracker.Issue, error) {
	f.candidateCalled = true
	return f.stubIssues, f.stubErr
}

func (f *fakeAdapter) FetchIssuesByStates(_ context.Context, stateNames []string) ([]ptracker.Issue, error) {
	f.byStateNames = stateNames
	return f.stubIssues, f.stubErr
}

func (f *fakeAdapter) FetchIssueStatesByIDs(_ context.Context, issueIDs []string) ([]ptracker.Issue, error) {
	f.refreshIDs = issueIDs
	return f.stubIssues, f.stubErr
}

// fakeFactoryResult bundles the captured construction args and the adapter returned.
type fakeFactoryResult struct {
	endpoint     string
	apiKey       string
	projectSlug  string
	activeStates []string
	adapter      *fakeAdapter
}

func newFakeFactory(stub *fakeAdapter) (adapterFactory, *fakeFactoryResult) {
	result := &fakeFactoryResult{adapter: stub}
	factory := func(ep, key, slug string, active []string) ptracker.Adapter {
		result.endpoint = ep
		result.apiKey = key
		result.projectSlug = slug
		result.activeStates = append([]string(nil), active...)
		return stub
	}
	return factory, result
}

func validCfg() wfconfig.Config {
	return wfconfig.Config{
		Tracker: wfconfig.TrackerConfig{
			Kind:           "linear",
			Endpoint:       "https://api.linear.app/graphql",
			APIKey:         "lin_api_key",
			ProjectSlug:    "my-project",
			ActiveStates:   []string{"Todo", "In Progress"},
			TerminalStates: []string{"Done", "Cancelled"},
		},
	}
}

func TestNew_UnsupportedKind(t *testing.T) {
	cfg := validCfg()
	cfg.Tracker.Kind = "jira"
	_, err := New(cfg)
	if !errors.Is(err, ptracker.ErrUnsupportedTrackerKind) {
		t.Fatalf("want ErrUnsupportedTrackerKind, got %v", err)
	}
}

func TestNew_MissingAPIKey(t *testing.T) {
	cfg := validCfg()
	cfg.Tracker.APIKey = ""
	_, err := New(cfg)
	if !errors.Is(err, ptracker.ErrMissingTrackerAPIKey) {
		t.Fatalf("want ErrMissingTrackerAPIKey, got %v", err)
	}
}

func TestNew_MissingProjectSlug(t *testing.T) {
	cfg := validCfg()
	cfg.Tracker.ProjectSlug = ""
	_, err := New(cfg)
	if !errors.Is(err, ptracker.ErrMissingTrackerProjectSlug) {
		t.Fatalf("want ErrMissingTrackerProjectSlug, got %v", err)
	}
}

func TestNew_PassesConstructionArgsToFactory(t *testing.T) {
	cfg := validCfg()
	factory, result := newFakeFactory(&fakeAdapter{})
	tr, err := newWithFactory(cfg, factory)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil Tracker")
	}
	if result.endpoint != cfg.Tracker.Endpoint {
		t.Errorf("endpoint: want %q, got %q", cfg.Tracker.Endpoint, result.endpoint)
	}
	if result.apiKey != cfg.Tracker.APIKey {
		t.Errorf("apiKey: want %q, got %q", cfg.Tracker.APIKey, result.apiKey)
	}
	if result.projectSlug != cfg.Tracker.ProjectSlug {
		t.Errorf("projectSlug: want %q, got %q", cfg.Tracker.ProjectSlug, result.projectSlug)
	}
	if len(result.activeStates) != len(cfg.Tracker.ActiveStates) {
		t.Fatalf("activeStates len: want %d, got %d", len(cfg.Tracker.ActiveStates), len(result.activeStates))
	}
	for i, s := range cfg.Tracker.ActiveStates {
		if result.activeStates[i] != s {
			t.Errorf("activeStates[%d]: want %q, got %q", i, s, result.activeStates[i])
		}
	}
}

func TestTerminalIssues_PassesTerminalStates(t *testing.T) {
	cfg := validCfg()
	fake := &fakeAdapter{}
	factory, _ := newFakeFactory(fake)
	tr, _ := newWithFactory(cfg, factory)

	_, _ = tr.TerminalIssues(context.Background())

	if len(fake.byStateNames) != len(cfg.Tracker.TerminalStates) {
		t.Fatalf("byStateNames len: want %d, got %d", len(cfg.Tracker.TerminalStates), len(fake.byStateNames))
	}
	for i, s := range cfg.Tracker.TerminalStates {
		if fake.byStateNames[i] != s {
			t.Errorf("terminalState[%d]: want %q, got %q", i, s, fake.byStateNames[i])
		}
	}
}

func TestRefreshStates_EmptyIDs_NoCall(t *testing.T) {
	cfg := validCfg()
	fake := &fakeAdapter{}
	factory, _ := newFakeFactory(fake)
	tr, _ := newWithFactory(cfg, factory)

	for _, ids := range [][]string{nil, {}} {
		fake.refreshIDs = nil
		issues, err := tr.RefreshStates(context.Background(), ids)
		if err != nil {
			t.Errorf("ids=%v: unexpected error: %v", ids, err)
		}
		if issues != nil {
			t.Errorf("ids=%v: want nil issues, got %v", ids, issues)
		}
		if fake.refreshIDs != nil {
			t.Errorf("ids=%v: FetchIssueStatesByIDs should not be called", ids)
		}
	}
}

func TestRefreshStates_NonEmpty_PassesIDs(t *testing.T) {
	cfg := validCfg()
	fake := &fakeAdapter{}
	factory, _ := newFakeFactory(fake)
	tr, _ := newWithFactory(cfg, factory)

	ids := []string{"id-1", "id-2"}
	_, _ = tr.RefreshStates(context.Background(), ids)

	if len(fake.refreshIDs) != len(ids) {
		t.Fatalf("refreshIDs len: want %d, got %d", len(ids), len(fake.refreshIDs))
	}
	for i, id := range ids {
		if fake.refreshIDs[i] != id {
			t.Errorf("refreshIDs[%d]: want %q, got %q", i, id, fake.refreshIDs[i])
		}
	}
}

func TestCandidates_DelegatesResult(t *testing.T) {
	cfg := validCfg()
	want := []ptracker.Issue{{ID: "i1", Title: "Test"}}
	fake := &fakeAdapter{stubIssues: want}
	factory, _ := newFakeFactory(fake)
	tr, _ := newWithFactory(cfg, factory)

	got, err := tr.Candidates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) || got[0].ID != want[0].ID {
		t.Errorf("want %v, got %v", want, got)
	}
	if !fake.candidateCalled {
		t.Error("FetchCandidateIssues was not called")
	}
}

func TestCandidates_PropagatesError(t *testing.T) {
	cfg := validCfg()
	sentinel := errors.New("some_adapter_error")
	fake := &fakeAdapter{stubErr: sentinel}
	factory, _ := newFakeFactory(fake)
	tr, _ := newWithFactory(cfg, factory)

	_, err := tr.Candidates(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel propagated, got %v", err)
	}
}
