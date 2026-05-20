package agentlaunch_test

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/platform/agentlaunch"
	"github.com/takezoh/agent-roost/platform/config"
)

// fakeDispatcher records calls for assertion in tests.
type fakeDispatcher struct {
	wrapCalled   bool
	adoptCalled  bool
	wrapErr      error
	adoptErr     error
	wrapResult   agentlaunch.WrappedLaunch
	isContainerV bool
}

func (f *fakeDispatcher) Wrap(_ context.Context, _ string, _ agentlaunch.LaunchPlan) (agentlaunch.WrappedLaunch, error) {
	f.wrapCalled = true
	return f.wrapResult, f.wrapErr
}

func (f *fakeDispatcher) AdoptFrame(_ context.Context, _, _ string) (func(context.Context) error, []agentlaunch.Mount, error) {
	f.adoptCalled = true
	return nil, nil, f.adoptErr
}

func (f *fakeDispatcher) EnsureProject(_ context.Context, _ string) error { return nil }

func (f *fakeDispatcher) IsContainer(_ string) bool { return f.isContainerV }

func TestSandboxDispatcher_DirectMode_RoutesToDirect(t *testing.T) {
	direct := &fakeDispatcher{wrapResult: agentlaunch.WrappedLaunch{Command: "bash"}}
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "direct"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: direct}

	plan := agentlaunch.LaunchPlan{Project: "/workspace/foo", Command: "bash"}
	got, err := d.Wrap(context.Background(), "f1", plan)
	if err != nil {
		t.Fatalf("Wrap error: %v", err)
	}
	if !direct.wrapCalled {
		t.Error("expected direct Wrap to be called")
	}
	if got.Command != "bash" {
		t.Errorf("Command = %q, want bash", got.Command)
	}
}

func TestSandboxDispatcher_EmptyMode_RoutesToDirect(t *testing.T) {
	direct := &fakeDispatcher{}
	resolver := config.NewSandboxResolver(config.SandboxConfig{})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: direct}

	_, err := d.Wrap(context.Background(), "f1", agentlaunch.LaunchPlan{Project: "/workspace/foo"})
	if err != nil {
		t.Fatalf("Wrap error: %v", err)
	}
	if !direct.wrapCalled {
		t.Error("expected direct.Wrap called for empty mode")
	}
}

func TestSandboxDispatcher_DevcontainerMode_NilDevcontainer_ReturnsError(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: &fakeDispatcher{}, Devcontainer: nil}

	_, err := d.Wrap(context.Background(), "f1", agentlaunch.LaunchPlan{Project: "/workspace/foo"})
	if err == nil {
		t.Error("expected error when devcontainer backend is nil but mode=devcontainer")
	}
}

func TestSandboxDispatcher_UnknownMode_ReturnsError(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "firecracker"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: &fakeDispatcher{}}

	_, err := d.Wrap(context.Background(), "f1", agentlaunch.LaunchPlan{Project: "/workspace/foo"})
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

// ForceHost bypasses project-level sandbox config and goes directly to Direct.
// Devcontainer is left nil; without the override that would return an error —
// the fact that Direct succeeds confirms the early-return path was taken.
func TestSandboxDispatcher_ForceHost_RoutesToDirect(t *testing.T) {
	direct := &fakeDispatcher{wrapResult: agentlaunch.WrappedLaunch{Command: "claude"}}
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &agentlaunch.SandboxDispatcher{
		Resolver:     resolver,
		Direct:       direct,
		Devcontainer: nil,
	}

	plan := agentlaunch.LaunchPlan{
		Project:   "/workspace/foo",
		Command:   "claude",
		ForceHost: true,
	}
	got, err := d.Wrap(context.Background(), "f1", plan)
	if err != nil {
		t.Fatalf("Wrap error: %v", err)
	}
	if !direct.wrapCalled {
		t.Error("expected Direct.Wrap to be called")
	}
	if got.Command != "claude" {
		t.Errorf("Command = %q, want claude", got.Command)
	}
}

func TestSandboxDispatcher_AdoptFrame_DirectMode(t *testing.T) {
	direct := &fakeDispatcher{}
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "direct"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: direct}

	_, _, err := d.AdoptFrame(context.Background(), "f1", "/workspace/foo")
	if err != nil {
		t.Fatalf("AdoptFrame error: %v", err)
	}
	if !direct.adoptCalled {
		t.Error("expected direct.AdoptFrame called")
	}
}

type fakeEnsureDispatcher struct {
	fakeDispatcher
	ensureCalled bool
	ensureErr    error
}

func (f *fakeEnsureDispatcher) EnsureProject(_ context.Context, _ string) error {
	f.ensureCalled = true
	return f.ensureErr
}

func TestSandboxDispatcher_EnsureProject_DirectMode(t *testing.T) {
	direct := &fakeEnsureDispatcher{}
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "direct"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: direct}

	if err := d.EnsureProject(context.Background(), "/p"); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if !direct.ensureCalled {
		t.Errorf("direct.EnsureProject not called")
	}
}

func TestSandboxDispatcher_EnsureProject_DevcontainerWithoutBackend_NoOp(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: &fakeEnsureDispatcher{}}

	if err := d.EnsureProject(context.Background(), "/p"); err != nil {
		t.Errorf("EnsureProject without backend: %v, want nil", err)
	}
}

func TestSandboxDispatcher_EnsureProject_UnknownMode(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "vagrant"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: &fakeEnsureDispatcher{}}
	if err := d.EnsureProject(context.Background(), "/p"); err == nil {
		t.Errorf("unknown mode must error")
	}
}

func TestSandboxDispatcher_IsContainer_NoBackend(t *testing.T) {
	d := &agentlaunch.SandboxDispatcher{
		Resolver: config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"}),
	}
	if d.IsContainer("/p") {
		t.Errorf("IsContainer must return false when no devcontainer backend")
	}
}

func TestSandboxDispatcher_AdoptFrame_DevcontainerWithoutBackend(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &agentlaunch.SandboxDispatcher{Resolver: resolver, Direct: &fakeDispatcher{}}
	cleanup, mounts, err := d.AdoptFrame(context.Background(), "f1", "/p")
	if err != nil || cleanup != nil || mounts != nil {
		t.Errorf("nil-backend devcontainer adopt: got cleanup-set=%v mounts=%v err=%v",
			cleanup != nil, mounts, err)
	}
}

func TestSandboxDispatcher_AdoptFrame_UnknownMode(t *testing.T) {
	d := &agentlaunch.SandboxDispatcher{
		Resolver: config.NewSandboxResolver(config.SandboxConfig{Mode: "vagrant"}),
		Direct:   &fakeDispatcher{},
	}
	_, _, err := d.AdoptFrame(context.Background(), "f1", "/p")
	if err == nil {
		t.Errorf("unknown mode must error")
	}
}
