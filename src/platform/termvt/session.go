package termvt

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

const (
	subBuffer = 256
	readChunk = 32 * 1024
)

// Spec describes a command to run in a pty.
type Spec struct {
	Argv []string // command + args; Argv[0] is the executable
	Env  []string // full environment; defaults to os.Environ() when nil
	Cols int
	Rows int
	// ScrollbackLines bounds the server-side VT scrollback buffer. Zero
	// leaves the underlying emulator's default in place (xvt currently
	// defaults to 10000). New subscribers receive this buffer as the first
	// seed frame so they can scroll up through history written before they
	// attached.
	ScrollbackLines int
}

// Session is one pty-backed program whose output is parsed by a server-side
// VT emulator (for OSC handling and reattach snapshots) and fanned out to any
// number of subscribers.
//
// Concurrency model: Session is an actor. All emulator state, subscriber
// state, and dimensions are owned exclusively by mainLoop; external callers
// reach them by sending sessionCmd values on cmdCh and reading a per-call
// reply channel. Three goroutines per Session:
//
//   - readerLoop:   pty.Read → chunkCh                  (kernel back-pressure)
//   - responseLoop: io.Copy(pty, em)                    (drains CSI replies)
//   - mainLoop:     select { chunkCh | cmdCh }          (sole state owner)
//
// Exit status is published on atomics so the runtime's dispatch goroutine
// can poll ExitCode without ever entering mainLoop — a slow chunk parse
// must never freeze IPC.
type Session struct {
	pty PTY
	cmd *exec.Cmd
	em  Emulator

	chunkCh chan []byte     // capacity 1; readerLoop → mainLoop
	cmdCh   chan sessionCmd // unbuffered RPC rendezvous
	done    chan struct{}   // closed by mainLoop on shutdown

	// Exit state lives on atomics so ExitCode() is lock-free. Stored in this
	// order on exit (exitCode first, then exited): a reader that observes
	// exited == true is guaranteed to read the final exitCode via the
	// release/acquire barrier on the second atomic store.
	exited   atomic.Bool
	exitCode atomic.Int32

	closeOnce sync.Once
}

// NewSession starts spec.Argv in a pty sized cols×rows and begins streaming.
func NewSession(spec Spec) (*Session, error) {
	if len(spec.Argv) == 0 {
		return nil, fmt.Errorf("termvt: empty argv")
	}
	cols, rows := normalizeSize(spec.Cols, spec.Rows)
	cmd := exec.Command(spec.Argv[0], spec.Argv[1:]...) //nolint:gosec // caller-supplied agent command
	cmd.Env = withTerm(spec.Env)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("termvt: pty start: %w", err)
	}
	em := emulatorFor(cols, rows)
	if spec.ScrollbackLines > 0 {
		em.SetScrollbackSize(spec.ScrollbackLines)
	}
	return startSession(em, realPTY{ptmx}, cmd, cols, rows), nil
}

// NewSessionWithDeps constructs a Session against caller-supplied dependencies
// instead of spawning a real pty. Intended for tests that want to drive the
// actor deterministically — a fake Emulator can return canned bytes from
// Read() and capture writes, and a fake PTY (typically an io.Pipe pair) lets
// the test feed chunks into readerLoop without forking a child. The cmd
// argument may be nil; Close() handles it.
func NewSessionWithDeps(em Emulator, p PTY, cmd *exec.Cmd, cols, rows int) *Session {
	cols, rows = normalizeSize(cols, rows)
	return startSession(em, p, cmd, cols, rows)
}

func startSession(em Emulator, p PTY, cmd *exec.Cmd, cols, rows int) *Session {
	s := &Session{
		pty:     p,
		cmd:     cmd,
		em:      em,
		chunkCh: make(chan []byte, 1),
		cmdCh:   make(chan sessionCmd),
		done:    make(chan struct{}),
	}
	go s.mainLoop(cols, rows)
	go s.readerLoop()
	go s.responseLoop()
	return s
}

// call is the single rendezvous through which every RPC reaches mainLoop. It
// allocates a buffered reply channel, hands it (via mk) to the cmd's
// constructor, posts the cmd, and waits for either the reply or shutdown.
// Centralising this one pattern pins the "every actor RPC must honor s.done"
// contract in exactly one place — a future RPC that uses call cannot forget
// the shutdown branch.
func call[R any](s *Session, mk func(chan R) sessionCmd, onShutdown R) R {
	reply := make(chan R, 1)
	if !s.send(mk(reply)) {
		return onShutdown
	}
	select {
	case r := <-reply:
		return r
	case <-s.done:
		return onShutdown
	}
}

// Subscribe registers a client and returns its id and event channel. The first
// event is a reattach snapshot of the current screen, captured atomically with
// respect to live writes by virtue of mainLoop processing it between chunks.
//
// If the Session has already shut down, Subscribe returns id 0 (the actor
// allocates ids ≥ 1, so 0 is an unambiguous shutdown sentinel) and a closed
// channel. This is strictly better than the pre-actor behaviour where a
// post-exit Subscribe leaked a goroutine waiting on events that never came.
func (s *Session) Subscribe() (int, <-chan Event) {
	r := call(s, func(ch chan subscribeReply) sessionCmd {
		return subscribeCmd{reply: ch}
	}, subscribeReply{})
	if r.ch == nil {
		return 0, closedEventChan()
	}
	return r.id, r.ch
}

// Unsubscribe drops a subscriber and closes its channel. Safe to call after
// shutdown (no-op).
func (s *Session) Unsubscribe(id int) {
	_ = call(s, func(ch chan struct{}) sessionCmd {
		return unsubscribeCmd{id: id, reply: ch}
	}, struct{}{})
}

