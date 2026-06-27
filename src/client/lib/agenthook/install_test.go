package agenthook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: read settings.json back as map[string]any for assertions.
func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return m
}

// helper: extract the list of commands registered for a given event.
func commandsFor(t *testing.T, settings map[string]any, event string) []string {
	t.Helper()
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}
	entries, _ := hooks[event].([]any)
	var out []string
	for _, e := range entries {
		em, _ := e.(map[string]any)
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm, _ := h.(map[string]any)
			if cmd, ok := hm["command"].(string); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}

func TestInstall_CreatesFileWhenMissing_Claude(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "settings.json")

	registered, err := Install(path, "/usr/bin/server event claude", Claude)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(registered) != len(Claude.Events) {
		t.Errorf("registered = %d events, want %d (all)", len(registered), len(Claude.Events))
	}

	settings := readSettings(t, path)
	for _, ev := range Claude.Events {
		cmds := commandsFor(t, settings, ev)
		if len(cmds) != 1 || cmds[0] != "/usr/bin/server event claude" {
			t.Errorf("event %s: cmds = %v, want one hook", ev, cmds)
		}
	}
}

func TestInstall_CreatesFileWhenMissing_Gemini(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "settings.json")

	registered, err := Install(path, "/usr/bin/server event gemini", Gemini)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(registered) != len(Gemini.Events) {
		t.Errorf("registered = %d events, want %d (all)", len(registered), len(Gemini.Events))
	}

	settings := readSettings(t, path)
	// Gemini exposes BeforeTool/AfterTool/BeforeAgent/AfterAgent — not
	// Claude's PreToolUse/PostToolUse names. Cross-check we used the right
	// vocabulary.
	for _, ev := range []string{"BeforeTool", "AfterTool", "PreCompress"} {
		cmds := commandsFor(t, settings, ev)
		if len(cmds) != 1 || cmds[0] != "/usr/bin/server event gemini" {
			t.Errorf("event %s: cmds = %v, want one hook", ev, cmds)
		}
	}
	// Claude-only event should not leak into Gemini's settings.
	cmds := commandsFor(t, settings, "PreToolUse")
	if len(cmds) != 0 {
		t.Errorf("Claude-only PreToolUse appeared in Gemini settings: %v", cmds)
	}
}

func TestInstall_IdempotentSecondRunIsNoOp(t *testing.T) {
	for _, spec := range []Spec{Claude, Gemini} {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		cmd := "/usr/bin/server event " + spec.Name

		if _, err := Install(path, cmd, spec); err != nil {
			t.Fatalf("[%s] first Install: %v", spec.Name, err)
		}
		registered, err := Install(path, cmd, spec)
		if err != nil {
			t.Fatalf("[%s] second Install: %v", spec.Name, err)
		}
		if len(registered) != 0 {
			t.Errorf("[%s] second run registered = %v, want empty", spec.Name, registered)
		}
	}
}

func TestInstall_StaleEntryRewrittenInPlace_Claude(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Seed with a stale reactor entry under SessionStart that points at the
	// old binary path. Install must overwrite it in place rather than
	// appending a duplicate.
	seed := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "/old/path/server event claude"},
					},
				},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	newCmd := "/new/path/server event claude -data-dir /var/lib/reactor"
	if _, err := Install(path, newCmd, Claude); err != nil {
		t.Fatalf("Install: %v", err)
	}

	settings := readSettings(t, path)
	cmds := commandsFor(t, settings, "SessionStart")
	if len(cmds) != 1 {
		t.Fatalf("SessionStart cmds = %v, want exactly one (stale rewritten in place)", cmds)
	}
	if cmds[0] != newCmd {
		t.Errorf("SessionStart cmd = %q, want %q", cmds[0], newCmd)
	}
}

func TestInstall_StaleMarkerEmpty_NoRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Seed a stale entry, then install with a Spec whose StaleMarker is "".
	// Expected behaviour: the stale entry survives AND a new entry is
	// appended (the append-only fallback for agents whose Spec opts out of
	// rewriting).
	seed := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "/old/path/server event foo"},
					},
				},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	noRewrite := Spec{Name: "foo", Events: []string{"SessionStart"}}
	if _, err := Install(path, "/new/path/server event foo", noRewrite); err != nil {
		t.Fatalf("Install: %v", err)
	}
	cmds := commandsFor(t, readSettings(t, path), "SessionStart")
	if len(cmds) != 2 {
		t.Errorf("SessionStart cmds = %v, want two (stale survives + new appended)", cmds)
	}
}

