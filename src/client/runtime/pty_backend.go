package runtime

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

// PtyBackend implements the FrameBackend role interfaces over platform/termvt,
// driving pty-backed sessions directly (ADR 0004). The data plane
// (lifecycle, IO, inspection, liveness) is implemented for real; the
// presentation plane (WindowLayout layout ops, BackendControl) is stubbed
// because a pty multiplexer has no server-side equivalent — layout
// composition moves client-side in the backend-replacement phase.
//
// Targets are FrameID strings: termvt.Manager keys every session on
// string(FrameID), so the live session is always resolved via mgr.Get(target).
// The runtime and reducers therefore address frames by the same id end-to-end.
type PtyBackend struct {
	mgr *termvt.Manager

	mu      sync.Mutex
	buffers map[string]string // named paste buffers
	env     map[string]string // session-level env

	// scrollbackLines is the per-session VT scrollback cap stamped into
	// every termvt.Spec built by SpawnFrame / RespawnFrame. Zero leaves
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
		scrollbackLines: scrollbackLines,
	}
}

// === FrameLifecycle ===

// SpawnFrame starts command in a new pty under frameID. The command is always
// invoked via the user's POSIX shell so that the shell strings the runtime
// emits (login-shell exec, stdin-wrapped bash -c, driver-launch lines — see
// interpret_spawn.buildSpawnCommand) keep their shell semantics; PtyBackend
// stays a thin shell host. startDir is currently unused: termvt.Spec has no
// working-directory field.
// TODO(B1): thread startDir once termvt.Spec gains a Dir field.
func (p *PtyBackend) SpawnFrame(frameID, name, command, startDir string, env map[string]string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("runtime: empty command for frame %q", name)
	}
	if _, err := p.mgr.Create(frameID, termvt.Spec{
		Argv:            shellArgv(command),
		Env:             envSlice(env),
		ScrollbackLines: p.scrollbackLines,
	}); err != nil {
		return err
	}
	return nil
}

// KillFrame closes the session for target. When the Manager reports the
// session missing, the error is wrapped with ErrFrameMissing so
// isMissingFrameErr can recognise it alongside SendKeys/CaptureFrame/
// ResizeWindow et al.
func (p *PtyBackend) KillFrame(target string) error {
	err := p.mgr.Remove(target)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	return err
}

// RespawnFrame tears the dead session down and re-creates a session under the
// same target. It does NOT carry over the session-env store or the original
// spawn env — respawn launches a fresh process with the default environment —
// and the new session starts at the default terminal size until the next
// ResizeWindow.
//
// The Manager owns the id→Session map and its own mutex; we never hold p.mu
// across mgr.Remove / mgr.Create.
func (p *PtyBackend) RespawnFrame(target, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("runtime: empty respawn command for %q", target)
	}

	// Tear down the old session if present. If teardown fails, abort: do not
	// stack a new session on top of a frame we could not cleanly remove.
	if _, known := p.mgr.Get(target); known {
		if err := p.mgr.Remove(target); err != nil {
			return fmt.Errorf("runtime: respawn %q: %w", target, err)
		}
	}

	if _, err := p.mgr.Create(target, termvt.Spec{
		Argv:            shellArgv(command),
		ScrollbackLines: p.scrollbackLines,
	}); err != nil {
		return err
	}
	return nil
}

// FrameExitStatus reports the exit code once the process has been reaped.
func (p *PtyBackend) FrameExitStatus(target string) (bool, int, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return false, -1, fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	code, exited := sess.ExitCode()
	if !exited {
		return false, -1, nil
	}
	return true, code, nil
}

// === FrameIO ===

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

// PipeFrame is a no-op: output taps are served by PtyFrameTap (see pty_tap.go),
// which subscribes directly to the termvt.Session and bypasses the legacy
// pipe-pane bridge. Tap teardown is driven by tap_manager.stop, which cancels
// its own per-frame tapCtx (propagating to the forwarder via the context
// chain) and then calls PtyFrameTap.Stop to cancel the inner sub-ctx; whichever
// fires first triggers the forwarder's ctx.Done branch and Session.Unsubscribe.
// The empty-command "stop the tap" contract collapses to a no-op here without
// losing functionality.
func (p *PtyBackend) PipeFrame(target, command string) error { return nil }

// === FrameInspect ===

