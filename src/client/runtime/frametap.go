package runtime

import "context"

// FrameTap is a source of raw terminal byte streams from a backend frame.
// The event loop starts one tap per frame when the frame is registered
// and stops it when the frame is unregistered. The tap byte stream is fed
// into a VT emulator; OSC callbacks enqueue EvFrameOsc and EvFramePrompt events.
//
// PtyFrameTap (see pty_tap.go) is the current implementation, subscribing
// directly to the termvt.Manager that PtyBackend owns.
type FrameTap interface {
	// Start begins delivering raw bytes for frameID into the returned channel.
	// The channel is closed when the tap is stopped or ctx is cancelled.
	Start(ctx context.Context, frameID string) (<-chan []byte, error)
	// Stop ends delivery for frameID and releases all resources.
	Stop(frameID string) error
}
