package termvt

import (
	"io"
	"os"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Emulator is the subset of *vt.Emulator that Session uses. The interface
// exists so tests can drive Session deterministically through
// NewSessionWithDeps without spawning a real pty — fakes return canned
// Render / Read / Write results and capture OSC handler registrations.
//
// Note CloseInputPipe instead of io.Closer: vt.Emulator.Close() writes an
// internal `closed` boolean field that vt.Emulator.Read() reads without
// synchronization, so the race detector flags a benign-but-real race on
// every shutdown. Closing only the input pipe wakes a parked Read() with
// io.EOF via the io.Pipe contract without touching the racy field.
//
// SerializeScrollback returns the ANSI-styled bytes of every line currently
// in the emulator's scrollback buffer (lines that have scrolled off the top
// of the visible grid). The format matches Render(): `\n`-separated rows
// with SGR escapes inline, no cursor positioning, no clear sequences. An
// empty buffer returns nil so callers can cheaply skip emitting a frame.
// SetScrollbackSize bounds that buffer in lines; 0 leaves the underlying
// emulator's default in place.
type Emulator interface {
	io.Writer                                           // shell output bytes go in
	io.Reader                                           // CSI reply bytes come out — drained back into the pty
	Render() string                                     // rendered grid for reattach snapshots
	Resize(cols, rows int)                              // grid dimension change
	SetCallbacks(cb vt.Callbacks)                       // title / bell hooks
	RegisterOscHandler(code int, handler vt.OscHandler) // OSC 9 / 133 hooks
	CloseInputPipe() error                              // shutdown signal — unblocks Read without racing
	SetScrollbackSize(maxLines int)                     // configure scrollback depth
	SerializeScrollback() []byte                        // ANSI-styled scrollback (nil when empty)
	CursorPosition() (x, y int)                         // 0-based cursor (x=col, y=row) within the visible grid
}

// PTY is the subset of *os.File + pty.Setsize that Session needs. Same
// rationale as Emulator: tests substitute an io.Pipe-backed fake so the
// actor loop's read / write paths can be driven without a real terminal.
type PTY interface {
	io.ReadWriteCloser
	SetSize(cols, rows int) error
}

// emulatorFor returns the production emulator with all OSC / callback wiring
// matched to the runtime contract.
func emulatorFor(cols, rows int) Emulator {
	return realEmulator{Emulator: vt.NewEmulator(cols, rows)}
}

// realEmulator wraps *vt.Emulator and adds CloseInputPipe. Embedding the
// pointer satisfies the io.Reader/Writer + Render/Resize/SetCallbacks/
// RegisterOscHandler + SetScrollbackSize methods promoted from *vt.Emulator.
type realEmulator struct {
	*vt.Emulator
}

// CursorPosition adapts *vt.Emulator.CursorPosition (which returns uv.Position)
// to the (x, y int) interface shape so callers do not need to import
// ultraviolet just to read two ints. x is the 0-based column, y the 0-based row
// within the visible grid.
func (e realEmulator) CursorPosition() (x, y int) {
	p := e.Emulator.CursorPosition()
	return p.X, p.Y
}

func (e realEmulator) CloseInputPipe() error {
	if c, ok := e.InputPipe().(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// SerializeScrollback renders the live scrollback buffer to a styled string.
// Returns nil when the buffer is empty so subscribeCmd can skip emitting a
// frame. The serialization format mirrors *vt.Emulator.Render() (which is
// `Lines(buf.Lines).Render()` under the hood) so the seed bytes can be
// concatenated client-side without format-mismatch concerns.
func (e realEmulator) SerializeScrollback() []byte {
	sb := e.Scrollback()
	if sb == nil || sb.Len() == 0 {
		return nil
	}
	return []byte(uv.Lines(sb.Lines()).Render())
}

// realPTY wraps an *os.File (the pty master fd) and adapts pty.Setsize to the
// interface's (cols, rows) shape.
type realPTY struct{ *os.File }

func (p realPTY) SetSize(cols, rows int) error {
	return pty.Setsize(p.File, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}
