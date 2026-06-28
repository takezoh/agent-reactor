package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/takezoh/agent-reactor/client/driver/vt"
	"github.com/takezoh/agent-reactor/client/state"
)

// tapEntry holds the cancel function and frame target for one running tap.
type tapEntry struct {
	cancel context.CancelFunc
	target string
}

// tapManager starts and stops FrameTap reader goroutines per frame.
// All methods must be called from the event loop goroutine.
type tapManager struct {
	tap     FrameTap
	ctx     context.Context
	cancels map[state.FrameID]tapEntry
}

func newTapManager(ctx context.Context, tap FrameTap) *tapManager {
	return &tapManager{
		tap:     tap,
		ctx:     ctx,
		cancels: map[state.FrameID]tapEntry{},
	}
}

// start begins a tap for the given (frameID, target) pair. If a tap already
// exists for frameID it is stopped first.
func (m *tapManager) start(frameID state.FrameID, target string, enqueue func(state.Event)) {
	if m.tap == nil {
		return
	}
	m.stop(frameID)

	tapCtx, cancel := context.WithCancel(m.ctx)
	ch, err := m.tap.Start(tapCtx, target)
	if err != nil {
		slog.Warn("frametap: start failed", "frame", frameID, "target", target, "err", err)
		cancel()
		return
	}
	slog.Info("frametap: started", "frame", frameID, "target", target)
	m.cancels[frameID] = tapEntry{cancel: cancel, target: target}
	go readTap(tapCtx, frameID, target, ch, enqueue)
}

// stop cancels the reader goroutine and stops the underlying tap forwarder.
func (m *tapManager) stop(frameID state.FrameID) {
	entry, ok := m.cancels[frameID]
	if !ok {
		return
	}
	entry.cancel()
	if err := m.tap.Stop(entry.target); err != nil {
		slog.Debug("frametap: stop failed", "frame", frameID, "target", entry.target, "err", err)
	}
	delete(m.cancels, frameID)
}

// stopAll cancels all running taps. Called on daemon shutdown.
func (m *tapManager) stopAll() {
	for id := range m.cancels {
		m.stop(id)
	}
}

// parseOscNotification extracts (title, body) from a vt.OscNotification.
func parseOscNotification(n vt.OscNotification) (title, body string) {
	switch n.Cmd {
	case 9:
		return strings.TrimSpace(n.Payload), ""
	case 777:
		parts := strings.SplitN(n.Payload, ";", 3)
		if len(parts) >= 3 {
			return parts[1], parts[2]
		}
		if len(parts) == 2 {
			return parts[1], ""
		}
	case 99:
		for _, part := range strings.Split(n.Payload, ":") {
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			switch k {
			case "d":
				title = v
			case "p":
				body = v
			}
		}
		if title == "" && body == "" {
			body = n.Payload
		}
	}
	return title, body
}

// vtPromptPhase converts a vt.PromptPhase to its state equivalent.
// The two enums are defined independently (state must not import vt),
// so an explicit switch ensures they stay in sync even if one is reordered.
func vtPromptPhase(p vt.PromptPhase) state.PromptPhase {
	switch p {
	case vt.PromptPhaseStart:
		return state.PromptPhaseStart
	case vt.PromptPhaseInput:
		return state.PromptPhaseInput
	case vt.PromptPhaseCommand:
		return state.PromptPhaseCommand
	case vt.PromptPhaseComplete:
		return state.PromptPhaseComplete
	default:
		return state.PromptPhaseNone
	}
}

// newFrameTapTerminal creates a VT emulator wired to emit EvFrameOsc and
// EvFramePrompt events via enqueue. Minimal 1×1 dimensions are used because
// the emulator is only needed for OSC sequence extraction, not rendering.
func newFrameTapTerminal(frameID state.FrameID, enqueue func(state.Event)) *vt.Terminal {
	term := vt.New(1, 1)
	term.OnWindowTitle = func(cmd int, title string) {
		if title != "" {
			enqueue(state.EvFrameOsc{FrameID: frameID, Cmd: cmd, Title: title, Now: time.Now()})
		}
	}
	term.OnOscNotification = func(n vt.OscNotification) {
		title, body := parseOscNotification(n)
		if title != "" || body != "" {
			enqueue(state.EvFrameOsc{FrameID: frameID, Cmd: n.Cmd, Title: title, Body: body, Now: time.Now()})
		}
	}
	term.OnPromptEvent = func(e vt.PromptEvent) {
		enqueue(state.EvFramePrompt{FrameID: frameID, Phase: vtPromptPhase(e.Phase), ExitCode: e.ExitCode, Now: time.Now()})
	}
	return term
}

// readTap feeds raw bytes from ch into a VT emulator and enqueues EvFrameOsc
// and EvFramePrompt events for each OSC sequence detected.
// Runs in its own goroutine; exits when ch is closed or ctx is cancelled.
func readTap(ctx context.Context, frameID state.FrameID, target string, ch <-chan []byte, enqueue func(state.Event)) {
	term := newFrameTapTerminal(frameID, enqueue)
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			feedSafe(frameID, target, term, data)
		case <-ctx.Done():
			return
		}
	}
}

// feedSafe drives the VT emulator with one chunk and recovers any panic.
// vt.New(1,1) panics on ESC M / CSI M / DECRC via InsertLineArea out-of-bounds;
// emulator handler state remains valid after the panic so the next chunk is safe.
func feedSafe(frameID state.FrameID, target string, term *vt.Terminal, data []byte) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("frametap: vt emulator panic, skipping chunk",
				"frame", frameID, "target", target,
				"err", fmt.Sprintf("%v", rec),
				"len", len(data))
		}
	}()
	if err := term.Feed(data); err != nil {
		slog.Debug("frametap: feed error", "frame", frameID, "target", target, "err", err)
	}
}
