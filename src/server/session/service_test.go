package session

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/termvt"
)

func newTestService() *Service { return NewService(agentlaunch.DirectDispatcher{}) }

func TestServiceCreateListStop(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	defer svc.CloseAll(ctx)

	info, err := svc.Create(ctx, Spec{Command: "sleep 5", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	if info.ID == "" || info.Command != "sleep 5" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if got := svc.List(); len(got) != 1 || got[0].ID != info.ID {
		t.Fatalf("List() = %+v", got)
	}
	if _, ok := svc.Session(info.ID); !ok {
		t.Fatal("expected live session")
	}
	if err := svc.Stop(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	if len(svc.List()) != 0 {
		t.Fatal("session still listed after Stop")
	}
	if err := svc.Stop(ctx, info.ID); err == nil {
		t.Fatal("expected error stopping missing session")
	}
}

func TestServiceCreateRunsCommand(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	defer svc.CloseAll(ctx)

	info, err := svc.Create(ctx, Spec{Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	sess, ok := svc.Session(info.ID)
	if !ok {
		t.Fatal("no session")
	}
	_, ch := sess.Subscribe()
	sess.WriteInput([]byte("echo-back\n"))

	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Kind == termvt.EventOutput && bytes.Contains(ev.Data, []byte("echo-back")) {
				return
			}
		case <-deadline:
			t.Fatal("did not observe echoed output")
		}
	}
}

func TestServiceCreateEmptyCommand(t *testing.T) {
	svc := newTestService()
	if _, err := svc.Create(context.Background(), Spec{Command: ""}); err == nil {
		t.Fatal("expected error for empty command")
	}
}
