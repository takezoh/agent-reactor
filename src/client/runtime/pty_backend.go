package runtime

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/termvt"
)

// PtyBackend implements the TmuxBackend role interfaces over platform/termvt,
// driving pty-backed sessions without tmux (ADR 0004). The data plane
// (lifecycle, IO, inspection, liveness) is implemented for real; the
// presentation plane (WindowLayout layout ops, TmuxControl) is stubbed because
// a pty multiplexer has no server-side equivalent — layout composition moves
// client-side in the tmux-removal phase.
//
// Targets are synthetic pane ids ("%1", "%2", …) that PtyBackend allocates and
// maps to live termvt sessions. The unchanged runtime/reducer/driver address
// panes by these ids exactly as they addressed tmux pane ids.
type PtyBackend struct {
	mgr *termvt.Manager

	mu       sync.Mutex
	panes    map[string]*termvt.Session // target (paneID) → session
	buffers  map[string]string          // named tmux-style paste buffers
	env      map[string]string          // session-level env (tmux session env stand-in)
	paneSeq  int                        // last allocated pane number
	winSeq   int                        // last allocated window index
}

// NewPtyBackend returns a PtyBackend with its own termvt.Manager.
func NewPtyBackend() *PtyBackend {
	return &PtyBackend{
		mgr:     termvt.NewManager(),
		panes:   map[string]*termvt.Session{},
		buffers: map[string]string{},
		env:     map[string]string{},
	}
}

// === PaneLifecycle ===

// SpawnWindow starts command in a new pty and returns synthetic window/pane ids.
// startDir is currently unused: termvt.Spec has no working-directory field.
// TODO(B1): thread startDir once termvt.Spec gains a Dir field.
func (p *PtyBackend) SpawnWindow(name, command, startDir string, env map[string]string) (string, string, error) {
	argv, err := agentlaunch.SplitArgs(command)
	if err != nil {
		return "", "", err
	}
	if len(argv) == 0 {
		return "", "", fmt.Errorf("runtime: empty command for window %q", name)
	}

	p.mu.Lock()
	p.paneSeq++
	p.winSeq++
	paneID := "%" + strconv.Itoa(p.paneSeq)
	winIdx := strconv.Itoa(p.winSeq)
	p.mu.Unlock()

	sess, err := p.mgr.Create(paneID, termvt.Spec{Argv: argv, Env: envSlice(env)})
	if err != nil {
		return "", "", err
	}

	p.mu.Lock()
	p.panes[paneID] = sess
	p.mu.Unlock()
	return winIdx, paneID, nil
}

// KillPaneWindow closes the session for target and forgets it.
func (p *PtyBackend) KillPaneWindow(target string) error {
	p.mu.Lock()
	_, ok := p.panes[target]
	delete(p.panes, target)
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q", target)
	}
	return p.mgr.Remove(target)
}

// RespawnPane closes the dead pane and re-creates a session under the same target.
func (p *PtyBackend) RespawnPane(target, command string) error {
	argv, err := agentlaunch.SplitArgs(command)
	if err != nil {
		return err
	}
	if len(argv) == 0 {
		return fmt.Errorf("runtime: empty respawn command for %q", target)
	}
	// Tear down the old session if present.
	p.mu.Lock()
	_, known := p.panes[target]
	delete(p.panes, target)
	p.mu.Unlock()
	if known {
		_ = p.mgr.Remove(target)
	}

	sess, err := p.mgr.Create(target, termvt.Spec{Argv: argv})
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.panes[target] = sess
	p.mu.Unlock()
	return nil
}

// PaneAlive reports whether the session is still running (not closed, not exited).
func (p *PtyBackend) PaneAlive(target string) (bool, error) {
	sess, ok := p.session(target)
	if !ok {
		return false, nil
	}
	_, exited := sess.ExitCode()
	return !exited, nil
}

// PaneExitStatus reports the exit code once the process has been reaped.
func (p *PtyBackend) PaneExitStatus(target string) (bool, int, error) {
	sess, ok := p.session(target)
	if !ok {
		return false, -1, nil
	}
	code, exited := sess.ExitCode()
	if !exited {
		return false, -1, nil
	}
	return true, code, nil
}

// === PaneIO ===

// SendKeys writes text followed by a carriage return to the pane.
func (p *PtyBackend) SendKeys(target, text string) error {
	return p.write(target, []byte(text+"\r"))
}

// SendEnter writes a single carriage return to the pane.
func (p *PtyBackend) SendEnter(target string) error {
	return p.write(target, []byte("\r"))
}

// SendKey writes a named key (or the literal key when unknown) to the pane.
func (p *PtyBackend) SendKey(target, key string) error {
	return p.write(target, []byte(keyBytes(key)))
}

// LoadBuffer stores text under name in the in-memory buffer map.
func (p *PtyBackend) LoadBuffer(name, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buffers[name] = text
	return nil
}

