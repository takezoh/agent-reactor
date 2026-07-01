package systemdnotify

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadyNoopWithoutNotifySocket(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")

	if err := Ready(); err != nil {
		t.Fatalf("Ready() error = %v, want nil", err)
	}
}

func TestReadySendsReadyMessage(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "notify.sock")
	pc, err := net.ListenPacket("unixgram", sockPath)
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer pc.Close()

	t.Setenv("NOTIFY_SOCKET", sockPath)

	errCh := make(chan error, 1)
	msgCh := make(chan string, 1)
	go func() {
		buf := make([]byte, 256)
		_ = pc.SetDeadline(time.Now().Add(2 * time.Second))
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			errCh <- err
			return
		}
		msgCh <- string(buf[:n])
	}()

	if err := Ready(); err != nil {
		t.Fatalf("Ready() error = %v, want nil", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("read notify message: %v", err)
	case msg := <-msgCh:
		if got := strings.TrimSpace(msg); got != "READY=1" {
			t.Fatalf("notify payload = %q, want READY=1", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notify payload")
	}
}

func TestReadyErrorsForMissingSocket(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", filepath.Join(t.TempDir(), "missing.sock"))

	if err := Ready(); err == nil {
		t.Fatal("Ready() error = nil, want error for missing notify socket")
	}
}
