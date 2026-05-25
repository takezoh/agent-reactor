package notify

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"a&b", "a&amp;b"},
		{"<tag>", "&lt;tag&gt;"},
		{`say "hi"`, "say &quot;hi&quot;"},
		{"it's", "it&apos;s"},
		{"<a>&\"'</a>", "&lt;a&gt;&amp;&quot;&apos;&lt;/a&gt;"},
	}
	for _, tt := range tests {
		if got := xmlEscape(tt.input); got != tt.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestNew_NoPSBackend verifies that New skips the PowerShell backend when
// powershell.exe is absent or scriptPath is empty, returning a no-op Notifier.
func TestNew_NoPSBackend(t *testing.T) {
	t.Setenv("PATH", "")
	n, err := New(context.Background(), "")
	if err != nil {
		t.Fatalf("New should return nil error, got: %v", err)
	}
	if n == nil {
		t.Fatal("New should return non-nil Notifier")
	}
	if n.psPath != "" {
		t.Errorf("Notifier should have empty psPath, got %q", n.psPath)
	}
}

// TestNotifier_Send_NoPowerShell verifies that Send on a no-op Notifier returns nil.
func TestNotifier_Send_NoPowerShell(t *testing.T) {
	n := &Notifier{} // no-op: psPath is ""
	if err := n.Send(context.Background(), "title", "body"); err != nil {
		t.Errorf("no-op Notifier.Send should return nil, got: %v", err)
	}
}

func TestNotifySendArgs(t *testing.T) {
	args := notifySendArgs("My Title", "Some body")
	wants := map[string]bool{
		"--app-name=roost":       false,
		"--icon=agent-roost":     false,
		"--category=im.received": false,
	}
	for _, a := range args {
		if _, ok := wants[a]; ok {
			wants[a] = true
		}
	}
	for flag, found := range wants {
		if !found {
			t.Errorf("notify-send args missing %q", flag)
		}
	}
	last2 := args[len(args)-2:]
	if last2[0] != "My Title" || last2[1] != "Some body" {
		t.Errorf("title/body not at end of args: %v", args)
	}
}

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{`hello`, `hello`},
		{`say "hi"`, `say \"hi\"`},
		{`back\slash`, `back\\slash`},
		{`a "b" \c`, `a \"b\" \\c`},
	}
	for _, tc := range tests {
		got := escapeAppleScript(tc.in)
		if got != tc.want {
			t.Errorf("escapeAppleScript(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNotifyScriptFileExists(t *testing.T) {
	// go test runs with working directory set to the package directory.
	content, err := os.ReadFile("notify.ps1")
	if err != nil {
		t.Skipf("notify.ps1 not available: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "param") {
		t.Error("notify.ps1 should have param declaration")
	}
	if !strings.Contains(s, "ToastText02") {
		t.Error("notify.ps1 should use ToastText02 template")
	}
	if !strings.Contains(s, "ToastNotificationManager") {
		t.Error("notify.ps1 should call ToastNotificationManager")
	}
	if !strings.Contains(s, `"Roost"`) {
		t.Error(`notify.ps1 should use "Roost" as notifier ID`)
	}
}
