package devcontainer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotenv_BasicKeyValue(t *testing.T) {
	in := strings.NewReader("FOO=bar\nBAZ=qux\n")
	env, err := ParseDotenv(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env["FOO"] != "bar" {
		t.Errorf("FOO = %q, want bar", env["FOO"])
	}
	if env["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want qux", env["BAZ"])
	}
}

func TestParseDotenv_StripsComments(t *testing.T) {
	in := strings.NewReader("# this is a comment\nFOO=bar\n#another\n")
	env, err := ParseDotenv(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(env) != 1 || env["FOO"] != "bar" {
		t.Errorf("env = %v, want only FOO=bar", env)
	}
}

func TestParseDotenv_UnquotesValues(t *testing.T) {
	in := strings.NewReader(`KEY1="double"` + "\n" + `KEY2='single'` + "\nKEY3=plain\n")
	env, err := ParseDotenv(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env["KEY1"] != "double" || env["KEY2"] != "single" || env["KEY3"] != "plain" {
		t.Errorf("env = %v", env)
	}
}

// Values containing '=' signs must be preserved (only the first '=' splits key/value).
func TestParseDotenv_PreservesEqualsInValue(t *testing.T) {
	in := strings.NewReader("URL=https://example.com/?a=1&b=2\nCREDS=user=admin\n")
	env, err := ParseDotenv(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env["URL"] != "https://example.com/?a=1&b=2" {
		t.Errorf("URL = %q, want full URL with query", env["URL"])
	}
	if env["CREDS"] != "user=admin" {
		t.Errorf("CREDS = %q, want user=admin", env["CREDS"])
	}
}

// Mismatched outer quotes are not unquoted; the literal value is preserved.
func TestParseDotenv_MismatchedQuotesPreserved(t *testing.T) {
	in := strings.NewReader(`MIX="open` + "\n" + `MIX2='close"` + "\n")
	env, err := ParseDotenv(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env["MIX"] != `"open` {
		t.Errorf("MIX = %q, want literal \"open", env["MIX"])
	}
	if env["MIX2"] != `'close"` {
		t.Errorf("MIX2 = %q, want literal 'close\"", env["MIX2"])
	}
}

func TestParseDotenv_IgnoresMalformed(t *testing.T) {
	in := strings.NewReader("VALID=ok\n=novalue\nnoequals\n")
	env, err := ParseDotenv(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(env) != 1 || env["VALID"] != "ok" {
		t.Errorf("env = %v, want only VALID=ok", env)
	}
}

func TestRunEnvScript_EmptyPath(t *testing.T) {
	out := RunEnvScript(context.Background(), "", "/project", true)
	if out != nil {
		t.Errorf("expected nil for empty script path, got %v", out)
	}
}

func TestRunEnvScript_NotAllowed(t *testing.T) {
	out := RunEnvScript(context.Background(), "/some/script.sh", "/project", false)
	if out != nil {
		t.Errorf("expected nil when not allowed, got %v", out)
	}
}

func TestRunEnvScript_ScriptOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "envscript.sh")
	const body = "#!/bin/sh\necho FOO=hello\necho BAR=\"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := RunEnvScript(context.Background(), script, "/proj", true)
	if got["FOO"] != "hello" {
		t.Errorf("FOO = %q, want hello", got["FOO"])
	}
	if got["BAR"] != "/proj" {
		t.Errorf("BAR = %q, want /proj", got["BAR"])
	}
}

func TestRunEnvScript_FailingScript(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := RunEnvScript(context.Background(), script, "/proj", true)
	if got != nil {
		t.Errorf("expected nil on failure, got %v", got)
	}
}