func TestInstall_PreservesUnrelatedEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Seed with a user-owned PostToolUse hook on Bash. Install must append
	// the reactor entry without touching the existing Bash entry — the
	// user's own audit script keeps running.
	seed := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "~/.claude/hooks/bash-audit.sh"},
					},
				},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	if _, err := Install(path, "/usr/bin/server event claude", Claude); err != nil {
		t.Fatalf("Install: %v", err)
	}
	settings := readSettings(t, path)
	cmds := commandsFor(t, settings, "PostToolUse")
	wantBash := "~/.claude/hooks/bash-audit.sh"
	wantReactor := "/usr/bin/server event claude"
	hasBash, hasReactor := false, false
	for _, c := range cmds {
		if c == wantBash {
			hasBash = true
		}
		if c == wantReactor {
			hasReactor = true
		}
	}
	if !hasBash {
		t.Errorf("Bash audit entry was dropped: cmds=%v", cmds)
	}
	if !hasReactor {
		t.Errorf("reactor entry not registered: cmds=%v", cmds)
	}
}

func TestInstall_PreservesUnrelatedRootFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	seed := map[string]any{
		"awsAuthRefresh": "aws sso login",
		"permissions": map[string]any{
			"allow": []any{"Bash(git *)"},
		},
		"env": map[string]any{"FOO": "bar"},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	if _, err := Install(path, "/usr/bin/server event claude", Claude); err != nil {
		t.Fatalf("Install: %v", err)
	}
	settings := readSettings(t, path)
	if settings["awsAuthRefresh"] != "aws sso login" {
		t.Errorf("awsAuthRefresh changed: %v", settings["awsAuthRefresh"])
	}
	if _, ok := settings["permissions"].(map[string]any); !ok {
		t.Errorf("permissions block lost: %v", settings["permissions"])
	}
	if _, ok := settings["env"].(map[string]any); !ok {
		t.Errorf("env block lost: %v", settings["env"])
	}
}

func TestInstall_WritesBackupBeforeOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	original := []byte(`{"hooks":{}}` + "\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	if _, err := Install(path, "/usr/bin/server event claude", Claude); err != nil {
		t.Fatalf("Install: %v", err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(bak) != string(original) {
		t.Errorf("backup content mismatch:\n  got:  %q\n  want: %q", bak, original)
	}
}

func TestInstall_EmptyArgsRejected(t *testing.T) {
	if _, err := Install("", "/usr/bin/server event claude", Claude); err == nil {
		t.Errorf("empty settingsPath should error")
	}
	if _, err := Install("/tmp/x.json", "", Claude); err == nil {
		t.Errorf("empty hookCmd should error")
	}
	if _, err := Install("/tmp/x.json", "   ", Claude); err == nil {
		t.Errorf("whitespace hookCmd should error")
	}
	if _, err := Install("/tmp/x.json", "/u/server event x", Spec{Name: "x"}); err == nil {
		t.Errorf("empty events should error")
	}
}

func TestHasStaleCmd_Boundary(t *testing.T) {
	cases := []struct {
		cmd, marker string
		want        bool
	}{
		{"/usr/bin/server event claude", " event claude", true},
		{"/usr/bin/server event claude -data-dir /x", " event claude", true},
		{"/usr/bin/server -data-dir /x event claude", " event claude", true},
		// "event claude-launcher" should NOT trigger — boundary is space/EOL.
		{"/usr/bin/server event claude-launcher", " event claude", false},
		{"~/.claude/hooks/bash-audit.sh", " event claude", false},
		{"", " event claude", false},
		// Gemini variant.
		{"/usr/bin/server event gemini", " event gemini", true},
		{"/usr/bin/server event gemini -data-dir /x", " event gemini", true},
	}
	for _, c := range cases {
		if got := hasStaleCmd(c.cmd, c.marker); got != c.want {
			t.Errorf("hasStaleCmd(%q, %q) = %v, want %v", c.cmd, c.marker, got, c.want)
		}
	}
}

func TestInstall_HookCmdRoundtripsThroughJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Path with spaces — JSON must encode it as a single string field, not
	// drop the whitespace.
	cmd := `/Users/Alice With Spaces/server event claude -data-dir "/tmp/with spaces"`
	if _, err := Install(path, cmd, Claude); err != nil {
		t.Fatalf("Install: %v", err)
	}
	settings := readSettings(t, path)
	cmds := commandsFor(t, settings, "SessionStart")
	if len(cmds) != 1 || cmds[0] != cmd {
		t.Errorf("roundtripped cmd mismatch:\n  got:  %q\n  want: %q", cmds[0], cmd)
	}
}

