package vt

import (
	"bytes"
	"strconv"
	"strings"

	xvt "github.com/charmbracelet/x/vt"
)

// Terminal wraps charmbracelet/x/vt Emulator and delivers OSC events via
// callbacks as they arrive in the byte stream. Callbacks fire synchronously
// during Feed — no batching, no Snapshot needed.
//
// All methods must be called from the same goroutine. No internal locking is
// added; callers own the concurrency model.
type Terminal struct {
	em *xvt.Emulator

	// Callbacks fired synchronously during Feed. nil callbacks are ignored.
	OnOscNotification func(OscNotification)
	OnPromptEvent     func(PromptEvent)
	OnWindowTitle     func(cmd int, title string)
}

// New creates a Terminal sized cols×rows. Defaults to 80×24 for zero values.
func New(cols, rows int) *Terminal {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	t := &Terminal{em: xvt.NewEmulator(cols, rows)}
	t.registerOscHandlers()
	return t
}

// Resize adjusts the emulator viewport to cols×rows.
func (t *Terminal) Resize(cols, rows int) {
	t.em.Resize(cols, rows)
}

// Feed writes raw ANSI bytes into the emulator. OSC callbacks fire
// synchronously for each sequence encountered.
func (t *Terminal) Feed(data []byte) error {
	_, err := t.em.Write(data)
	return err
}

// Reset clears the emulator buffer and re-registers OSC handlers.
func (t *Terminal) Reset() {
	cols, rows := t.em.Width(), t.em.Height()
	t.em = xvt.NewEmulator(cols, rows)
	t.registerOscHandlers()
}

func oscStripPrefix(data []byte) string {
	if i := bytes.IndexByte(data, ';'); i >= 0 {
		return string(data[i+1:])
	}
	return string(data)
}

func (t *Terminal) registerOscHandlers() {
	t.em.RegisterOscHandler(0, func(data []byte) bool {
		if t.OnWindowTitle != nil {
			t.OnWindowTitle(0, oscStripPrefix(data))
		}
		return false
	})
	t.em.RegisterOscHandler(2, func(data []byte) bool {
		if t.OnWindowTitle != nil {
			t.OnWindowTitle(2, oscStripPrefix(data))
		}
		return false
	})

	for _, cmd := range []int{9, 99, 777} {
		t.em.RegisterOscHandler(cmd, func(data []byte) bool {
			if t.OnOscNotification != nil {
				t.OnOscNotification(OscNotification{Cmd: cmd, Payload: oscStripPrefix(data)})
			}
			return false
		})
	}

	t.em.RegisterOscHandler(133, t.handleOsc133)
}

// handleOsc133 processes OSC 133 semantic shell prompts.
// Payload format after stripping "133;": A | B | C | D[;<exit-code>]
func (t *Terminal) handleOsc133(data []byte) bool {
	parts := strings.SplitN(oscStripPrefix(data), ";", 2)
	if len(parts) == 0 {
		return false
	}
	var ev PromptEvent
	switch parts[0] {
	case "A":
		ev.Phase = PromptPhaseStart
	case "B":
		ev.Phase = PromptPhaseInput
	case "C":
		ev.Phase = PromptPhaseCommand
	case "D":
		ev.Phase = PromptPhaseComplete
		if len(parts) == 2 {
			if code, err := strconv.Atoi(parts[1]); err == nil {
				ev.ExitCode = &code
			}
		}
	default:
		return false
	}
	if t.OnPromptEvent != nil {
		t.OnPromptEvent(ev)
	}
	return false
}
