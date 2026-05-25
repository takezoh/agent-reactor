package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type fakeRunner struct {
	available bool
	responses map[string][]byte
	errs      map[string]error
	mu        sync.Mutex
	calls     []string
}

func newFake() *fakeRunner {
	return &fakeRunner{available: true, responses: map[string][]byte{}, errs: map[string]error{}}
}

func (f *fakeRunner) Available() bool { return f.available }

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	f.mu.Lock()
	f.calls = append(f.calls, key)
	f.mu.Unlock()
	for prefix, resp := range f.responses {
		if strings.HasPrefix(key, prefix) {
			if err := f.errs[prefix]; err != nil {
				return nil, err
			}
			return resp, nil
		}
	}
	return nil, fmt.Errorf("no fake response for: %s", key)
}

func TestFetchSummaryUnavailable(t *testing.T) {
	f := newFake()
	f.available = false
	_, err := FetchSummaryWith(context.Background(), f)
	if !errors.Is(err, ErrNotAvailable) {
		t.Errorf("got %v, want ErrNotAvailable", err)
	}
}

func TestFetchSummaryHappyPath(t *testing.T) {
	f := newFake()
	prJSON := `[{"number":1,"title":"PR","repository":{"nameWithOwner":"o/r"},"url":"https://x/1","updatedAt":"2025-01-01T00:00:00Z"}]`
	issuesJSON := `[{"number":2,"title":"Iss","repository":{"nameWithOwner":"o/r"},"url":"https://x/2","updatedAt":"2025-01-01T00:00:00Z"}]`
	reposJSON := `[{"nameWithOwner":"o/r"}]`
	runsJSON := `[{"name":"CI","status":"completed","conclusion":"failure","headBranch":"main","updatedAt":"2025-01-01T00:00:00Z","url":"https://x/r1"}]`
	f.responses["search prs"] = []byte(prJSON)
	f.responses["search issues"] = []byte(issuesJSON)
	f.responses["repo list"] = []byte(reposJSON)
	f.responses["run list"] = []byte(runsJSON)

	sum, err := FetchSummaryWith(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.PRs) != 1 || sum.PRs[0].Number != 1 {
		t.Errorf("PRs: %+v", sum.PRs)
	}
	if len(sum.Issues) != 1 || sum.Issues[0].Number != 2 {
		t.Errorf("Issues: %+v", sum.Issues)
	}
	if len(sum.Runs) != 1 || sum.Runs[0].Conclusion != "failure" {
		t.Errorf("Runs: %+v", sum.Runs)
	}
}

func TestFetchSummaryPRError(t *testing.T) {
	f := newFake()
	f.responses["search prs"] = nil
	f.errs["search prs"] = errors.New("boom")
	_, err := FetchSummaryWith(context.Background(), f)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchSummaryIssuesError(t *testing.T) {
	f := newFake()
	f.responses["search prs"] = []byte(`[]`)
	f.responses["search issues"] = nil
	f.errs["search issues"] = errors.New("boom")
	_, err := FetchSummaryWith(context.Background(), f)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchSummaryRunsFailureIsIgnored(t *testing.T) {
	f := newFake()
	f.responses["search prs"] = []byte(`[]`)
	f.responses["search issues"] = []byte(`[]`)
	f.responses["repo list"] = nil
	f.errs["repo list"] = errors.New("network")
	sum, err := FetchSummaryWith(context.Background(), f)
	if err != nil {
		t.Fatalf("runs error should not fail: %v", err)
	}
	if len(sum.Runs) != 0 {
		t.Errorf("runs = %+v", sum.Runs)
	}
}

func TestListRepoRunsSkipsSuccessfulCompleted(t *testing.T) {
	f := newFake()
	f.responses["run list"] = []byte(`[
		{"name":"OK","status":"completed","conclusion":"success","headBranch":"main","updatedAt":"2025-01-01T00:00:00Z","url":"https://x/ok"},
		{"name":"InProg","status":"in_progress","conclusion":"","headBranch":"main","updatedAt":"2025-01-01T00:00:00Z","url":"https://x/ip"},
		{"name":"Fail","status":"completed","conclusion":"failure","headBranch":"main","updatedAt":"2025-01-01T00:00:00Z","url":"https://x/f"}
	]`)
	runs, err := listRepoRuns(context.Background(), f, "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("len = %d, want 2 (OK skipped)", len(runs))
	}
	for _, r := range runs {
		if r.Name == "OK" {
			t.Errorf("successful completed run not filtered: %+v", r)
		}
	}
}

func TestListRepoRunsInvalidJSON(t *testing.T) {
	f := newFake()
	f.responses["run list"] = []byte(`bad`)
	_, err := listRepoRuns(context.Background(), f, "o/r")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListMyReposInvalidJSON(t *testing.T) {
	f := newFake()
	f.responses["repo list"] = []byte(`bad`)
	_, err := listMyRepos(context.Background(), f)
	if err == nil {
		t.Error("expected error")
	}
}