// ResolveID echoes target back when the frame is known.
func (p *PtyBackend) ResolveID(target string) (string, error) {
	if _, ok := p.mgr.Get(target); !ok {
		return "", fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	return target, nil
}

// FrameSize returns the session's current terminal dimensions.
func (p *PtyBackend) FrameSize(target string) (int, int, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return 0, 0, fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	cols, rows := sess.Size()
	return cols, rows, nil
}

// CaptureFrame returns the trailing nLines of the pane's rendered screen with SGR
// escapes stripped.
func (p *PtyBackend) CaptureFrame(target string, nLines int) (string, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return "", fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
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
func (p *PtyBackend) ResizeWindow(target string, width, height int) error {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
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
// managed by PtyBackend. They look the frame up via mgr.Get directly — the
// caller addresses panes by the same FrameID string the runtime uses.

// SubscribeSurface registers a subscriber on the termvt.Session for target
// and returns the subscriber id and its event channel. The first event on the
// channel is a reattach snapshot of the current screen (termvt's guarantee).
// The caller is responsible for calling UnsubscribeSurface when done.
func (p *PtyBackend) SubscribeSurface(target string) (int, <-chan termvt.Event, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return 0, nil, fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	id, ch := sess.Subscribe()
	return id, ch, nil
}

// UnsubscribeSurface releases the subscriber id on target's session. It is
// safe to call after the session has exited (the channel will already be
// closed); the missing-frame error is ignored in that case because the
// caller's only goal is teardown.
func (p *PtyBackend) UnsubscribeSurface(target string, id int) error {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	sess.Unsubscribe(id)
	return nil
}

// WriteSurface writes raw bytes directly to target's pty. Unlike SendKeys,
// no carriage return is appended; the caller (xterm.js via terminal_relay)
// is expected to have already assembled the correct byte sequence.
func (p *PtyBackend) WriteSurface(target string, data []byte) error {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	return sess.WriteInput(data)
}

// ResizeSurface resizes target's pty and VT emulator grid to (cols, rows).
// It delegates directly to sess.Resize so both the pty winsize and the
// emulator grid are updated atomically.
func (p *PtyBackend) ResizeSurface(target string, cols, rows int) error {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
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
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown frame %q: %w", target, ErrFrameMissing)
	}
	return sess.WriteInput(b)
}

// shellArgv wraps a command string in the POSIX shell so the runtime's emitted
// shell strings keep their shell semantics (exec prefixes, bash -c stdin
// wrappers, driver-launch lines) — PtyBackend stays a thin shell host and
// defers command shape to interpret_spawn.buildSpawnCommand.
func shellArgv(command string) []string {
	return []string{"/bin/sh", "-c", command}
}

// envSlice returns the KEY=VALUE slice termvt.Spec.Env wants for the spawned
// pty child. The daemon's process env (os.Environ — populated from the
// user-scope systemd unit's EnvironmentFile, or the invoking user's env when
// run directly) is ALWAYS the base; entries in overrides shadow individual
// keys in place. This preserves Unix fork-exec inheritance so daemon children
// see the same PATH / HOME / mise+cargo shims the daemon itself runs with.
//
// Host-direct launches (SandboxOverrideHost → DirectDispatcher) carry only
// {ROOST_SESSION_ID, ROOST_FRAME_ID} as overrides; without inheritance they
// lose PATH and fail `exec claude` (~/.local/bin) with exit 127 (a5ec8f11
// incident, 2026-06-27). Devcontainer launches are unaffected: their
// in-container env is carried as `docker exec -e KEY=VAL …` arguments,
// separate from this slice.
//
// Merge details live in mergeEnv.
func envSlice(overrides map[string]string) []string {
	return mergeEnv(os.Environ(), overrides)
}

// mergeEnv overlays overrides onto base and returns the merged KEY=VALUE slice.
// Contract:
//   - base order is preserved; for duplicate keys in base, only the first
//     occurrence is emitted (matches glibc getenv). Duplicates arise after
//     sudo / layered EnvironmentFiles and are legal under POSIX execve.
//   - a base entry whose key appears in overrides is rewritten in place to
//     KEY=<override-value>; no key is emitted more than once.
//   - malformed base entries with no '=' are passed through verbatim and do
//     NOT participate in the keyed dedup — a bare "FOO" must not shadow a
//     well-formed "FOO=…" or block an overlay for FOO.
//   - overrides whose key is absent from base are appended in sorted order
//     so the result is deterministic between runs (Spec.Env feeds memoization
//     / golden tests downstream).
//   - len(overrides)==0 returns base verbatim.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	seenKey := make(map[string]struct{}, len(base)+len(overrides))
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			out = append(out, kv) // malformed, preserve, don't track as key
			continue
		}
		key := kv[:eq]
		if _, dup := seenKey[key]; dup {
			continue
		}
		seenKey[key] = struct{}{}
		if v, ok := overrides[key]; ok {
			out = append(out, key+"="+v)
		} else {
			out = append(out, kv)
		}
	}
	extra := make([]string, 0, len(overrides))
	for k := range overrides {
		if _, dup := seenKey[k]; dup {
			continue
		}
		extra = append(extra, k)
	}
	sort.Strings(extra)
	for _, k := range extra {
		out = append(out, k+"="+overrides[k])
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
