package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/takezoh/agent-roost/state"
)

// tokenStore maps container access tokens to frame IDs and back.
// It is safe for concurrent use by the spawn goroutine (writer) and the
// container endpoint accept loop (reader).
type tokenStore struct {
	frameToToken sync.Map // state.FrameID → string
	tokenToFrame sync.Map // string → state.FrameID
}

// Generate creates a new random 32-byte token for frameID, stores it, and
// returns the hex-encoded token. Replaces any existing token for the frame.
func (s *tokenStore) Generate(frameID state.FrameID) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token generate: %w", err)
	}
	token := hex.EncodeToString(b)

	// Revoke any prior token for this frame before storing the new one.
	s.Revoke(frameID)

	s.frameToToken.Store(frameID, token)
	s.tokenToFrame.Store(token, frameID)
	return token, nil
}

// Lookup returns the FrameID associated with token, or ("", false) if the
// token is unknown or has been revoked.
func (s *tokenStore) Lookup(token string) (state.FrameID, bool) {
	v, ok := s.tokenToFrame.Load(token)
	if !ok {
		return "", false
	}
	return v.(state.FrameID), true
}

// Register stores an externally-supplied token for frameID (warm-start path).
// Replaces any prior token for the same frame.
func (s *tokenStore) Register(frameID state.FrameID, token string) {
	s.Revoke(frameID)
	s.frameToToken.Store(frameID, token)
	s.tokenToFrame.Store(token, frameID)
}

// Revoke removes the token associated with frameID from both maps.
// No-op if the frame has no token.
func (s *tokenStore) Revoke(frameID state.FrameID) {
	v, ok := s.frameToToken.LoadAndDelete(frameID)
	if !ok {
		return
	}
	s.tokenToFrame.Delete(v.(string))
}
