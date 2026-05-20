// Package framereg provides a single-writer/multi-reader registry that maps
// container bearer tokens and bind-mount tables to frame IDs.
//
// The event loop is the sole writer (Register, RegisterWithMounts, StoreMounts, Delete).
// Container endpoint HTTP handlers read concurrently (Lookup, GetMounts).
// A single RWMutex owned by this package is the only lock in the runtime root
// call path, keeping the "No mutexes outside sources" principle intact.
package framereg

import (
	"sync"

	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/state"
)

// Registry maps container bearer tokens to frame IDs and holds per-frame
// bind-mount tables. The event loop is the single writer; container endpoint
// HTTP handlers read concurrently.
type Registry struct {
	mu           sync.RWMutex
	frameToToken map[state.FrameID]string
	tokenToFrame map[string]state.FrameID
	mounts       map[state.FrameID]pathmap.Mounts
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		frameToToken: make(map[state.FrameID]string),
		tokenToFrame: make(map[string]state.FrameID),
		mounts:       make(map[state.FrameID]pathmap.Mounts),
	}
}

// Register stores token for frameID, replacing any prior token.
// Must be called from the single writer (event loop).
func (reg *Registry) Register(frameID state.FrameID, token string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if old, ok := reg.frameToToken[frameID]; ok {
		delete(reg.tokenToFrame, old)
	}
	reg.frameToToken[frameID] = token
	reg.tokenToFrame[token] = frameID
}

// RegisterWithMounts atomically stores the token and bind-mount table for
// frameID under a single lock, eliminating the window between Register and
// StoreMounts where a concurrent reader would see the token but not the mounts.
// Must be called from the single writer (event loop).
func (reg *Registry) RegisterWithMounts(frameID state.FrameID, token string, mounts pathmap.Mounts) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if old, ok := reg.frameToToken[frameID]; ok {
		delete(reg.tokenToFrame, old)
	}
	reg.frameToToken[frameID] = token
	reg.tokenToFrame[token] = frameID
	if len(mounts) > 0 {
		reg.mounts[frameID] = mounts
	}
}

// StoreMounts associates mounts with frameID.
// Must be called from the single writer (event loop).
func (reg *Registry) StoreMounts(frameID state.FrameID, mounts pathmap.Mounts) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.mounts[frameID] = mounts
}

// Lookup returns the FrameID associated with token, or ("", false).
// Safe for concurrent reads.
func (reg *Registry) Lookup(token string) (state.FrameID, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	id, ok := reg.tokenToFrame[token]
	return id, ok
}

// GetMounts returns the bind-mount table for frameID, or (nil, false).
// Safe for concurrent reads.
func (reg *Registry) GetMounts(frameID state.FrameID) (pathmap.Mounts, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	ms, ok := reg.mounts[frameID]
	return ms, ok
}

// Delete removes the token and mounts associated with frameID.
// Must be called from the single writer (event loop).
func (reg *Registry) Delete(frameID state.FrameID) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if token, ok := reg.frameToToken[frameID]; ok {
		delete(reg.tokenToFrame, token)
		delete(reg.frameToToken, frameID)
	}
	delete(reg.mounts, frameID)
}
