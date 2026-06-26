package termvt

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/charmbracelet/x/vt"
)

// loopState holds everything mainLoop owns exclusively. It is allocated once
// at mainLoop entry and never escapes — nothing outside this file ever
// dereferences it, so accesses do not need synchronization.
type loopState struct {
	em      Emulator
	pty     PTY
	subs    map[int]chan Event
	pending []Control
	nextID  int
	cols    int
	rows    int
}

// sessionCmd is the actor command interface. Every public Session method that
// needs to touch mainLoop-owned state ships a command implementation, plus a
// per-call reply channel, and mainLoop runs it serially between chunks.
type sessionCmd interface {
	run(ls *loopState)
}

type subscribeReply struct {
	id int
	ch <-chan Event
}

type subscribeCmd struct {
	reply chan subscribeReply
}

// Subscribe seeds the new subscriber's channel with a reattach snapshot so
// the client sees a coherent screen before any live chunk arrives — this
// must happen inside mainLoop so the snapshot is consistent with the next
// chunk's processing.
//
// Seed shape: when the emulator's scrollback buffer holds any rows, they
// are emitted as a first EventOutput frame (terminated with a trailing
// newline so the screen render that follows starts on a fresh row). The
// current visible grid is then emitted as a second EventOutput frame, with
// a trailing CUP escape (\x1b[<y+1>;<x+1>H) that pins xterm.js's cursor to
// the same (x, y) the server-side emulator holds. Both content frames share
// the format documented on Emulator.SerializeScrollback / Render —
// newline-separated rows with inline SGR escapes, no cursor positioning, no
// clear sequences — so a client that writes them in order builds the same
// scrollback its xterm.js would have accumulated had it been attached from
// the start. An empty scrollback (fresh session, or an alt-screen TUI whose
// draws never spilled to history) elides the first frame entirely; the
// screen frame is unconditional.
//
// The trailing CUP is what keeps typed input rendering at the correct
// position after a session switch. Without it, xterm.js's cursor settles at
// the bottom of the rendered grid (Render() emits cell content + '\n'
// separators only) while the PTY's cursor sits at the shell prompt; the
// next echoed character would then be painted at the wrong screen cell.
//
// IDs are allocated starting from 1 so 0 stays reserved as the post-shutdown
// sentinel that Session.Subscribe returns when mainLoop has exited; any
// caller that wants to distinguish "Subscribe came back from the actor" from
// "Subscribe took the shutdown branch" can compare id against 0.
func (c subscribeCmd) run(ls *loopState) {
	ls.nextID++
	id := ls.nextID
	ch := make(chan Event, subBuffer)
	if sb := ls.em.SerializeScrollback(); len(sb) > 0 {
		// Append a separator newline so the screen render's first row does
		// not concatenate onto the last scrollback row when xterm.js writes
		// the two frames back-to-back.
		ch <- Event{Kind: EventOutput, Data: append(sb, '\n')}
	}
	rendered := []byte(ls.em.Render())
	x, y := ls.em.CursorPosition()
	// CUP is 1-based; emulator coords are 0-based.
	rendered = fmt.Appendf(rendered, "\x1b[%d;%dH", y+1, x+1)
	ch <- Event{Kind: EventOutput, Data: rendered}
	ls.subs[id] = ch
	c.reply <- subscribeReply{id: id, ch: ch}
}

type unsubscribeCmd struct {
	id    int
	reply chan struct{}
}

func (c unsubscribeCmd) run(ls *loopState) {
	if ch, ok := ls.subs[c.id]; ok {
		close(ch)
		delete(ls.subs, c.id)
	}
	close(c.reply)
}

type resizeCmd struct {
	cols, rows int
	reply      chan error
}

func (c resizeCmd) run(ls *loopState) {
	cols, rows := normalizeSize(c.cols, c.rows)
	ls.cols, ls.rows = cols, rows
	ls.em.Resize(cols, rows)
	c.reply <- ls.pty.SetSize(cols, rows)
}

type snapshotCmd struct {
	reply chan []byte
}

func (c snapshotCmd) run(ls *loopState) {
	c.reply <- []byte(ls.em.Render())
}

type sizeCmd struct {
	reply chan [2]int
}

func (c sizeCmd) run(ls *loopState) {
	c.reply <- [2]int{ls.cols, ls.rows}
}

// mainLoop is the sole owner of loopState. It services pty chunks (with
// fanout) and actor commands serially. On shutdown — either pty close
// (chunkCh closes) or explicit Session.Close — it reaps the child, sets
// the exit atomics, fans out EventExit, closes all subscribers, then
// closes s.done so any in-flight or future public method unblocks via
// the s.done branch in send() and the reply waits.
func (s *Session) mainLoop(cols, rows int) {
	defer close(s.done)
	ls := &loopState{
		em:   s.em,
		pty:  s.pty,
		subs: map[int]chan Event{},
		cols: cols,
		rows: rows,
	}
	s.registerOSC(ls)
	for {
		select {
		case chunk, ok := <-s.chunkCh:
			if !ok {
				s.handleExit(ls)
				return
			}
			s.processChunk(ls, chunk)
		case cmd := <-s.cmdCh:
			cmd.run(ls)
		}
	}
}

