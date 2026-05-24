package agentlaunch

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestSpawn_StdioRoundtrip(t *testing.T) {
	ctx := context.Background()
	w := WrappedLaunch{
		Argv:     []string{"cat"},
		StartDir: t.TempDir(),
	}
	res, err := Spawn(ctx, w, SpawnOptions{InheritEnv: true})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	t.Cleanup(func() { _ = res.Wait() })

	want := "hello agentlaunch\n"
	if _, err := io.WriteString(res.Stdin, want); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	res.Stdin.Close()

	got, err := io.ReadAll(res.Stdout)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if string(got) != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if res.PID == 0 {
		t.Error("PID should be non-zero")
	}
}

func TestSpawn_ExitCode(t *testing.T) {
	ctx := context.Background()
	w := WrappedLaunch{
		Argv: []string{"sh", "-c", "exit 7"},
	}
	res, err := Spawn(ctx, w, SpawnOptions{InheritEnv: true})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	_, _ = io.ReadAll(res.Stdout)
	waitErr := res.Wait()
	if waitErr == nil {
		t.Error("Wait() should return an error for exit code 7")
	}
}

func TestSpawn_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := WrappedLaunch{
		Argv: []string{"sleep", "100"},
	}
	res, err := Spawn(ctx, w, SpawnOptions{InheritEnv: true})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	cancel()

	done := make(chan error, 1)
	go func() {
		_, _ = io.ReadAll(res.Stdout)
		done <- res.Wait()
	}()
	select {
	case <-done:
		// process reaped — success
	case <-time.After(10 * time.Second):
		t.Error("process group not reaped within 10s after context cancel")
	}
}

func TestSpawn_StderrCapture(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	w := WrappedLaunch{
		Argv: []string{"sh", "-c", "echo stderr-line >&2"},
	}
	res, err := Spawn(ctx, w, SpawnOptions{InheritEnv: true, Stderr: &buf})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	_, _ = io.ReadAll(res.Stdout)
	_ = res.Wait()
	if !strings.Contains(buf.String(), "stderr-line") {
		t.Errorf("stderr buf = %q, want to contain 'stderr-line'", buf.String())
	}
}

func TestSpawn_EnvInherit(t *testing.T) {
	ctx := context.Background()
	w := WrappedLaunch{
		Argv: []string{"sh", "-c", "echo $HOME"},
		Env:  map[string]string{"EXTRA": "1"},
	}
	res, err := Spawn(ctx, w, SpawnOptions{InheritEnv: true})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	out, _ := io.ReadAll(res.Stdout)
	_ = res.Wait()
	// $HOME should be non-empty when inheriting
	if strings.TrimSpace(string(out)) == "" {
		t.Error("$HOME should be non-empty with InheritEnv=true")
	}
}

func TestSpawn_EmptyArgvError(t *testing.T) {
	ctx := context.Background()
	w := WrappedLaunch{}
	_, err := Spawn(ctx, w, SpawnOptions{})
	if err == nil {
		t.Error("Spawn with empty Argv should return an error")
	}
}
