package runtime

import (
	"testing"

	"github.com/takezoh/agent-roost/client/state"
)

func TestTokenStoreGenerateLookup(t *testing.T) {
	var s tokenStore
	frameA := state.FrameID("frame-a")

	token, err := s.Generate(frameA)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	got, ok := s.Lookup(token)
	if !ok {
		t.Fatal("Lookup: expected ok=true")
	}
	if got != frameA {
		t.Fatalf("Lookup: got %q, want %q", got, frameA)
	}
}

func TestTokenStoreRevokeInvalidates(t *testing.T) {
	var s tokenStore
	frame := state.FrameID("frame-revoke")

	token, _ := s.Generate(frame)
	s.Revoke(frame)

	if _, ok := s.Lookup(token); ok {
		t.Fatal("Lookup after Revoke: expected ok=false")
	}
}

func TestTokenStoreRevokeNoOp(t *testing.T) {
	var s tokenStore
	// Revoking an unknown frame must not panic.
	s.Revoke(state.FrameID("nonexistent"))
}

func TestTokenStoreFrameIsolation(t *testing.T) {
	var s tokenStore
	frameA := state.FrameID("frame-iso-a")
	frameB := state.FrameID("frame-iso-b")

	tokenA, _ := s.Generate(frameA)
	tokenB, _ := s.Generate(frameB)

	// tokenA must not resolve to frameB and vice versa.
	if got, _ := s.Lookup(tokenA); got != frameA {
		t.Fatalf("tokenA resolves to %q, want %q", got, frameA)
	}
	if got, _ := s.Lookup(tokenB); got != frameB {
		t.Fatalf("tokenB resolves to %q, want %q", got, frameB)
	}

	s.Revoke(frameA)
	if _, ok := s.Lookup(tokenA); ok {
		t.Fatal("tokenA still valid after revoking frameA")
	}
	if _, ok := s.Lookup(tokenB); !ok {
		t.Fatal("tokenB invalidated by unrelated revoke")
	}
}

func TestTokenStoreRegenerateReplacesPrior(t *testing.T) {
	var s tokenStore
	frame := state.FrameID("frame-regen")

	old, _ := s.Generate(frame)
	fresh, _ := s.Generate(frame)

	if _, ok := s.Lookup(old); ok {
		t.Fatal("old token still valid after regeneration")
	}
	if got, ok := s.Lookup(fresh); !ok || got != frame {
		t.Fatalf("fresh token does not resolve to frame: ok=%v got=%q", ok, got)
	}
}

func TestTokenStoreRegisterLookup(t *testing.T) {
	var s tokenStore
	frame := state.FrameID("frame-register")
	token := "externally-supplied-token"

	s.Register(frame, token)

	got, ok := s.Lookup(token)
	if !ok {
		t.Fatal("Lookup after Register: expected ok=true")
	}
	if got != frame {
		t.Fatalf("Lookup: got %q, want %q", got, frame)
	}
}

func TestTokenStoreRegisterRevokeInvalidates(t *testing.T) {
	var s tokenStore
	frame := state.FrameID("frame-register-revoke")
	token := "warm-recovered-token"

	s.Register(frame, token)
	s.Revoke(frame)

	if _, ok := s.Lookup(token); ok {
		t.Fatal("token still valid after Revoke")
	}
}