// processChunk applies one pty read to the emulator, then fanouts any Control
// events that the OSC handlers appended during em.Write, then the raw chunk.
// The Control-before-Output ordering is pinned by fanout_contract_test.go —
// observers must see a state update before the bytes that drove it.
func (s *Session) processChunk(ls *loopState, chunk []byte) {
	ls.pending = ls.pending[:0]
	if _, werr := ls.em.Write(chunk); werr != nil {
		slog.Warn("termvt: emulator write error", "err", werr)
	}
	for _, c := range ls.pending {
		fanout(ls, Event{Kind: EventControl, Ctl: c})
	}
	fanout(ls, Event{Kind: EventOutput, Data: chunk})
}

// handleExit reaps the process (if any), publishes the exit atomics, and
// drains the subscriber set. cmd may be nil when Session was constructed via
// NewSessionWithDeps without a real child — in that case the exit code stays
// at the zero default. Atomics are stored before the EventExit fanout so a
// subscriber that observes channel close and then calls ExitCode reads
// consistent values.
func (s *Session) handleExit(ls *loopState) {
	code := 0
	if s.cmd != nil {
		code = exitCodeFromWait(s.cmd.Wait())
	}
	s.exitCode.Store(int32(code))
	s.exited.Store(true)
	fanout(ls, Event{Kind: EventExit})
	for id, ch := range ls.subs {
		close(ch)
		delete(ls.subs, id)
	}
}

// fanout delivers an event to every live subscriber on ls. A subscriber whose
// buffer is full is disconnected (channel closed) rather than having events
// silently dropped — dropping mid-stream would corrupt its terminal; the
// client reconnects and resyncs from a fresh snapshot. Runs only from
// mainLoop (no synchronization needed on ls.subs).
func fanout(ls *loopState, ev Event) {
	for id, ch := range ls.subs {
		select {
		case ch <- ev:
		default:
			slog.Warn("termvt: subscriber too slow, disconnecting", "sub", id)
			close(ch)
			delete(ls.subs, id)
		}
	}
}

// readerLoop pumps pty bytes onto chunkCh. Closing chunkCh signals "pty is
// done" to mainLoop, which then runs handleExit. The benign error classes
// (EOF, ErrClosed, EIO from a child closing the slave) are swallowed; only
// genuine I/O faults are logged.
func (s *Session) readerLoop() {
	defer close(s.chunkCh)
	buf := make([]byte, readChunk)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.chunkCh <- chunk
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				slog.Warn("termvt: pty read error", "err", err)
			}
			return
		}
	}
}

// responseLoop drains the VT emulator's reply pipe back into the pty as if
// the bytes had been typed at the keyboard. The emulator answers DEC/ANSI
// queries (DECRQM "Report Mode", DSR, cursor position, …) by writing to an
// internal io.Pipe; the pipe is unbuffered, so an unread reply blocks
// em.Write inside mainLoop and freezes the entire actor. The runtime's
// dispatch goroutine polls ExitCode on every tick — a single blocking
// query is enough to freeze the whole daemon's IPC. This goroutine is the
// reader the pipe needs.
//
// io.Copy exits when Close → em.Close makes em.Read return io.EOF, or when
// pty.Close makes the write fail; both are normal shutdown paths and
// surface as benign error classes.
func (s *Session) responseLoop() {
	// io.Copy already treats io.EOF as a clean end (returns nil), so only
	// real I/O classes warrant a log.
	if _, err := io.Copy(s.pty, s.em); err != nil &&
		!errors.Is(err, io.ErrClosedPipe) && !errors.Is(err, os.ErrClosed) {
		slog.Warn("termvt: response drain error", "err", err)
	}
}

// registerOSC wires the server-side OSC "tee": semantic sequences are
// captured here and surfaced as Control events instead of being left in the
// raw stream. Handlers run synchronously inside em.Write (called only from
// mainLoop) and append to ls.pending, which mainLoop drains during the same
// processChunk turn.
func (s *Session) registerOSC(ls *loopState) {
	s.em.SetCallbacks(vt.Callbacks{
		Title: func(t string) { ls.pending = append(ls.pending, Control{Kind: "title", Data: t}) },
		Bell:  func() { ls.pending = append(ls.pending, Control{Kind: "bell"}) },
	})
	// OSC 9: desktop notification — captured so it reaches the operating
	// client rather than firing on the server host.
	s.em.RegisterOscHandler(9, func(data []byte) bool {
		ls.pending = append(ls.pending, Control{Kind: "osc", Code: 9, Data: oscText(data)})
		return true
	})
	// OSC 133: shell prompt / command markers — drives run-state detection.
	s.em.RegisterOscHandler(133, func(data []byte) bool {
		ls.pending = append(ls.pending, Control{Kind: "prompt", Code: 133, Data: oscText(data)})
		return true
	})
}
