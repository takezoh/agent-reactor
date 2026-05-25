package credproxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
)

type stubProvider struct {
	name string
	spec container.Spec
	err  error
}

func (s *stubProvider) Name() string                 { return s.name }
func (s *stubProvider) Init() error                  { return nil }
func (s *stubProvider) Routes() []credproxylib.Route { return nil }
func (s *stubProvider) ContainerSpec(_ context.Context, _ string) (container.Spec, error) {
	return s.spec, s.err
}

// closerProvider is a stubProvider that also implements io.Closer, used to
// verify Shutdown invokes the optional Closer hook.
type closerProvider struct {
	stubProvider
	closed *bool
}

func (c *closerProvider) Close() error { *c.closed = true; return nil }

func TestRunnerShutdownCancelsWaitsAndCloses(t *testing.T) {
	done := make(chan struct{})
	cancelled := false
	closed := false
	r := &Runner{
		srvCancel:  func() { cancelled = true; close(done) },
		serverDone: done,
		providers:  []container.Provider{&closerProvider{stubProvider: stubProvider{name: "c"}, closed: &closed}},
	}

	r.Shutdown(context.Background())

	if !cancelled {
		t.Error("srvCancel was not called")
	}
	if !closed {
		t.Error("provider Close was not called")
	}
}

func TestRunnerShutdownNilSafe(t *testing.T) {
	// A Runner whose Start never completed has nil srvCancel/serverDone.
	(&Runner{}).Shutdown(context.Background())
}

func TestRunnerShutdownRespectsCtxWhenServerHangs(t *testing.T) {
	r := &Runner{srvCancel: func() {}, serverDone: make(chan struct{})} // never closes
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { r.Shutdown(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not return when ctx was cancelled and server hung")
	}
}

func TestRunner_ContainerSpec_MergesProviders(t *testing.T) {
	r := &Runner{
		providers: []container.Provider{
			&stubProvider{name: "p1", spec: container.Spec{
				Env:    map[string]string{"KEY_A": "val_a"},
				Mounts: []string{"/host/a:/container/a"},
			}},
			&stubProvider{name: "p2", spec: container.Spec{
				Env:    map[string]string{"KEY_B": "val_b"},
				Mounts: []string{"/host/b:/container/b"},
			}},
		},
	}

	out, err := r.ContainerSpec(context.Background(), "/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Env["KEY_A"] != "val_a" {
		t.Errorf("KEY_A = %q, want val_a", out.Env["KEY_A"])
	}
	if out.Env["KEY_B"] != "val_b" {
		t.Errorf("KEY_B = %q, want val_b", out.Env["KEY_B"])
	}
	if len(out.Mounts) != 2 {
		t.Errorf("Mounts len = %d, want 2: %v", len(out.Mounts), out.Mounts)
	}
}

func TestRunner_ContainerSpec_SkipsFailingProvider(t *testing.T) {
	r := &Runner{
		providers: []container.Provider{
			&stubProvider{name: "good", spec: container.Spec{
				Env: map[string]string{"KEY_OK": "ok"},
			}},
			&stubProvider{name: "bad", err: errors.New("provider down")},
		},
	}

	out, err := r.ContainerSpec(context.Background(), "/project")
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if out.Env["KEY_OK"] != "ok" {
		t.Errorf("KEY_OK = %q, want ok", out.Env["KEY_OK"])
	}
}

func TestRunner_ContainerSpec_EmptyProviders(t *testing.T) {
	r := &Runner{}
	out, err := r.ContainerSpec(context.Background(), "/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Env) != 0 {
		t.Errorf("Env = %v, want empty", out.Env)
	}
}
