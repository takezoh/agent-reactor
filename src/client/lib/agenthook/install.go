// Package agenthook registers the reactor server / reactor-bridge as an
// agent's hook handler in that agent's settings.json (~/.claude, ~/.gemini).
//
// Claude Code and Gemini CLI are both hook-driven: the daemon learns
// SessionID / TranscriptPath / status transitions / last-prompt only by
// receiving a synchronous shell-out on every lifecycle event (SessionStart,
// PreToolUse / BeforeTool, UserPromptSubmit, Stop, …). Without those hooks
// the session card title stays at "New Session" forever and status never
// moves.
//
// This is the canonical writer for that registration. It is invoked
// directly from the runtime — once at server startup against the host's
// settings.json for each supported agent, and once inside every
// devcontainer's postCreate against the container's settings.json (via the
// reactor-bridge `claude-setup-hooks` / `gemini-setup-hooks` subcommands).
// Registration is NOT delegated to external Make / shell / dotfiles glue:
// it is a runtime invariant.
//
// Agent neutrality: the JSON shape is identical across Claude and Gemini
// (`hooks: {<Event>: [{hooks: [{type: "command", command: "..."}]}]}`); the
// per-agent differences (event list, stale-marker substring, optional
// in-place rewrite of stale entries) are captured in Spec values. Adding a
// future agent with the same shape needs only a new Spec, not a new
// package.
//
// Historical scripts/setup-{claude,gemini}.sh were deleted in lockstep so
// the JSON-mutation behaviour is never duplicated.
package agenthook

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Spec captures one agent's hook-registration parameters. The same Install
// function consumes any Spec, so adding a future agent CLI that uses the
// same JSON shape needs only a new Spec value — no new code path, no parallel
// edit in cmd/server/coordinator.go or cmd/reactor-bridge/main.go (both
// consume `All` and use these fields directly).
type Spec struct {
	// Name labels the agent in log messages and error wrapping AND is the
	// wire token in the registered hook command ("event <Name>"). The
	// consumer side (client/event.Run dispatch) keys on this exact value —
	// renaming Name without updating the dispatcher silently drops events.
	// Lowercase kebab-case ("claude", "gemini").
	Name string

	// Events is the closed list of lifecycle event names registered under
	// settings.hooks.<Event>. Must stay in sync with the consumer side
	// (src/client/driver/<agent>_event.go) — adding an event upstream
	// without listing it here means the driver silently drops that class.
	Events []string

	// SettingsRel is the agent's settings.json path relative to $HOME
	// (".claude/settings.json", ".gemini/settings.json"). Carrying it on
	// the Spec keeps the (Name, Events, SettingsRel) triple in one place so
	// callers in two binaries (cmd/server, cmd/reactor-bridge) don't drift
	// — the original separation into per-caller `variant` structs duplicated
	// this string in both binaries.
	SettingsRel string

	// SubcmdName is the reactor-bridge subcommand that triggers this Spec's
	// in-container registration. Convention is "<Name>-setup-hooks". Lives
	// on Spec so cmd/server/coordinator.go can derive devcontainer
	// postCreate commands directly from agenthook.All — adding an agent no
	// longer requires editing a parallel string list.
	SubcmdName string

	// StaleMarker, when non-empty, anchors detection of a previous
	// reactor-owned hook so a binary-path change rewrites in place instead
	// of accreting a duplicate entry. Match boundary is space-or-EOL — see
	// hasStaleCmd. Set to " event <Name>" for the standard reactor hooks.
	//
	// Empty StaleMarker disables in-place rewrite — the agent's hook list
	// would then accrete one entry per binary-path change. Prefer setting
	// a marker so worktree / branch switches don't pile up orphans.
	//
	// Known limitations:
	//  - Two daemons sharing $HOME with distinct -data-dir values will
	//    rewrite each other's settings.json on every boot (the marker
	//    matches both daemons' hookCmd). Single-daemon per host is the
	//    supported topology; multi-daemon dev runs need a separate
	//    settings.json (override via reactor-bridge's -settings flag).
	//  - An entry whose inner hooks array holds BOTH the current command
	//    and a stale one — a shape only manual editing produces — is
	//    treated as "current entry" and the inner stale hook persists.
	//    Real-world impact: agent invokes both commands on every event.
	//    Recoverable by hand-trimming the entry.
	StaleMarker string
}

