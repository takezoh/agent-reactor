package main

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

// SPEC §17.5 — the claude-app-server shim emits the same §10.4 event method
// names as the codex app-server protocol, ensuring agent-switch transparency.
// A successful turn must produce ThreadStarted and TurnCompleted notifications
// using the protocol-standard method strings.
func TestSPEC_17_5_AgentSwitchEventParity(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineResultOK},
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	if err := codexclient.Initialize(clientConn); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := codexclient.StartTurn(clientConn, "", "/ws", []byte("hi"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	// The shim must emit the protocol-standard method names — not claude-specific
	// names — so that the orchestrator's agent runner can consume either shim or
	// native codex app-server without changes (§10.4 event parity).
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})
}
