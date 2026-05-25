package github

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"sync"
	"time"
)

var ErrNotAvailable = errors.New("gh CLI not available")

type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
	Available() bool
}

type execRunner struct {
	once  sync.Once
	found bool
}

func (e *execRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "gh", args...).Output()
}

func (e *execRunner) Available() bool {
	e.once.Do(func() { _, err := exec.LookPath("gh"); e.found = err == nil })
	return e.found
}

var DefaultRunner Runner = &execRunner{}

type Summary struct {
	PRs    []Item
	Issues []Item
	Runs   []Run
}

type Run struct {
	Name       string
	Status     string
	Conclusion string
	Branch     string
	Repo       string
	URL        string
	Age        time.Duration
}

type Item struct {
	Number int
	Title  string
	Repo   string
	URL    string
	Age    time.Duration
}

type ghSearchItem struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func FetchSummary(ctx context.Context) (Summary, error) {
	return FetchSummaryWith(ctx, DefaultRunner)
}

func FetchSummaryWith(ctx context.Context, r Runner) (Summary, error) {
	if !r.Available() {
		return Summary{}, ErrNotAvailable
	}
	prs, err := searchPRs(ctx, r)
	if err != nil {
		return Summary{}, err
	}
	issues, err := searchIssues(ctx, r)
	if err != nil {
		return Summary{}, err
	}
	runs, _ := fetchRuns(ctx, r)
	return Summary{PRs: prs, Issues: issues, Runs: runs}, nil
}

func searchPRs(ctx context.Context, r Runner) ([]Item, error) {
	out, err := r.Run(ctx, "search", "prs",
		"--author=@me", "--state=open",
		"--json", "number,title,repository,url,updatedAt",
	)
	if err != nil {
		return nil, err
	}
	return parseItems(out)
}

func searchIssues(ctx context.Context, r Runner) ([]Item, error) {
	owned, err := runIssueSearch(ctx, r, "--owner=@me")
	if err != nil {
		return nil, err
	}
	assigned, err := runIssueSearch(ctx, r, "--assignee=@me")
	if err != nil {
		return nil, err
	}
	return dedup(owned, assigned), nil
}

func runIssueSearch(ctx context.Context, r Runner, filter string) ([]Item, error) {
	out, err := r.Run(ctx, "search", "issues",
		filter, "--state=open",
		"--json", "number,title,repository,url,updatedAt",
	)
	if err != nil {
		return nil, err
	}
	return parseItems(out)
}

func dedup(primary, secondary []Item) []Item {
	seen := make(map[string]struct{}, len(primary))
	for _, item := range primary {
		seen[item.URL] = struct{}{}
	}
	result := make([]Item, len(primary), len(primary)+len(secondary))
	copy(result, primary)
	for _, item := range secondary {
		if _, ok := seen[item.URL]; !ok {
			result = append(result, item)
		}
	}
	return result
}

type ghRepo struct {
	NameWithOwner string `json:"nameWithOwner"`
}

type ghRunItem struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HeadBranch string    `json:"headBranch"`
	UpdatedAt  time.Time `json:"updatedAt"`
	URL        string    `json:"url"`
}

func fetchRuns(ctx context.Context, r Runner) ([]Run, error) {
	repos, err := listMyRepos(ctx, r)
	if err != nil {
		return nil, err
	}

	type result struct {
		runs []Run
	}

	sem := make(chan struct{}, 5)
	ch := make(chan result, len(repos))
	for _, repo := range repos {
		go func(repo string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			runs, _ := listRepoRuns(ctx, r, repo)
			ch <- result{runs: runs}
		}(repo)
	}

	var all []Run
	for range repos {
		res := <-ch
		all = append(all, res.runs...)
	}
	return all, nil
}

func listMyRepos(ctx context.Context, r Runner) ([]string, error) {
	out, err := r.Run(ctx, "repo", "list", "--json", "nameWithOwner", "--limit", "30")
	if err != nil {
		return nil, err
	}
	var repos []ghRepo
	if err := json.Unmarshal(out, &repos); err != nil {
		return nil, err
	}
	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.NameWithOwner
	}
	return names, nil
}

func listRepoRuns(ctx context.Context, r Runner, repo string) ([]Run, error) {
	out, err := r.Run(ctx, "run", "list",
		"--repo", repo,
		"--json", "name,status,conclusion,headBranch,updatedAt,url",
		"--limit", "5",
	)
	if err != nil {
		return nil, err
	}
	var raw []ghRunItem
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	now := time.Now()
	var runs []Run
	for _, r := range raw {
		if r.Status == "completed" && r.Conclusion != "failure" {
			continue
		}
		runs = append(runs, Run{
			Name:       r.Name,
			Status:     r.Status,
			Conclusion: r.Conclusion,
			Branch:     r.HeadBranch,
			Repo:       repo,
			URL:        r.URL,
			Age:        now.Sub(r.UpdatedAt),
		})
	}
	return runs, nil
}

func parseItems(data []byte) ([]Item, error) {
	var raw []ghSearchItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	items := make([]Item, len(raw))
	now := time.Now()
	for i, r := range raw {
		items[i] = Item{
			Number: r.Number,
			Title:  r.Title,
			Repo:   r.Repository.NameWithOwner,
			URL:    r.URL,
			Age:    now.Sub(r.UpdatedAt),
		}
	}
	return items, nil
}