// Claude is the canonical Spec for Claude Code (~/.claude/settings.json).
// Event list mirrors the consumer in src/client/driver/claude_event.go.
var Claude = Spec{
	Name: "claude",
	Events: []string{
		"SessionStart",
		"SessionEnd",
		"PreToolUse",
		"PostToolUse",
		"PostToolUseFailure",
		"Stop",
		"StopFailure",
		"UserPromptSubmit",
		"PreCompact",
		"PostCompact",
		"Notification",
		"SubagentStart",
		"SubagentStop",
		"TaskCreated",
		"TaskCompleted",
	},
	SettingsRel: ".claude/settings.json",
	SubcmdName:  "claude-setup-hooks",
	StaleMarker: " event claude",
}

// Gemini is the canonical Spec for Gemini CLI (~/.gemini/settings.json).
// Gemini's smaller event surface uses Before/After verbs where Claude
// uses Pre/Post — these are upstream wire names, not a translation.
var Gemini = Spec{
	Name: "gemini",
	Events: []string{
		"SessionStart",
		"SessionEnd",
		"BeforeTool",
		"AfterTool",
		"BeforeAgent",
		"AfterAgent",
		"Notification",
		"PreCompress",
	},
	SettingsRel: ".gemini/settings.json",
	SubcmdName:  "gemini-setup-hooks",
	StaleMarker: " event gemini",
}

// All is the canonical list of agent Specs the runtime registers. Adding a
// future agent (e.g. Codex hooks if/when they ship) means appending one
// Spec here — every site that fans out across agents (host boot,
// devcontainer postCreate subcmd list, reactor-bridge subcommand dispatch)
// reads from this single list.
var All = []Spec{Claude, Gemini}

// Install ensures every event in spec.Events has a hook entry pointing at
// hookCmd in settingsPath. Idempotent: an event already carrying hookCmd
// verbatim is skipped; an event carrying a stale reactor command (any
// command containing spec.StaleMarker bounded by space-or-EOL) has that
// entry rewritten in place; otherwise a fresh entry is appended.
//
// settingsPath is created with empty {} if missing (and its parent dir is
// created with 0o755). The previous file is best-effort backed up to
// settingsPath + ".bak" before any write — a failed backup is logged but
// does not block registration. The final write is fatal: a settings.json
// the file system rejects is an environment-level failure the caller must
// see.
//
// Returns the events whose hook list was mutated. An empty result with nil
// error means every event was already correctly registered.
func Install(settingsPath, hookCmd string, spec Spec) ([]string, error) {
	if settingsPath == "" {
		return nil, fmt.Errorf("agenthook[%s]: settingsPath is empty", spec.Name)
	}
	if strings.TrimSpace(hookCmd) == "" {
		return nil, fmt.Errorf("agenthook[%s]: hookCmd is empty", spec.Name)
	}
	if len(spec.Events) == 0 {
		return nil, fmt.Errorf("agenthook[%s]: spec.Events is empty", spec.Name)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return nil, fmt.Errorf("agenthook[%s]: mkdir parent of %s: %w", spec.Name, settingsPath, err)
	}
	lock, err := acquireSettingsLock(settingsPath, spec.Name)
	if err != nil {
		return nil, err
	}
	defer lock.release()
	prev, err := readOrInit(settingsPath, spec.Name)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(prev, &root); err != nil {
		return nil, fmt.Errorf("agenthook[%s]: parse %s: %w", spec.Name, settingsPath, err)
	}
	if root == nil {
		root = map[string]any{}
	}
	hooks := mapField(root, "hooks")
	registered := applyToHooks(hooks, hookCmd, spec)
	if len(registered) == 0 {
		return nil, nil
	}
	root["hooks"] = hooks
	if err := backupAndWrite(settingsPath, prev, root, spec.Name); err != nil {
		return registered, err
	}
	return registered, nil
}

type settingsLock struct {
	f *os.File
}

func acquireSettingsLock(settingsPath, name string) (*settingsLock, error) {
	lockPath := settingsPath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("agenthook[%s]: open lock %s: %w", name, lockPath, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("agenthook[%s]: flock %s: %w", name, lockPath, err)
	}
	return &settingsLock{f: f}, nil
}

func (l *settingsLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	_ = l.f.Close()
	l.f = nil
}

