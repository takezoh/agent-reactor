package cli

import (
	"context"
	"sync"

	"github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
)

// Factory provides CLI Backend instances keyed by project. A single Backend
// serves all frames in the same project regardless of sandbox mode — sandbox
// handling is transparent to the Backend itself (worktree creation happens
// on the host filesystem either way).
type Factory struct {
	mu       sync.Mutex
	backends map[state.SubsystemID]*Backend
}

// NewFactory constructs a CLI Factory.
func NewFactory() *Factory {
	return &Factory{backends: make(map[state.SubsystemID]*Backend)}
}

// Ensure implements subsystem.Factory.
func (f *Factory) Ensure(_ context.Context, _ state.SessionID, project string, _ state.LaunchPlan) (subsystem.Subsystem, state.SubsystemID, error) {
	id := state.SubsystemID("cli:" + project)
	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.backends[id]; ok {
		return b, id, nil
	}
	b := New(project)
	f.backends[id] = b
	return b, id, nil
}

// Range iterates all live backends.
func (f *Factory) Range(fn func(*Backend) bool) {
	f.mu.Lock()
	snapshot := make([]*Backend, 0, len(f.backends))
	for _, b := range f.backends {
		snapshot = append(snapshot, b)
	}
	f.mu.Unlock()
	for _, b := range snapshot {
		if !fn(b) {
			return
		}
	}
}
