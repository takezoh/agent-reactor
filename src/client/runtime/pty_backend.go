package runtime

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

// PtyBackend implements the PaneBackend role interfaces over platform/termvt,
// driving pty-backed sessions directly (ADR 0004). The data plane
// (lifecycle, IO, inspection, liveness) is implemented for real; the
// presentation plane (WindowLayout layout ops, BackendControl) is stubbed
// because a pty multiplexer has no server-side equivalent — layout
// composition moves client-side in the backend-replacement phase.
//
// Targets are synthetic pane ids ("%1", "%2", …) that PtyBackend allocates and
// uses as the termvt.Manager session id, so the live session is always resolved
// via mgr.Get(target) — the Manager is the single owner of the id→Session map.
// The unchanged runtime/reducer/driver address panes by these ids exactly as
// they addressed pane ids (e.g. "%1", "%2").
//
// Targets passed in from the runtime are normalised by resolvePaneTarget before
// the Manager is consulted: a "sessionName:windowIndex" form (e.g. "arc:1",
// emitted by interpret_spawn.ResizeWindow) is stripped to its windowIndex and
// then looked up in the windowIndex→paneID map populated at spawn.
type PtyBackend struct {
	mgr *termvt.Manager

	mu      sync.Mutex
	buffers map[string]string // named paste buffers
	env     map[string]string // session-level env
	windows map[string]string // windowIndex -> paneID (filled by SpawnWindow)
	paneSeq int               // last allocated pane number
	winSeq  int               // last allocated window index

	// scrollbackLines is the per-session VT scrollback cap stamped into
	// every termvt.Spec built by SpawnWindow / RespawnPane. Zero leaves
	// the underlying xvt emulator default (10000) in effect — tests pass
	// 0 to avoid coupling to the cap, production passes the value from
	// settings.toml's [terminal] scrollback_lines.
	scrollbackLines int
}

// NewPtyBackend returns a PtyBackend with its own termvt.Manager.
// scrollbackLines is the per-session VT scrollback cap; 0 keeps the
// underlying emulator's default.
func NewPtyBackend(scrollbackLines int) *PtyBackend {
	return &PtyBackend{
		mgr:             termvt.NewManager(),
		buffers:         map[string]string{},
		env:             map[string]string{},
		windows:         map[string]string{},
		scrollbackLines: scrollbackLines,
	}
}

// === PaneLifecycle ===

// SpawnWindow starts command in a new pty and returns synthetic window/pane
// ids. The command is always invoked via the user's POSIX shell so that the
// shell strings the runtime emits (login-shell exec, stdin-wrapped bash -c,
// driver-launch lines — see interpret_spawn.buildSpawnCommand) keep their
// shell semantics; PtyBackend stays a thin shell host. startDir is currently
// unused: termvt.Spec has no working-directory field.
// TODO(B1): thread startDir once termvt.Spec gains a Dir field.
func (p *PtyBackend) SpawnWindow(name, command, startDir string, env map[string]string) (string, string, error) {
	if strings.TrimSpace(command) == "" {
		return "", "", fmt.Errorf("runtime: empty command for window %q", name)
	}

	// mu protects only the id counters and the windows map; the ids it yields
	// are unique, so release it before Create rather than holding it across the
	// fork/exec in pty.StartWithSize (which would serialise every other backend
	// op behind a spawn). The Manager has its own mutex and rejects duplicate ids.
	p.mu.Lock()
	p.paneSeq++
	p.winSeq++
	paneID := newPaneID(p.paneSeq)
	winIdx := newWindowIndex(p.winSeq)
	p.windows[winIdx] = paneID
	p.mu.Unlock()

	if _, err := p.mgr.Create(paneID, termvt.Spec{
		Argv:            shellArgv(command),
		Env:             envSlice(env),
		ScrollbackLines: p.scrollbackLines,
	}); err != nil {
		p.mu.Lock()
		delete(p.windows, winIdx)
		p.mu.Unlock()
		return "", "", err
	}
	return winIdx, paneID, nil
}