// applyToHooks rewrites the map under settings.hooks so each event in
// spec.Events carries hookCmd. The map is mutated in place; the returned
// slice lists every event whose entry list changed.
//
// Four cases per event, based on what scanList finds in the existing list:
//
//	exact NO,  stale NO   → append a fresh entry
//	exact NO,  stale YES  → rewrite the stale entry in place (binary moved)
//	exact YES, stale NO   → no-op (already registered correctly)
//	exact YES, stale YES  → DELETE the stale entry (orphan from prior boot)
//
// The last branch matters when a user hand-edited an old reactor entry
// back in, or a sloppy migration left one behind: without it, the agent
// would fire BOTH commands on every event, double-delivering state to the
// daemon.
func applyToHooks(hooks map[string]any, hookCmd string, spec Spec) []string {
	var registered []string
	for _, ev := range spec.Events {
		list := listField(hooks, ev)
		exactIdx, staleIdx := scanList(list, hookCmd, spec.StaleMarker)
		switch {
		case exactIdx < 0 && staleIdx < 0:
			list = append(list, newHookEntry(hookCmd))
		case exactIdx < 0 && staleIdx >= 0:
			list[staleIdx] = newHookEntry(hookCmd)
		case exactIdx >= 0 && staleIdx >= 0:
			list = removeIndex(list, staleIdx)
		default:
			// exactIdx >= 0, staleIdx < 0: already registered.
			hooks[ev] = list
			continue
		}
		hooks[ev] = list
		registered = append(registered, ev)
	}
	return registered
}

// scanList walks list ONCE and returns the indices of (the first exact
// match, the first stale entry). -1 means absent. Combining the two
// checks halves the per-event scan cost AND keeps them in lockstep —
// splitting them risks one drifting (e.g. case sensitivity,
// command-field key spelling) without the other. A full scan is required:
// returning early on the first exact match would miss a stale entry that
// sits later in the list, leaving an orphan that double-fires on every
// agent event.
//
// An entry that contains the exact hookCmd is NEVER reported as stale,
// even when its command satisfies hasStaleCmd — the marker check is a
// superset (every reactor-emitted exact match contains the marker by
// construction), so without this guard the caller would treat the current
// entry as an orphan and delete its own registration.
//
// staleMarker == "" disables stale detection: staleIdx is always -1.
func scanList(list []any, hookCmd, staleMarker string) (exactIdx, staleIdx int) {
	exactIdx, staleIdx = -1, -1
	for i, entry := range list {
		var hasExact, hasStaleCandidate bool
		for _, h := range entryHooks(entry) {
			cmd, _ := h["command"].(string)
			if cmd == hookCmd {
				hasExact = true
				continue
			}
			if staleMarker != "" && hasStaleCmd(cmd, staleMarker) {
				hasStaleCandidate = true
			}
		}
		if hasExact && exactIdx == -1 {
			exactIdx = i
		}
		if hasStaleCandidate && !hasExact && staleIdx == -1 {
			staleIdx = i
		}
	}
	return exactIdx, staleIdx
}

// removeIndex returns list with element at i deleted. Used to evict the
// stale orphan when the same event already carries a current hookCmd
// elsewhere in the list.
func removeIndex(list []any, i int) []any {
	return append(list[:i], list[i+1:]...)
}

// newHookEntry returns a fresh entry value `{hooks: [{type: "command",
// command: hookCmd}]}`. Built per-call rather than cloned from a template
// — the map literal is cheaper and clearer than reflective deep-copy, and
// it makes "is the only writer the loop body" trivially true.
func newHookEntry(hookCmd string) map[string]any {
	return map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": hookCmd},
		},
	}
}

// hasStaleCmd anchors marker at a space or EOL inside cmd. The anchor
// catches both `<bin> event claude` (EOL) and `<bin> event claude
// -data-dir /x` (space) shapes while excluding `… event claude-launcher …`
// (other character follows). Returns false if cmd does not contain marker.
func hasStaleCmd(cmd, marker string) bool {
	idx := strings.Index(cmd, marker)
	if idx < 0 {
		return false
	}
	after := cmd[idx+len(marker):]
	return after == "" || after[0] == ' '
}

