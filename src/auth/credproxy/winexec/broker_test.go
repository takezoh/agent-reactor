package winexec

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/config"
)

// TestBroker_RejectsDisallowedExe spins up a real broker, connects a client over
// Unix socket, sends a request with a name not in the allowlist, and verifies
// that the broker responds with exit code 1 (no exe execution attempted).
func TestBroker_RejectsDisallowedExe(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "winexec.sock")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	br := &broker{
		ctx:     ctx,
		sock:    sock,
		ln:      ln,
		project: "/proj",
		onStop:  func() {},
	}
	cfg := config.WinExecConfig{AllowedExes: []string{"code.exe"}}
	br.cfg.Store(&cfg)

	served := make(chan struct{})
	go func() {
		br.serve()
		close(served)
	}()
	t.Cleanup(func() {
		ln.Close()
		<-served
	})

	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: sock, Net: "unix"})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	r0, w0, _ := os.Pipe()
	r1, w1, _ := os.Pipe()
	r2, w2, _ := os.Pipe()
	defer func() {
		r0.Close()
		w0.Close()
		r1.Close()
		w1.Close()
		r2.Close()
		w2.Close()
	}()

	if err := SendRequest(conn, Request{Name: "evil.exe", Args: []string{"x"}}, [3]int{int(r0.Fd()), int(w1.Fd()), int(w2.Fd())}); err != nil {
		t.Fatalf("send: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 (allowlist rejection)", resp.ExitCode)
	}
}

// TestBroker_serveExitsOnContextCancel verifies the accept loop terminates
// cleanly when the context is cancelled and onStop is invoked exactly once.
func TestBroker_serveExitsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "winexec.sock")

	ctx, cancel := context.WithCancel(context.Background())

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var stopped atomic.Int32
	br := &broker{
		ctx:     ctx,
		sock:    sock,
		ln:      ln,
		project: "/proj",
		onStop:  func() { stopped.Add(1) },
	}
	cfg := config.WinExecConfig{}
	br.cfg.Store(&cfg)

	done := make(chan struct{})
	go func() {
		br.serve()
		close(done)
	}()

	cancel()
	ln.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit within 2s")
	}

	if got := stopped.Load(); got != 1 {
		t.Errorf("onStop called %d times, want 1", got)
	}

	// Socket file should be removed by serve's defer.
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Errorf("socket %s still exists after serve exit", sock)
	}
}
