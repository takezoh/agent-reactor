package driver

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	roostgit "github.com/takezoh/agent-roost/lib/git"
	"github.com/takezoh/agent-roost/lib/vcs"
	"github.com/takezoh/agent-roost/runtime/worker"
)

// RegisterRunners registers all worker-pool runners for the driver package.
func RegisterRunners(summarizeCmd string) {
	tp, hs := newTranscriptSummaryRunners(summarizeCmd)
	worker.RegisterRunner("transcript_parse", tp)
	worker.RegisterRunner("codex_transcript_parse", newCodexTranscriptParse())
	worker.RegisterRunner("gemini_transcript_parse", newGeminiTranscriptParse())
	worker.RegisterRunner("summary_command", hs)
	worker.RegisterRunner("branch_detect", newBranchDetect())
	worker.RegisterRunner("worktree_setup", newWorktreeSetup())
}

func newBranchDetect() func(context.Context, BranchDetectInput) (BranchDetectResult, error) {
	return func(ctx context.Context, in BranchDetectInput) (BranchDetectResult, error) {
		r := vcs.DetectBranch(ctx, in.WorkingDir)
		return BranchDetectResult{
			Branch: r.Branch, Background: r.Background, Foreground: r.Foreground,
			IsWorktree: r.IsWorktree, ParentBranch: r.ParentBranch,
		}, nil
	}
}

func newWorktreeSetup() func(context.Context, WorktreeSetupInput) (WorktreeSetupResult, error) {
	return func(ctx context.Context, in WorktreeSetupInput) (WorktreeSetupResult, error) {
		root, err := roostgit.RepoRoot(ctx, in.RepoDir)
		if err != nil {
			return WorktreeSetupResult{}, err
		}
		for _, name := range in.CandidateNames {
			if name == "" {
				continue
			}
			path := filepath.Join(root, ".roost", "worktrees", name)
			if _, err := os.Stat(path); err == nil {
				continue
			}
			dir, err := roostgit.CreateWorktree(ctx, in.RepoDir, name)
			if err == nil {
				return WorktreeSetupResult{StartDir: dir, Name: name}, nil
			}
		}
		return WorktreeSetupResult{}, errors.New("failed to create managed worktree")
	}
}
