package secretenv

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// writeFakeCredproxy writes a fake "credproxy resolve" executable that emits
// jsonResponse to stdout. The JSON is written to a separate file and cat'd by
// the script to avoid any shell-quoting issues with the response content.
func writeFakeCredproxy(t *testing.T, dir, jsonResponse string) string {
	t.Helper()
	jsonPath := filepath.Join(dir, "resolve-output.json")
	if err := os.WriteFile(jsonPath, []byte(jsonResponse), 0o644); err != nil {
		t.Fatalf("write fake JSON: %v", err)
	}
	path := filepath.Join(dir, "credproxy")
	script := "#!/bin/sh\ncat \"" + jsonPath + "\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake credproxy: %v", err)
	}
	return path
}

// writeFakeCredproxyEcho writes a fake "credproxy resolve" executable that echoes
// its --env-file argument as the PATH_RECEIVED key in the JSON output. Used to
// assert that the broker passes the correct (translated) host path to credproxy.
func writeFakeCredproxyEcho(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "credproxy")
	// args from broker: resolve --env-file <path>
	// $3 is the path after "resolve" and "--env-file".
	script := "#!/bin/sh\nprintf '{\"env\":{\"PATH_RECEIVED\":\"%s\"}}' \"$3\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake credproxy echo: %v", err)
	}
	return path
}

func startTestBroker(t *testing.T, allow []string, credproxyBin, hostPrefix string) string {
	t.Helper()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	br := &broker{
		ctx:                 context.Background(),
		sock:                sockPath,
		ln:                  ln,
		project:             "/test/project",
		gate:                NewGate(allow),
		credproxyBin:        credproxyBin,
		hostPathMountPrefix: hostPrefix,
		onStop:              func() {},
	}
	go br.serve()
	t.Cleanup(func() { ln.Close() })
	return sockPath
}

func sendRequest(t *testing.T, sockPath string, req Request) Response {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func TestBroker_resolves(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "test.env")
	if err := os.WriteFile(envFile, []byte("SECRET=op://vault/item/field\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	fakeJSON := `{"env":{"SECRET":"s3cr3t"}}`
	fakeBin := writeFakeCredproxy(t, dir, fakeJSON)
	sockPath := startTestBroker(t, []string{filepath.Join(dir, "*.env")}, fakeBin, "")

	resp := sendRequest(t, sockPath, Request{EnvFilePath: envFile})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Env["SECRET"] != "s3cr3t" {
		t.Errorf("want SECRET=s3cr3t, got %q", resp.Env["SECRET"])
	}
}

func TestBroker_gateBlocks(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "test.env")
	_ = os.WriteFile(envFile, []byte("SECRET=op://vault/item/field\n"), 0o600)

	// Allow only /other/*.env — different dir. Fake bin never called (gate fires first).
	fakeBin := writeFakeCredproxy(t, dir, `{"env":{"SECRET":"s3cr3t"}}`)
	sockPath := startTestBroker(t, []string{"/other/*.env"}, fakeBin, "")

	resp := sendRequest(t, sockPath, Request{EnvFilePath: envFile})
	if resp.Error == "" {
		t.Fatal("expected error, got nil")
	}
	if len(resp.Env) > 0 {
		t.Errorf("expected no env on gate deny, got %v", resp.Env)
	}
}

func TestBroker_relativePathDenied(t *testing.T) {
	dir := t.TempDir()
	fakeBin := writeFakeCredproxy(t, dir, `{"env":{"SECRET":"s3cr3t"}}`)
	// Allow everything — the reject must come from the absolute-path check, not the gate.
	sockPath := startTestBroker(t, []string{"*"}, fakeBin, "")

	resp := sendRequest(t, sockPath, Request{EnvFilePath: "relative/path.env"})
	if resp.Error == "" {
		t.Fatal("expected error for relative path, got nil")
	}
	if len(resp.Env) > 0 {
		t.Errorf("expected no env on relative path deny, got %v", resp.Env)
	}
}

func TestBroker_containerPathTranslated(t *testing.T) {
	dir := t.TempDir()
	// Fake credproxy echoes back the --env-file arg as PATH_RECEIVED.
	fakeBin := writeFakeCredproxyEcho(t, dir)

	// Simulate devcontainer with prefix /mnt. Container path /mnt/data/x.env
	// should be translated to host path /data/x.env before gate + exec.
	prefix := "/mnt"
	hostDir := "/data"
	hostPattern := hostDir + "/*.env"
	containerPath := prefix + hostDir + "/x.env"

	sockPath := startTestBroker(t, []string{hostPattern}, fakeBin, prefix)

	resp := sendRequest(t, sockPath, Request{EnvFilePath: containerPath})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	got := resp.Env["PATH_RECEIVED"]
	want := hostDir + "/x.env"
	if got != want {
		t.Errorf("credproxy received path %q; want %q (host path after prefix strip)", got, want)
	}
}
