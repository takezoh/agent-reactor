//go:build legacy_session

// Package session is the server's session-lifecycle service: it creates,
// lists, and stops agent sessions, each backed by a termvt pty session. Launch
// wrapping (direct vs devcontainer) is delegated to an agentlaunch.Dispatcher,
// so running an agent inside a sandbox is a dispatcher swap, not a code change.
package session

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/termvt"
)

// Spec describes a session to create.
type Spec struct {
	Project string `json:"project"`
	Command string `json:"command"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
}

// Info is the public metadata for a session.
type Info struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"created_at"`
}

type entry struct {
	info    Info
	cleanup func(context.Context) error
}

// Service owns the live sessions. Safe for concurrent use.
type Service struct {
	mgr  *termvt.Manager
	disp agentlaunch.Dispatcher

	mu      sync.Mutex
	entries map[string]entry
	seq     int
}

// NewService builds a Service that launches via disp (e.g. DirectDispatcher for
// host processes, SandboxDispatcher for devcontainers).
func NewService(disp agentlaunch.Dispatcher) *Service {
	return &Service{mgr: termvt.NewManager(), disp: disp, entries: map[string]entry{}}
}

// Create launches a new session and returns its metadata.
func (s *Service) Create(ctx context.Context, spec Spec) (Info, error) {
	argv, err := agentlaunch.SplitArgs(spec.Command)
	if err != nil {
		return Info{}, err
	}
	if len(argv) == 0 {
		return Info{}, fmt.Errorf("session: empty command")
	}

	id := s.nextID()
	if spec.Project != "" {
		if err := s.disp.EnsureProject(ctx, spec.Project); err != nil {
			return Info{}, fmt.Errorf("session: ensure project: %w", err)
		}
	}
	wrapped, err := s.disp.Wrap(ctx, id, agentlaunch.LaunchPlan{
		Argv: argv, Command: spec.Command, StartDir: spec.Project, Project: spec.Project,
	})
	if err != nil {
		return Info{}, fmt.Errorf("session: wrap: %w", err)
	}

	argv = pick(wrapped.Argv, argv)
	if _, err := s.mgr.Create(id, termvt.Spec{
		Argv: argv, Env: mergeEnv(wrapped.Env), Cols: spec.Cols, Rows: spec.Rows,
	}); err != nil {
		if wrapped.Cleanup != nil {
			_ = wrapped.Cleanup(ctx)
		}
		return Info{}, err
	}

	info := Info{ID: id, Project: spec.Project, Command: spec.Command, CreatedAt: time.Now()}
	s.mu.Lock()
	s.entries[id] = entry{info: info, cleanup: wrapped.Cleanup}
	s.mu.Unlock()
	return info, nil
}

// List returns session metadata sorted by id.
func (s *Service) List() []Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Info, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Session returns the live termvt session for id (for attach).
func (s *Service) Session(id string) (*termvt.Session, bool) {
	return s.mgr.Get(id)
}

// Stop terminates a session and runs its launch cleanup.
func (s *Service) Stop(ctx context.Context, id string) error {
	s.mu.Lock()
	e, ok := s.entries[id]
	delete(s.entries, id)
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("session: %q not found", id)
	}
	return s.teardown(ctx, id, e)
}

// CloseAll stops every session.
func (s *Service) CloseAll(ctx context.Context) {
	s.mu.Lock()
	ents := s.entries
	s.entries = map[string]entry{}
	s.mu.Unlock()
	for id, e := range ents {
		_ = s.teardown(ctx, id, e)
	}
}

// teardown removes a session from the manager and runs its launch cleanup.
func (s *Service) teardown(ctx context.Context, id string, e entry) error {
	err := s.mgr.Remove(id)
	if e.cleanup != nil {
		_ = e.cleanup(ctx)
	}
	return err
}

func (s *Service) nextID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return fmt.Sprintf("s%d", s.seq)
}

func pick(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func mergeEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
