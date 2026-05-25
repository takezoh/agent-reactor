package codexclient_test

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
)

func TestDialUDS(t *testing.T) {
	sockPath := t.TempDir() + "/test.sock"
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer c.CloseNow()
			ctx := r.Context()
			for {
				_, data, err := c.Read(ctx)
				if err != nil {
					return
				}
				_ = c.Write(ctx, websocket.MessageText, data)
			}
		}),
	}
	go srv.Serve(l) //nolint:errcheck

	tr, err := codexclient.DialUDS(sockPath, 3*time.Second)
	if err != nil {
		t.Fatalf("DialUDS: %v", err)
	}
	_ = tr.Close()
}

func TestDialUDS_Timeout(t *testing.T) {
	_, err := codexclient.DialUDS(t.TempDir()+"/noexist.sock", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
