package termvt

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/x/vt"
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
}

// Session is one pty-backed program whose output is parsed by a server-side VT
// emulator (for OSC handling and reattach snapshots) and fanned out to any
// number of subscribers.
//
// Single-writer discipline: readLoop is the only writer of the emulator and the
// only producer of events. mu guards the emulator, the subscriber set, and the
// pending control buffer together, so a Subscribe snapshot is atomic with
// respect to live writes.
type Session struct {
	ptmx *os.File
	cmd  *exec.Cmd
	em   *vt.Emulator

	mu      sync.Mutex
	subs    map[int]chan Event
	pending []Control // produced by OSC handlers during em.Write
	nextID  int
	cols    int
	rows    int
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
	s := &Session{
		ptmx: ptmx,
		cmd:  cmd,
		em:   vt.NewEmulator(cols, rows),
		subs: map[int]chan Event{},
		cols: cols,
		rows: rows,
	}
	s.registerOSC()
	go s.readLoop()
	return s, nil
}

// registerOSC wires the server-side OSC "tee": semantic sequences are captured
// here and surfaced as Control events instead of being left in the raw stream.
// Handlers run synchronously inside em.Write (mu held) and append to pending.
func (s *Session) registerOSC() {
	s.em.SetCallbacks(vt.Callbacks{
		Title: func(t string) { s.pending = append(s.pending, Control{Kind: "title", Data: t}) },
		Bell:  func() { s.pending = append(s.pending, Control{Kind: "bell"}) },
	})
	// OSC 9: desktop notification — captured so it reaches the operating client
	// rather than firing on the server host.
	s.em.RegisterOscHandler(9, func(data []byte) bool {
		s.pending = append(s.pending, Control{Kind: "osc", Code: 9, Data: oscText(data)})
		return true
	})
	// OSC 133: shell prompt / command markers — drives run-state detection.
	s.em.RegisterOscHandler(133, func(data []byte) bool {
		s.pending = append(s.pending, Control{Kind: "prompt", Code: 133, Data: oscText(data)})
		return true
	})
}

func (s *Session) readLoop() {
	buf := make([]byte, readChunk)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			s.pending = s.pending[:0]
			_, _ = s.em.Write(chunk) // fires OSC handlers → append to pending
			for _, c := range s.pending {
				s.fanout(Event{Kind: EventControl, Ctl: c})
			}
			s.fanout(Event{Kind: EventOutput, Data: chunk})
			s.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	_ = s.cmd.Wait() // reap the process so it does not linger as a zombie
	s.mu.Lock()
	s.fanout(Event{Kind: EventExit})
	for id, ch := range s.subs {
		close(ch)
		delete(s.subs, id)
	}
	s.mu.Unlock()
}

// fanout delivers an event to every subscriber. Caller must hold mu. A
// subscriber whose buffer is full is disconnected (channel closed) rather than
// having events silently dropped — dropping mid-stream would corrupt its
// terminal; the client reconnects and resyncs from a fresh snapshot.
func (s *Session) fanout(ev Event) {
	for id, ch := range s.subs {
		select {
		case ch <- ev:
		default:
			slog.Warn("termvt: subscriber too slow, disconnecting", "sub", id)
			close(ch)
			delete(s.subs, id)
		}
	}
}

// Subscribe registers a client and returns its id and event channel. The first
// event is a reattach snapshot of the current screen, captured atomically with
// respect to live writes.
func (s *Session) Subscribe() (int, <-chan Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	s.nextID++
	ch := make(chan Event, subBuffer)
	ch <- Event{Kind: EventOutput, Data: []byte(s.em.Render())}
	s.subs[id] = ch
	return id, ch
}

// Unsubscribe drops a subscriber and closes its channel.
func (s *Session) Unsubscribe(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.subs[id]; ok {
		close(ch)
		delete(s.subs, id)
	}
}

// WriteInput forwards client keystrokes to the pty.
func (s *Session) WriteInput(b []byte) {
	_, _ = s.ptmx.Write(b)
}

// Resize updates the pty window size and the emulator grid.
func (s *Session) Resize(cols, rows int) error {
	cols, rows = normalizeSize(cols, rows)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cols, s.rows = cols, rows
	s.em.Resize(cols, rows)
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Snapshot returns the current rendered screen (used for reattach).
func (s *Session) Snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []byte(s.em.Render())
}

// Size returns the current terminal dimensions.
func (s *Session) Size() (cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cols, s.rows
}

// Close kills the process and closes the pty. readLoop then emits EventExit and
// closes subscriber channels.
func (s *Session) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.ptmx.Close()
}

func normalizeSize(cols, rows int) (int, int) {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	return cols, rows
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