// WriteInput forwards client keystrokes to the pty. It bypasses mainLoop
// because keystrokes never need to consult emulator state. Writes from
// responseLoop and WriteInput share the pty master fd: small payloads are
// safe by kernel PIPE_BUF atomicity, but a paste > PIPE_BUF could interleave
// with a concurrent CSI reply — that's an existing pty-master multiplexing
// concern unchanged by this refactor.
func (s *Session) WriteInput(b []byte) error {
	_, err := s.pty.Write(b)
	return err
}

// Resize updates the pty window size and the emulator grid in lockstep — the
// emulator must agree on dimensions with the kernel pty winsize the child
// reads via TIOCGWINSZ.
func (s *Session) Resize(cols, rows int) error {
	return call(s, func(ch chan error) sessionCmd {
		return resizeCmd{cols: cols, rows: rows, reply: ch}
	}, os.ErrClosed)
}

// Snapshot returns the current rendered screen (used for reattach).
func (s *Session) Snapshot() []byte {
	return call(s, func(ch chan []byte) sessionCmd {
		return snapshotCmd{reply: ch}
	}, nil)
}

// Size returns the current terminal dimensions.
func (s *Session) Size() (cols, rows int) {
	v := call(s, func(ch chan [2]int) sessionCmd {
		return sizeCmd{reply: ch}
	}, [2]int{})
	return v[0], v[1]
}

// ExitCode reports the process exit code once it has been reaped. exited is
// false while the process is still running (code is then meaningless and 0).
//
// Lock-free by design: the runtime's single dispatch goroutine polls this
// every tick via FrameAlive; routing it through cmdCh would let any caller
// that monopolises mainLoop (e.g. a slow chunk parse) freeze the entire
// IPC. The atomic load on `exited` synchronizes-with the matching store in
// handleExit, so a reader who observes exited == true is guaranteed to see
// the final exitCode value.
func (s *Session) ExitCode() (code int, exited bool) {
	exited = s.exited.Load()
	code = int(s.exitCode.Load())
	return code, exited
}

// CaptureTail returns the trailing n rendered lines of the session's screen
// with terminal escape sequences stripped. PtyBackend.CaptureFrame uses this to
// read plain text out of the emulator grid.
func CaptureTail(s *Session, n int) string {
	return stripSGRTail(string(s.Snapshot()), n)
}

// Close kills the process and tears the actor down. Idempotent. Order: kill
// → pty.Close (readerLoop unblocks → chunkCh closes → mainLoop's handleExit
// runs → atomics set + EventExit fanned out) → em.CloseInputPipe
// (responseLoop unblocks via io.EOF on em.Read without racing on the
// emulator's internal `closed` flag).
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.cmd != nil && s.cmd.Process != nil {
			// The process may already have exited (handleExit reaped it);
			// ErrProcessDone is expected and uninteresting. Surface anything
			// else.
			if killErr := s.cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				slog.Warn("termvt: kill process", "err", killErr)
			}
		}
		err = s.pty.Close()
		// Unblock responseLoop's em.Read by closing the emulator's input
		// pipe. We deliberately avoid em.Close: upstream vt.Emulator.Close
		// writes an unsynchronised `closed` boolean that races with the
		// parked Read, and we don't need that field set — the io.EOF on
		// the pipe is the only signal responseLoop needs.
		_ = s.em.CloseInputPipe()
	})
	return err
}

// send posts an actor command, returning false if the Session has already
// shut down. Caller still has to wait on the per-command reply.
func (s *Session) send(cmd sessionCmd) bool {
	select {
	case s.cmdCh <- cmd:
		return true
	case <-s.done:
		return false
	}
}

// closedEventChan returns a pre-closed channel of Event, used as the "session
// is shut down" return from public methods that hand back a channel.
func closedEventChan() <-chan Event {
	ch := make(chan Event)
	close(ch)
	return ch
}

// maxDim caps terminal dimensions. A client controls cols/rows (session
// create and resize), so without an upper bound a hostile value would either
// overflow the uint16 pty winsize fields (65536 wraps to 0 → a zero-width
// pty) or drive the VT emulator toward an enormous grid allocation (OOM).
// 2000 is far beyond any real multi-monitor terminal.
const maxDim = 2000

func normalizeSize(cols, rows int) (int, int) {
	return clampDim(cols, 80), clampDim(rows, 24)
}

// clampDim floors a non-positive dimension to def and caps it at maxDim.
func clampDim(d, def int) int {
	switch {
	case d <= 0:
		return def
	case d > maxDim:
		return maxDim
	default:
		return d
	}
}

func withTerm(env []string) []string {
	if len(env) == 0 {
		env = os.Environ()
	}
	for _, e := range env {
		if strings.HasPrefix(e, "TERM=") {
			return env
		}
	}
	return append(env, "TERM=xterm-256color")
}

// oscText drops the leading "<cmd>;" that x/vt includes in the OSC payload.
func oscText(data []byte) string {
	str := string(data)
	if i := strings.IndexByte(str, ';'); i >= 0 {
		return str[i+1:]
	}
	return str
}

// exitCodeFromWait extracts the process exit code from cmd.Wait()'s error.
// nil → 0 (clean exit); *exec.ExitError → the reported code; any other error
// → -1 (could not determine, e.g. process was signalled before exec).
func exitCodeFromWait(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// stripSGRTail returns the trailing n lines of s with terminal escape
// sequences removed. n <= 0 returns the empty string; n larger than the
// line count returns all lines. ansi.Strip removes every escape class (SGR
// colours/styles, OSC 8 hyperlinks, cursor moves, …) rather than SGR alone,
// so the result is plain text regardless of what vt.Emulator.Render emits.
func stripSGRTail(s string, n int) string {
	if n <= 0 {
		return ""
	}
	clean := ansi.Strip(s)
	lines := strings.Split(clean, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