// entryHooks unpacks the "hooks" field on a single event entry. Returns nil
// for absent or malformed entries; the caller treats nil as "no hooks here"
// and continues to the next entry.
func entryHooks(entry any) []map[string]any {
	m, _ := entry.(map[string]any)
	if m == nil {
		return nil
	}
	rawList, _ := m["hooks"].([]any)
	out := make([]map[string]any, 0, len(rawList))
	for _, x := range rawList {
		if mm, ok := x.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

// BuildHookCmd constructs a hook command string for settings.json:
//
//	<binPath> event <Name> [-data-dir <dataDir>]
//
// binPath and dataDir are shell-quoted because the agent CLI (Claude /
// Gemini) hands the registered `command` string back to a shell — an
// unquoted path containing spaces, `$`, `'`, etc. tokenizes wrong and the
// hook either exec-fails or routes to the wrong file. The deleted
// scripts/setup-claude.sh used `printf %q` for exactly this; this helper
// is the Go equivalent.
//
// dataDir == "" omits the flag.
func BuildHookCmd(binPath, dataDir string, spec Spec) string {
	cmd := shellQuote(binPath) + " event " + spec.Name
	if dataDir != "" {
		cmd += " -data-dir " + shellQuote(dataDir)
	}
	return cmd
}

// shellQuote wraps s in single quotes, escaping embedded single quotes by
// closing-quote + escaped quote + opening-quote ('\”). The result is
// always one shell token under sh / bash. Empty input becomes the literal
// ” (an explicit empty arg), not an empty string — matches `printf %q`.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !needsShellQuote(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			b.WriteString(`'\''`)
			continue
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('\'')
	return b.String()
}

// needsShellQuote reports whether s contains any character that would change
// shell semantics. Conservatively flags anything outside the safe portable
// set (alnum, _, -, /, ., :, +, =, @) — the corner cases of POSIX word
// splitting / glob expansion / parameter expansion are not worth threading.
func needsShellQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '/' || c == '.' ||
			c == ':' || c == '+' || c == '=' || c == '@':
		default:
			return true
		}
	}
	return false
}

// mapField returns root[key] as a map[string]any, creating an empty map if
// the key is absent OR holds a non-map value. The replace-on-non-map branch
// is defensive: a settings.json hand-edited to "hooks": null would otherwise
// panic on assignment downstream.
func mapField(root map[string]any, key string) map[string]any {
	if existing, ok := root[key].(map[string]any); ok {
		return existing
	}
	out := map[string]any{}
	root[key] = out
	return out
}

// listField returns m[key] as []any, or nil when absent / non-list. nil and
// empty are interchangeable for the caller — append() handles both.
func listField(m map[string]any, key string) []any {
	existing, _ := m[key].([]any)
	return existing
}

// readOrInit reads settingsPath. If the file is missing, `{}` bytes are
// returned so the caller can proceed against a fresh empty document. Any
// other read failure is fatal.
func readOrInit(settingsPath, name string) ([]byte, error) {
	raw, err := os.ReadFile(settingsPath)
	if err == nil {
		return raw, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("agenthook[%s]: read %s: %w", name, settingsPath, err)
	}
	return []byte("{}"), nil
}

// backupAndWrite writes prevRaw to settingsPath+".bak" (best-effort, logged
// on failure but does not abort), then atomically replaces settingsPath
// with the encoded root. The settings write itself is fatal because losing
// the rewrite means the next session starts without hooks.
func backupAndWrite(settingsPath string, prevRaw []byte, root map[string]any, name string) error {
	if err := os.WriteFile(settingsPath+".bak", prevRaw, 0o644); err != nil {
		slog.Warn("agenthook: backup write failed",
			"agent", name, "path", settingsPath+".bak", "err", err)
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("agenthook[%s]: encode: %w", name, err)
	}
	out = append(out, '\n')
	dir := filepath.Dir(settingsPath)
	tmp, err := os.CreateTemp(dir, filepath.Base(settingsPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("agenthook[%s]: create temp for %s: %w", name, settingsPath, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("agenthook[%s]: chmod temp %s: %w", name, tmpPath, err)
	}
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("agenthook[%s]: write temp %s: %w", name, tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("agenthook[%s]: close temp %s: %w", name, tmpPath, err)
	}
	if err := os.Rename(tmpPath, settingsPath); err != nil {
		return fmt.Errorf("agenthook[%s]: rename temp %s to %s: %w", name, tmpPath, settingsPath, err)
	}
	cleanup = false
	return nil
}
