package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	codexschemav1 "github.com/takezoh/agent-reactor/platform/agent/codexschema/v1"
)

func newTestTransport(input string, out *bytes.Buffer) codexclient.Transport {
	return codexclient.StdioTransport(strings.NewReader(input), out)
}

// isolateHome points HOME at a temp dir so runWith's logger.Init writes the
// server.log under a writable temp path instead of the developer's (or the
// sandbox's read-only) ~/.agent-reactor. Without this, logger.Init fails, runWith
// returns early before the read loop starts, and any client write blocks
// forever on a pipe with no reader. logger.Init resolves paths via os.UserHomeDir().
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestInitializeResponse(t *testing.T) {
	isolateHome(t)
	var buf bytes.Buffer
	tr := newTestTransport(`{"id":1,"method":"initialize","params":{}}`, &buf)
	code := run(context.Background(), tr)
	if code != 0 {
		t.Fatalf("want 0, got %d", code)
	}

	var msg struct {
		ID     int64           `json:"id"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(&buf).Decode(&msg); err != nil {
		t.Fatalf("parse response: %v; raw: %s", err, &buf)
	}
	if msg.ID != 1 {
		t.Errorf("want id=1, got %d", msg.ID)
	}

	// The response must satisfy the pinned codex InitializeResponse schema:
	// codexHome, platformFamily, platformOs, userAgent are all required.
	var result codexschemav1.InitializeResponse
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.CodexHome == "" || result.PlatformFamily == "" || result.PlatformOS == "" || result.UserAgent == "" {
		t.Errorf("InitializeResponse missing required fields: %+v", result)
	}
}

func TestUnknownMethodError(t *testing.T) {
	isolateHome(t)
	var buf bytes.Buffer
	tr := newTestTransport(`{"id":2,"method":"unknown/method","params":{}}`, &buf)
	run(context.Background(), tr)

	var msg struct {
		ID    int64           `json:"id"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(&buf).Decode(&msg); err != nil {
		t.Fatalf("parse response: %v; raw: %s", err, &buf)
	}
	if msg.ID != 2 {
		t.Errorf("want id=2, got %d", msg.ID)
	}
	if len(msg.Error) == 0 || string(msg.Error) == "null" {
		t.Error("want error field in response for unknown method")
	}
}

func TestEOFExits(t *testing.T) {
	isolateHome(t)
	var buf bytes.Buffer
	tr := newTestTransport("", &buf)
	code := run(context.Background(), tr)
	if code != 0 {
		t.Errorf("want 0 on EOF, got %d", code)
	}
}

func TestContextCancelExits(t *testing.T) {
	isolateHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buf bytes.Buffer
	tr := newTestTransport("", &buf)
	code := run(ctx, tr)
	if code != 0 {
		t.Errorf("want 0 on context cancel, got %d", code)
	}
}