// PasteBuffer writes a stored buffer to target then deletes it.
func (p *PtyBackend) PasteBuffer(name, target string) error {
	p.mu.Lock()
	text, ok := p.buffers[name]
	if ok {
		delete(p.buffers, name)
	}
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("runtime: unknown buffer %q", name)
	}
	return p.write(target, []byte(text))
}

// PipePane is a no-op: output taps are served by termvt.Subscribe in a separate
// task, not by re-piping pane output through a shell command.
// TODO(B1): wire the output tap via Session.Subscribe in the pty_tap task.
func (p *PtyBackend) PipePane(target, command string) error { return nil }

// === PaneInspect ===

// PaneID echoes the synthetic pane id back when the pane is known.
func (p *PtyBackend) PaneID(target string) (string, error) {
	if _, ok := p.session(target); !ok {
		return "", fmt.Errorf("runtime: unknown pane %q", target)
	}
	return target, nil
}

// PaneSize returns the session's current terminal dimensions.
func (p *PtyBackend) PaneSize(target string) (int, int, error) {
	sess, ok := p.session(target)
	if !ok {
		return 0, 0, fmt.Errorf("runtime: unknown pane %q", target)
	}
	cols, rows := sess.Size()
	return cols, rows, nil
}

// CapturePane returns the trailing nLines of the pane's rendered screen with SGR
// escapes stripped.
func (p *PtyBackend) CapturePane(target string, nLines int) (string, error) {
	sess, ok := p.session(target)
	if !ok {
		return "", fmt.Errorf("runtime: unknown pane %q", target)
	}
	return termvt.CaptureTail(sess, nLines), nil
}

// === SessionEnv ===

// SetEnv writes a session-level env var into the in-process store.
func (p *PtyBackend) SetEnv(key, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.env[key] = value
	return nil
}

// UnsetEnv removes a session-level env var from the in-process store.
func (p *PtyBackend) UnsetEnv(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.env, key)
	return nil
}

// ShowEnvironment returns the session env as KEY=VALUE lines sorted by key.
func (p *PtyBackend) ShowEnvironment() (string, error) {
	p.mu.Lock()
	keys := make([]string, 0, len(p.env))
	for k := range p.env {
		keys = append(keys, k)
	}
	vals := make(map[string]string, len(p.env))
	for k, v := range p.env {
		vals[k] = v
	}
	p.mu.Unlock()

	sort.Strings(keys)
	var b []byte
	for _, k := range keys {
		b = append(b, k...)
		b = append(b, '=')
		b = append(b, vals[k]...)
		b = append(b, '\n')
	}
	return string(b), nil
}

// === WindowLayout ===

// ResizeWindow resizes the session's pty/grid. The other layout ops are stubbed.
func (p *PtyBackend) ResizeWindow(target string, width, height int) error {
	sess, ok := p.session(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q", target)
	}
	return sess.Resize(width, height)
}

// The following WindowLayout ops have no pty equivalent — layout composition
// moves client-side in the tmux-removal phase (ADR 0004).
func (p *PtyBackend) SwapPane(srcPane, dstPane string) error  { return nil }
func (p *PtyBackend) BreakPane(srcPane, dstWindow string) error { return nil }
func (p *PtyBackend) BreakPaneToNewWindow(srcPane, name string) (string, error) {
	return "", nil
}
func (p *PtyBackend) JoinPane(srcPane, dstPane string, before bool, sizePct int) error {
	return nil
}
func (p *PtyBackend) SelectPane(target string) error { return nil }
func (p *PtyBackend) RunChain(ops ...[]string) error { return nil }

// === TmuxControl (all stubbed — no server-side equivalent) ===

func (p *PtyBackend) SetStatusLine(line string) error               { return nil }
func (p *PtyBackend) DetachClient() error                           { return nil }
func (p *PtyBackend) KillSession() error                            { return nil }
func (p *PtyBackend) DisplayPopup(width, height, cmd string) error  { return nil }

// === helpers ===

func (p *PtyBackend) session(target string) (*termvt.Session, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sess, ok := p.panes[target]
	return sess, ok
}

func (p *PtyBackend) write(target string, b []byte) error {
	sess, ok := p.session(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q", target)
	}
	sess.WriteInput(b)
	return nil
}

// envSlice converts a KEY→VALUE map into the KEY=VALUE slice termvt.Spec wants.
// A nil/empty map yields nil so the session inherits os.Environ().
func envSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// keyBytes maps the common named keys the runtime sends to their byte sequence.
// Unknown keys pass through literally.
// TODO(B1): extend coverage to the full tmux key-name table as drivers need it.
func keyBytes(key string) string {
	switch key {
	case "Escape":
		return "\x1b"
	case "Enter":
		return "\r"
	case "Up":
		return "\x1b[A"
	case "Down":
		return "\x1b[B"
	case "Right":
		return "\x1b[C"
	case "Left":
		return "\x1b[D"
	case "Tab":
		return "\t"
	case "BSpace":
		return "\x7f"
	case "Space":
		return " "
	default:
		return key
	}
}