// KillPaneWindow closes the session for target and forgets it.
//
// forgetWindowFor runs unconditionally so a stale windowIndex→paneID entry is
// cleaned up even when the Manager already lost the session (e.g. it was
// reaped concurrently). When the Manager reports the session missing, the
// error is wrapped with ErrPaneMissing so isMissingPaneErr can recognise it
// alongside SendKeys/CapturePane/ResizeWindow et al. The error message keeps
// the caller's original target shape — runtime debugging stays readable when
// the call site passed "arc:1" rather than the resolved "%1".
func (p *PtyBackend) KillPaneWindow(target string) error {
	resolved := p.resolvePaneTarget(target)
	err := p.mgr.Remove(resolved)
	p.forgetWindowFor(resolved)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	return err
}

// RespawnPane tears the dead pane down and re-creates a session under the same
// target. It does NOT carry over the session-env store or the original spawn
// env — respawn launches a fresh process with the default environment — and the
// new session starts at the default terminal size until the next ResizeWindow.
//
// The Manager owns the id→Session map and its own mutex, so we never hold
// p.mu across mgr.Remove / mgr.Create — that would serialise every other
// backend op (SendKeys, CapturePane, etc.) behind the respawn's fork/exec, in
// violation of the lock-discipline SpawnWindow's comment already documents.
func (p *PtyBackend) RespawnPane(target, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("runtime: empty respawn command for %q", target)
	}
	target = p.resolvePaneTarget(target)

	// Tear down the old session if present. If teardown fails, abort: do not
	// stack a new session on top of a pane we could not cleanly remove.
	if _, known := p.mgr.Get(target); known {
		if err := p.mgr.Remove(target); err != nil {
			return fmt.Errorf("runtime: respawn %q: %w", target, err)
		}
	}

	if _, err := p.mgr.Create(target, termvt.Spec{
		Argv:            shellArgv(command),
		ScrollbackLines: p.scrollbackLines,
	}); err != nil {
		// Manager.Remove already dropped the session; with Create failing too,
		// any windows-map entry that pointed at this paneID is now stale —
		// drop it so a stale windowIndex never resolves to a dead pane id.
		p.forgetWindowFor(target)
		return err
	}
	return nil
}

