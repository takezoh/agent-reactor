package subsystem

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	roostgit "github.com/takezoh/agent-roost/platform/lib/git"
)

const WorktreeNameAttempts = 5

// WorktreeInput is the request to create a managed worktree.
type WorktreeInput struct {
	RepoDir        string
	CandidateNames []string
}

// WorktreeResult is the outcome of worktree creation or adoption.
type WorktreeResult struct {
	StartDir string
	Name     string
}

// CreateWorktree tries each candidate name and returns the first
// successfully created (or already-extant) worktree.
func CreateWorktree(ctx context.Context, in WorktreeInput) (WorktreeResult, error) {
	root, err := roostgit.RepoRoot(ctx, in.RepoDir)
	if err != nil {
		return WorktreeResult{}, err
	}
	for _, name := range in.CandidateNames {
		if name == "" {
			continue
		}
		path := filepath.Join(root, ".roost", "worktrees", name)
		if _, err := os.Stat(path); err == nil {
			return WorktreeResult{StartDir: path, Name: name}, nil
		}
		dir, err := roostgit.CreateWorktree(ctx, in.RepoDir, name)
		if err == nil {
			return WorktreeResult{StartDir: dir, Name: name}, nil
		}
	}
	return WorktreeResult{}, errors.New("failed to create managed worktree after all candidates")
}

// RemoveWorktree removes a single managed worktree asynchronously.
func RemoveWorktree(path string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := roostgit.RemoveWorktree(ctx, path); err != nil {
			slog.Warn("subsystem: remove worktree failed", "path", path, "err", err)
		}
	}()
}

// CleanupUntracked removes worktrees under <project>/.roost/worktrees/
// that are not present in the tracked set.
func CleanupUntracked(ctx context.Context, projects []string, tracked map[string]struct{}) {
	for _, project := range projects {
		worktreesDir := filepath.Join(project, ".roost", "worktrees")
		entries, err := os.ReadDir(worktreesDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			path := filepath.Clean(filepath.Join(worktreesDir, e.Name()))
			if _, ok := tracked[path]; ok {
				continue
			}
			c, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := roostgit.RemoveWorktree(c, path); err != nil {
				slog.Warn("subsystem: cleanup untracked worktree failed", "path", path, "err", err)
			}
			cancel()
		}
	}
}

// GenerateWorktreeNames returns n petnames to use as worktree candidates.
func GenerateWorktreeNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = petname.Generate(2, "-")
	}
	return names
}

// IsManagedWorktreePath returns true if the path looks like a roost-managed worktree.
func IsManagedWorktreePath(path string) bool {
	clean := filepath.Clean(path)
	parent := filepath.Base(filepath.Dir(clean))
	grand := filepath.Base(filepath.Dir(filepath.Dir(clean)))
	return parent == "worktrees" && grand == ".roost"
}
