// Package cli implements the CLI execution subsystem backend.
// It manages worktree lifecycle for CLI-spawned frames (claude, gemini,
// shell, etc.) and provides the standard Subsystem interface.
package cli

import (
	"context"
	"sync"

	"github.com/takezoh/agent-roost/runtime/subsystem"
	"github.com/takezoh/agent-roost/state"
)

// Backend is the CLI execution subsystem. One instance per project path.
type Backend struct {
	project string
	mu      sync.Mutex
	frames  map[state.FrameID]string // frameID → worktree path ("" = none)
}

// New constructs a Backend for the given project.
func New(project string) *Backend {
	return &Backend{
		project: project,
		frames:  make(map[state.FrameID]string),
	}
}

func (b *Backend) Kind() state.LaunchSubsystem { return state.LaunchSubsystemCLI }

func (b *Backend) Start(_ context.Context) error { return nil }

// BindFrame resolves the launch plan for a CLI frame. If StartDir is already
// a managed worktree path (cold-start adoption), or Options.Worktree.Enabled
// is set (fresh start), a worktree is registered for cleanup on ReleaseFrame.
func (b *Backend) BindFrame(ctx context.Context, req subsystem.BindRequest) (subsystem.BindResult, error) {
	result := subsystem.BindResult{Plan: req.Plan}
	worktreePath := ""

	switch {
	case subsystem.IsManagedWorktreePath(req.Plan.StartDir):
		// Cold-start adoption: existing worktree, no creation needed.
		worktreePath = req.Plan.StartDir
		result.WorktreeStartDir = worktreePath

	case req.Plan.Options.Worktree.Enabled:
		// Fresh start: create a new managed worktree.
		names := subsystem.GenerateWorktreeNames(subsystem.WorktreeNameAttempts)
		wt, err := subsystem.CreateWorktree(ctx, subsystem.WorktreeInput{
			RepoDir:        req.Plan.StartDir,
			CandidateNames: names,
		})
		if err != nil {
			return subsystem.BindResult{}, err
		}
		worktreePath = wt.StartDir
		result.Plan.StartDir = wt.StartDir
		result.WorktreeStartDir = wt.StartDir
		result.WorktreeName = wt.Name
	}

	b.mu.Lock()
	b.frames[req.FrameID] = worktreePath
	b.mu.Unlock()
	return result, nil
}

// ReleaseFrame removes the frame from tracking and asynchronously removes
// its managed worktree (if any).
func (b *Backend) ReleaseFrame(frameID state.FrameID) {
	b.mu.Lock()
	path := b.frames[frameID]
	delete(b.frames, frameID)
	b.mu.Unlock()
	if path != "" {
		subsystem.RemoveWorktree(path)
	}
}

// Stop is a no-op for the CLI backend; resources are released per-frame.
func (b *Backend) Stop(_ context.Context) {}

// CleanupUntracked removes worktrees under the project's .roost/worktrees/
// that are not tracked by any registered frame.
func (b *Backend) CleanupUntracked(ctx context.Context) {
	b.mu.Lock()
	tracked := make(map[string]struct{}, len(b.frames))
	for _, path := range b.frames {
		if path != "" {
			tracked[path] = struct{}{}
		}
	}
	b.mu.Unlock()
	subsystem.CleanupUntracked(ctx, []string{b.project}, tracked)
}