// PaneExitStatus reports the exit code once the process has been reaped.
func (p *PtyBackend) PaneExitStatus(target string) (bool, int, error) {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return false, -1, fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
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

// PipePane is a no-op: output taps are served by PtyPaneTap (see pty_tap.go),
// which subscribes directly to the termvt.Session and bypasses the legacy
// pipe-pane bridge. Tap teardown is driven by tap_manager.stop, which cancels
// its own per-frame tapCtx (propagating to the forwarder via the context
// chain) and then calls PtyPaneTap.Stop to cancel the inner sub-ctx; whichever
// fires first triggers the forwarder's ctx.Done branch and Session.Unsubscribe.
// The empty-command "stop the tap" contract collapses to a no-op here without
// losing functionality.
func (p *PtyBackend) PipePane(target, command string) error { return nil }

// === PaneInspect ===

// PaneID echoes the synthetic pane id back when the pane is known.
func (p *PtyBackend) PaneID(target string) (string, error) {
	target = p.resolvePaneTarget(target)
	if _, ok := p.mgr.Get(target); !ok {
		return "", fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	return target, nil
}

// PaneSize returns the session's current terminal dimensions.
func (p *PtyBackend) PaneSize(target string) (int, int, error) {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return 0, 0, fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	cols, rows := sess.Size()
	return cols, rows, nil
}

// CapturePane returns the trailing nLines of the pane's rendered screen with SGR
// escapes stripped.
func (p *PtyBackend) CapturePane(target string, nLines int) (string, error) {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return "", fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	return termvt.CaptureTail(sess, nLines), nil
}

// === SessionEnv ===
//
// The session-env store is in-process only: it lives in p.env and dies with the
// process. It is NOT a persistence layer — values do not survive a daemon
// restart and are not injected into spawned children. Cross-restart pane
// recovery is out of scope for B1 (ADR 0004) and belongs to a later phase.

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
	pairs := make([][2]string, 0, len(p.env))
	for k, v := range p.env {
		pairs = append(pairs, [2]string{k, v})
	}
	p.mu.Unlock()

	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	var b []byte
	for _, kv := range pairs {
		b = append(b, kv[0]...)
		b = append(b, '=')
		b = append(b, kv[1]...)
		b = append(b, '\n')
	}
	return string(b), nil
}

// === WindowLayout ===

// ResizeWindow resizes the session's pty/grid. The other layout ops are stubbed.
//
// target accepts the windowIndex form ("1") produced by SpawnWindow, the
// sessionName:windowIndex form ("arc:1") that interpret_spawn emits, and the
// raw pane id form ("%1"). resolvePaneTarget normalises any of them to the
// pane id the Manager indexes on.
func (p *PtyBackend) ResizeWindow(target string, width, height int) error {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	return sess.Resize(width, height)
}

// The following WindowLayout ops have no pty equivalent — layout composition
// moves client-side in the backend-replacement phase (ADR 0004).
func (p *PtyBackend) SwapPane(srcPane, dstPane string) error    { return nil }
func (p *PtyBackend) BreakPane(srcPane, dstWindow string) error { return nil }
func (p *PtyBackend) BreakPaneToNewWindow(srcPane, name string) (string, error) {
	return "", nil
}
func (p *PtyBackend) JoinPane(srcPane, dstPane string, before bool, sizePct int) error {
	return nil
}
func (p *PtyBackend) SelectPane(target string) error { return nil }
func (p *PtyBackend) RunChain(ops ...[]string) error { return nil }

// === Surface accessors (ADR 0009) ===
//
// SubscribeSurface, UnsubscribeSurface, WriteSurface, and ResizeSurface are
// the bridge between the web-facing terminal_relay and the termvt sessions
// managed by PtyBackend. They follow the same resolvePaneTarget + mgr.Get
// pattern as the existing inspect and IO methods so the call site in
// terminal_relay addresses panes by the same synthetic id the runtime uses.

// SubscribeSurface registers a subscriber on the termvt.Session for paneID
// and returns the subscriber id and its event channel. The first event on the
// channel is a reattach snapshot of the current screen (termvt's guarantee).
// The caller is responsible for calling UnsubscribeSurface when done.
func (p *PtyBackend) SubscribeSurface(target string) (int, <-chan termvt.Event, error) {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return 0, nil, fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	id, ch := sess.Subscribe()
	return id, ch, nil
}

// UnsubscribeSurface releases the subscriber id on paneID's session. It is
// safe to call after the session has exited (the channel will already be
// closed); the pane-not-found error is ignored in that case because the
// caller's only goal is teardown.
func (p *PtyBackend) UnsubscribeSurface(target string, id int) error {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	sess.Unsubscribe(id)
	return nil
}

// WriteSurface writes raw bytes directly to paneID's pty. Unlike SendKeys,
// no carriage return is appended; the caller (xterm.js via terminal_relay)
// is expected to have already assembled the correct byte sequence.
func (p *PtyBackend) WriteSurface(target string, data []byte) error {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	return sess.WriteInput(data)
}

// ResizeSurface resizes paneID's pty and VT emulator grid to (cols, rows).
// It delegates directly to sess.Resize so both the pty winsize and the
// emulator grid are updated atomically.
func (p *PtyBackend) ResizeSurface(target string, cols, rows int) error {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	return sess.Resize(cols, rows)
}

// === BackendControl (all stubbed — no server-side equivalent) ===

func (p *PtyBackend) SetStatusLine(line string) error              { return nil }
func (p *PtyBackend) DetachClient() error                          { return nil }
func (p *PtyBackend) KillSession() error                           { return nil }
func (p *PtyBackend) DisplayPopup(width, height, cmd string) error { return nil }

// === helpers ===

func (p *PtyBackend) write(target string, b []byte) error {
	target = p.resolvePaneTarget(target)
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)
	}
	return sess.WriteInput(b)
}

