package codexclient

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"
)

// StdioTransport returns a Transport that reads newline-delimited JSON from r
// and writes newline-delimited JSON to w.  Pass os.Stdin/os.Stdout for the
// claude-app-server shim or orchestrator agent-launch paths.
func StdioTransport(r io.Reader, w io.Writer) Transport {
	return &stdioTransport{
		scanner: bufio.NewScanner(r),
		w:       w,
	}
}

// DefaultStdioTransport returns a StdioTransport over os.Stdin/os.Stdout.
func DefaultStdioTransport() Transport {
	return StdioTransport(os.Stdin, os.Stdout)
}

type stdioTransport struct {
	scanner *bufio.Scanner
	w       io.Writer
	mu      sync.Mutex // serialises writes
}

func (t *stdioTransport) ReadMessage(_ context.Context) ([]byte, error) {
	// bufio.Scanner is not context-aware; cancellation is best-effort.
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	b := t.scanner.Bytes()
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

func (t *stdioTransport) WriteMessage(_ context.Context, data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, err := t.w.Write(data); err != nil {
		return err
	}
	_, err := t.w.Write([]byte("\n"))
	return err
}

func (t *stdioTransport) Close() error { return nil }
