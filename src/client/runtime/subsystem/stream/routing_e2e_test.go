//go:build e2e

package stream

// Opt-in fidelity backstop: runs the routing-isolation invariant against a REAL
// app-server, so the in-process fake (routing_wired_test.go) is validated
// against the wire behaviour it imitates. This is the ADR-0002 guarantee that a
// test passing against the fake means the same thing as a test against a real
// server.
//
// The stream backend fronts the codex app-server *protocol* (WebSocket-over-UDS
// JSON-RPC), not a specific binary, so this is NOT codex-only: any conforming
// app-server is a valid target. Backends are discovered from the environment:
//
//   - REACTOR_E2E_CODEX_BIN      → the codex app-server (convenience alias)
//   - REACTOR_E2E_APPSERVER_BIN  → any other conforming app-server
//     REACTOR_E2E_APPSERVER_NAME → subtest label for it (default "appserver")
//     REACTOR_E2E_APPSERVER_ARGS → extra argv passed to it, space-split
//
// Gated two ways so it never runs in normal CI: the build tag `e2e` excludes
// this file from default builds, and the test skips unless at least one backend
// is configured. See docs/technical/client/stream-backend-e2e.md.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
)

// e2eBackend is one real app-server implementation to validate the fake against.
type e2eBackend struct {
	name string
	bin  string
	args []string
}

// configuredE2EBackends collects every app-server the environment points at.
func configuredE2EBackends() []e2eBackend {
	var out []e2eBackend
	if bin := os.Getenv("REACTOR_E2E_CODEX_BIN"); bin != "" {
		out = append(out, e2eBackend{name: "codex", bin: bin})
	}
	if bin := os.Getenv("REACTOR_E2E_APPSERVER_BIN"); bin != "" {
		name := os.Getenv("REACTOR_E2E_APPSERVER_NAME")
		if name == "" {
			name = "appserver"
		}
		out = append(out, e2eBackend{
			name: name,
			bin:  bin,
			args: strings.Fields(os.Getenv("REACTOR_E2E_APPSERVER_ARGS")),
		})
	}
	return out
}

func TestStreamRoutingE2EIsolation(t *testing.T) {
	backends := configuredE2EBackends()
	if len(backends) == 0 {
		t.Skip("no real app-server configured; set REACTOR_E2E_CODEX_BIN and/or REACTOR_E2E_APPSERVER_BIN")
	}
	for _, be := range backends {
		t.Run(be.name, func(t *testing.T) { runE2EIsolation(t, be) })
	}
}

// runE2EIsolation launches a real app-server and asserts that two frames in
// distinct working dirs each receive only their own turn's output. Distinct
// cwds let the (current) demux disambiguate; the point here is to confirm the
// real server's event stream routes per-frame the way the fake's does — i.e.
// that the fake is faithful, not that the cross-talk bug is absent.
func runE2EIsolation(t *testing.T, be e2eBackend) {
	rt := &recordingRuntime{}
	listen := filepath.Join(t.TempDir(), "appserver-e2e.sock")
	b := New(rt, nil, "sid", "e2e", "/p", be.bin, be.args, "", false, true,
		listen, 30*time.Second)

	ctx := context.Background()
	if err := b.Start(ctx); err != nil {
		t.Fatalf("start real %s app-server: %v", be.name, err)
	}
	t.Cleanup(func() { b.Stop(ctx) })

	type frame struct {
		id     state.FrameID
		marker string
	}
	frames := []frame{{"A", "E2E_MARKER_ALPHA"}, {"B", "E2E_MARKER_BRAVO"}}
	for _, f := range frames {
		dir := t.TempDir()
		prompt := "Reply with exactly this token and nothing else: " + f.marker
		if _, err := b.BindFrame(ctx, subsystem.BindRequest{
			FrameID: f.id,
			Plan:    state.LaunchPlan{StartDir: dir},
			Stdin:   []byte(prompt),
		}); err != nil {
			t.Fatalf("BindFrame(%s): %v", f.id, err)
		}
	}

	// Real model turns are slow; wait generously for both markers to surface.
	deadline := time.Now().Add(3 * time.Minute)
	for len(rt.framesWithMarker(frames[0].marker)) == 0 ||
		len(rt.framesWithMarker(frames[1].marker)) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for both markers; got A=%v B=%v",
				rt.framesWithMarker(frames[0].marker), rt.framesWithMarker(frames[1].marker))
		}
		time.Sleep(500 * time.Millisecond)
	}

	for _, f := range frames {
		assertMarkerFrames(t, rt, f.marker, f.id)
	}
}