// resolvePaneTarget normalises target into the pane id the Manager indexes on.
// It accepts three shapes the runtime emits today:
//   - paneID ("%1") — used as-is (newPaneID always emits "%<digits>", so the
//     numeric suffix check protects against a malformed target sneaking through
//     the windows-map lookup as a bogus "windowIndex").
//   - windowIndex ("1") — translated via the windows map populated at spawn.
//   - sessionName:windowIndex ("arc:1", emitted by interpret_spawn after the
//     SpawnWindow→ResizeWindow handoff) — the prefix is stripped, then the
//     windowIndex path runs.
//
// If no translation matches (e.g. the runtime is probing a pane id we never
// minted) the original target falls through so the caller's mgr.Get reports
// ErrPaneMissing with the same target the runtime asked about.
func (p *PtyBackend) resolvePaneTarget(target string) string {
	if target == "" {
		return target
	}
	if isPaneIDForm(target) {
		return target
	}
	if i := strings.LastIndex(target, ":"); i >= 0 {
		target = target[i+1:]
	}
	p.mu.Lock()
	paneID, ok := p.windows[target]
	p.mu.Unlock()
	if ok {
		return paneID
	}
	return target
}

// isPaneIDForm reports whether target is in PtyBackend's own "%<digits>" pane
// id form (the shape newPaneID emits). Plain "%" or "%abc" do not qualify.
func isPaneIDForm(target string) bool {
	if len(target) < 2 || target[0] != '%' {
		return false
	}
	for i := 1; i < len(target); i++ {
		if target[i] < '0' || target[i] > '9' {
			return false
		}
	}
	return true
}

// forgetWindowFor removes the windowIndex→paneID entry for paneID. Called on
// teardown so that a window index does not survive the pane that backed it.
// Each paneID appears at most once as a value (SpawnWindow allocates fresh,
// unique paneIDs; KillPaneWindow drops the entry; RespawnPane reuses the
// same paneID without adding a new windows entry), so we stop after the
// first hit — both a slight speed-up and a clearer expression of the
// invariant.
func (p *PtyBackend) forgetWindowFor(paneID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for idx, id := range p.windows {
		if id == paneID {
			delete(p.windows, idx)
			return
		}
	}
}

// shellArgv wraps a command string in the POSIX shell so the runtime's emitted
// shell strings keep their shell semantics (exec prefixes, bash -c stdin
// wrappers, driver-launch lines) — PtyBackend stays a thin shell host and
// defers command shape to interpret_spawn.buildSpawnCommand.
func shellArgv(command string) []string {
	return []string{"/bin/sh", "-c", command}
}

// newPaneID formats a synthetic pane id ("%1", "%2", …).
func newPaneID(n int) string { return "%" + strconv.Itoa(n) }

// newWindowIndex formats a synthetic window index ("1", "2", …).
func newWindowIndex(n int) string { return strconv.Itoa(n) }

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

// keyBytes maps the named keys the runtime sends to their byte sequence.
// Control chords ("C-c") and meta chords ("M-x") are decoded generically;
// remaining unknown keys pass through literally.
// TODO(B1): extend coverage to the full key-name table as drivers need it.
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
	}
	if b, ok := chordBytes(key); ok {
		return b
	}
	return key
}

// chordBytes decodes a single-character control or meta chord. "C-<ch>" maps to
// the control byte (ch & 0x1f), so "C-c" → 0x03 (SIGINT). "M-<ch>" maps to ESC
// followed by ch. It reports ok=false for anything that is not a recognised
// single-character chord so the caller can fall back to literal passthrough.
func chordBytes(key string) (string, bool) {
	if len(key) != 3 || key[1] != '-' {
		return "", false
	}
	ch := key[2]
	switch key[0] {
	case 'C':
		return string([]byte{ch & 0x1f}), true
	case 'M':
		return string([]byte{0x1b, ch}), true
	default:
		return "", false
	}
}