func TestInstall_MalformedHooksFieldIsRepaired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Hand-written `"hooks": null` would otherwise panic on the type assert.
	raw := []byte(`{"hooks": null}` + "\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	if _, err := Install(path, "/usr/bin/server event claude", Claude); err != nil {
		t.Fatalf("Install: %v", err)
	}
	settings := readSettings(t, path)
	cmds := commandsFor(t, settings, "SessionStart")
	if len(cmds) != 1 {
		t.Errorf("SessionStart cmds = %v, want one entry after repair", cmds)
	}
}

func TestInstall_RegisteredListMatchesActualChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	cmd := "/usr/bin/server event claude"

	// Seed SessionStart already correct → registered list must exclude it
	// on next run, while every other event is fresh and must be listed.
	seed := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": cmd},
					},
				},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	registered, err := Install(path, cmd, Claude)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, ev := range registered {
		if ev == "SessionStart" {
			t.Errorf("SessionStart listed as registered despite being pre-seeded: %v", registered)
		}
	}
	if want := len(Claude.Events) - 1; len(registered) != want {
		t.Errorf("registered count = %d, want %d", len(registered), want)
	}
	joined := strings.Join(registered, ",")
	for _, ev := range Claude.Events {
		if ev == "SessionStart" {
			continue
		}
		if !strings.Contains(joined, ev) {
			t.Errorf("registered list missing %q: %v", ev, registered)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"plain", "plain"},
		{"/usr/bin/server", "/usr/bin/server"},
		{"a-b_c.d:e+f=g@h/i", "a-b_c.d:e+f=g@h/i"},
		{"with space", "'with space'"},
		{"$VAR", "'$VAR'"},
		{"a;b", "'a;b'"},
		{"don't", `'don'\''t'`},
		{`/Users/Alice With Spaces/bin/server`, `'/Users/Alice With Spaces/bin/server'`},
		{"`whoami`", "'`whoami`'"},
		{"a|b", "'a|b'"},
	}
	for _, c := range cases {
		if got := shellQuote(c.in); got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildHookCmd_QuotesUnsafePaths(t *testing.T) {
	spec := Claude
	cases := []struct {
		bin, dataDir, want string
	}{
		// Safe paths: no quotes injected.
		{"/usr/bin/server", "", "/usr/bin/server event claude"},
		{"/usr/bin/server", "/var/lib/r", "/usr/bin/server event claude -data-dir /var/lib/r"},
		// Spaces in bin path: must be quoted as ONE shell token.
		{"/home/Alice User/bin/server", "", "'/home/Alice User/bin/server' event claude"},
		// Spaces in data-dir: same treatment.
		{"/usr/bin/server", "/var/lib/agent reactor", "/usr/bin/server event claude -data-dir '/var/lib/agent reactor'"},
		// Shell metachars in bin path: also quoted.
		{"/opt/srv$1/server", "", "'/opt/srv$1/server' event claude"},
	}
	for _, c := range cases {
		if got := BuildHookCmd(c.bin, c.dataDir, spec); got != c.want {
			t.Errorf("BuildHookCmd(%q, %q) = %q, want %q", c.bin, c.dataDir, got, c.want)
		}
	}
}

func TestAll_SpecCoverage(t *testing.T) {
	// Lock the supported-agent list and the (Name, SubcmdName, SettingsRel)
	// triple. Drift here is the failure mode the unified All slice was
	// added to prevent — both cmd/server and cmd/reactor-bridge consume
	// this same slice, so omissions surface here, not as silent partial
	// coverage at runtime.
	want := []Spec{Claude, Gemini}
	if len(All) != len(want) {
		t.Fatalf("len(All) = %d, want %d", len(All), len(want))
	}
	seenSubcmd := map[string]bool{}
	seenSettings := map[string]bool{}
	for i, spec := range All {
		if spec.Name == "" {
			t.Errorf("All[%d].Name is empty", i)
		}
		if spec.SubcmdName == "" {
			t.Errorf("All[%d] (%s).SubcmdName is empty", i, spec.Name)
		}
		if spec.SettingsRel == "" {
			t.Errorf("All[%d] (%s).SettingsRel is empty", i, spec.Name)
		}
		if len(spec.Events) == 0 {
			t.Errorf("All[%d] (%s).Events is empty", i, spec.Name)
		}
		if seenSubcmd[spec.SubcmdName] {
			t.Errorf("duplicate SubcmdName %q", spec.SubcmdName)
		}
		seenSubcmd[spec.SubcmdName] = true
		if seenSettings[spec.SettingsRel] {
			t.Errorf("duplicate SettingsRel %q", spec.SettingsRel)
		}
		seenSettings[spec.SettingsRel] = true
		// Convention: SubcmdName = "<Name>-setup-hooks". The bridge dispatcher
		// is keyed on SubcmdName, so a violation silently breaks postCreate.
		wantSub := spec.Name + "-setup-hooks"
		if spec.SubcmdName != wantSub {
			t.Errorf("All[%d] SubcmdName = %q, want %q (convention <Name>-setup-hooks)",
				i, spec.SubcmdName, wantSub)
		}
	}
}

func TestScanList_FindsBothIndices(t *testing.T) {
	// scanList does a full walk and returns the first index of each kind.
	// The exact-AND-stale case is what applyToHooks uses to delete the
	// orphan — without it the agent would double-fire on every event.
	stale := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "/old/server event claude"}}}
	exact := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "/new/server event claude"}}}

	cases := []struct {
		name                          string
		list                          []any
		hookCmd, marker               string
		wantExactIdx, wantStaleIdxRes int
	}{
		{
			"stale_then_exact_returns_both",
			[]any{stale, exact},
			"/new/server event claude", " event claude",
			1, 0,
		},
		{
			"exact_then_stale_returns_both",
			[]any{exact, stale},
			"/new/server event claude", " event claude",
			0, 1,
		},
		{
			"only_stale",
			[]any{stale},
			"/new/server event claude", " event claude",
			-1, 0,
		},
		{
			"only_exact",
			[]any{exact},
			"/new/server event claude", " event claude",
			0, -1,
		},
		{
			"empty_marker_disables_stale",
			[]any{stale},
			"/new/server event claude", "",
			-1, -1,
		},
		{
			"empty_list",
			nil,
			"/x event claude", " event claude",
			-1, -1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotExact, gotStale := scanList(c.list, c.hookCmd, c.marker)
			if gotExact != c.wantExactIdx {
				t.Errorf("exactIdx = %d, want %d", gotExact, c.wantExactIdx)
			}
			if gotStale != c.wantStaleIdxRes {
				t.Errorf("staleIdx = %d, want %d", gotStale, c.wantStaleIdxRes)
			}
		})
	}
}

func TestInstall_DeletesOrphanWhenExactAlsoPresent(t *testing.T) {
	// User (or a botched migration) leaves a stale reactor entry behind
	// AND the current entry is also registered. Without orphan removal,
	// the agent fires both commands on every event; with it, the stale
	// is dropped and the list ends with just one current entry.
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	currentCmd := "/new/path/server event claude"

	seed := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{map[string]any{"type": "command", "command": "/old/path/server event claude"}},
				},
				map[string]any{
					"hooks": []any{map[string]any{"type": "command", "command": currentCmd}},
				},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	registered, err := Install(path, currentCmd, Claude)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	foundInRegistered := false
	for _, ev := range registered {
		if ev == "SessionStart" {
			foundInRegistered = true
			break
		}
	}
	if !foundInRegistered {
		t.Errorf("SessionStart not in registered list; orphan was not removed")
	}

	cmds := commandsFor(t, readSettings(t, path), "SessionStart")
	if len(cmds) != 1 {
		t.Errorf("SessionStart cmds = %v, want exactly one (orphan removed)", cmds)
	}
	if len(cmds) == 1 && cmds[0] != currentCmd {
		t.Errorf("SessionStart surviving cmd = %q, want %q", cmds[0], currentCmd)
	}
}
