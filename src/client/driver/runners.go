package driver

import (
	"context"

	"github.com/takezoh/agent-roost/client/runtime/worker"
	"github.com/takezoh/agent-roost/platform/lib/vcs"
)

// RegisterRunners registers all worker-pool runners for the driver package.
func RegisterRunners(summarizeCmd string) {
	tp, hs := newTranscriptSummaryRunners(summarizeCmd)
	worker.RegisterRunner("transcript_parse", tp)
	worker.RegisterRunner("codex_transcript_parse", newCodexTranscriptParse())
	worker.RegisterRunner("gemini_transcript_parse", newGeminiTranscriptParse())
	worker.RegisterRunner("summary_command", hs)
	worker.RegisterRunner("branch_detect", newBranchDetect())
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
