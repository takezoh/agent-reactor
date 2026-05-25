// Package cli implements the CLI execution subsystem backend.
// It manages worktree lifecycle for CLI-spawned frames (claude, gemini,
// shell, etc.) and provides the standard Subsystem interface.
package cli

import (
	"context"
	"sync"

	"github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
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

// BindFrame resolves the launch plan for a CLI frame.
//
// Worktree ownership rules:
//   - Worktree.Enabled=true, StartDir not a managed path → fresh create; this
//     frame owns the worktree and is responsible for removing it on release.
//   - StartDir is already a managed path AND Worktree.Enabled=true (cold-start
//     re-adoption by the original owner) → adopt without creating; this frame
//     still owns the worktree and removes it on release.
//   - StartDir is already a managed path AND Worktree.Enabled=false → a child
//     frame borrowing another frame's worktree; do NOT register for cleanup.
//     This prevents cross-frame (or cross-backend) deletion of a shared path.
func (b *Backend) BindFrame(ctx context.Context, req subsystem.BindRequest) (subsystem.BindResult, error) {
	result := subsystem.BindResult{Plan: req.Plan}
	worktreePath := ""

	switch {
	case subsystem.IsManagedWorktreePath(req.Plan.StartDir):
		// The StartDir already points at an existing managed worktree.
		result.WorktreeStartDir = req.Plan.StartDir
		if req.Plan.Options.Worktree.Enabled {
			// Original owner re-adopting on cold-start: register for cleanup.
			worktreePath = req.Plan.StartDir
		}
		// Borrower (Enabled=false): worktreePath stays "", so ReleaseFrame
		// will not remove a worktree owned by a different frame or backend.

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
// its managed worktree if no other tracked frame still references the same path.
// This prevents destroying a worktree shared between root and child frames.
func (b *Backend) ReleaseFrame(frameID state.FrameID) {
	b.mu.Lock()
	path := b.frames[frameID]
	delete(b.frames, frameID)
	stillUsed := false
	for _, p := range b.frames {
		if p == path {
			stillUsed = true
			break
		}
	}
	b.mu.Unlock()
	if path != "" && !stillUsed {
		subsystem.RemoveWorktree(path)
	}
}

// Stop is a no-op for the CLI backend; resources are released per-frame.
func (b *Backend) Stop(_ context.Context) {}
