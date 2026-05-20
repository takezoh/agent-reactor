package framereg_test

import (
	"sync"
	"testing"

	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/runtime/framereg"
	"github.com/takezoh/agent-roost/state"
)

func TestRegisterAndLookup(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame1", "token-abc")

	got, ok := reg.Lookup("token-abc")
	if !ok || got != "frame1" {
		t.Fatalf("Lookup: got (%q, %v), want (frame1, true)", got, ok)
	}
}

func TestLookupUnknownToken(t *testing.T) {
	reg := framereg.New()
	_, ok := reg.Lookup("no-such-token")
	if ok {
		t.Fatal("Lookup of unknown token should return false")
	}
}

func TestRegisterReplacesOldToken(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame1", "old-token")
	reg.Register("frame1", "new-token")

	if _, ok := reg.Lookup("old-token"); ok {
		t.Error("old token should have been removed")
	}
	got, ok := reg.Lookup("new-token")
	if !ok || got != "frame1" {
		t.Fatalf("new token lookup: got (%q, %v)", got, ok)
	}
}

func TestDeleteRemovesTokenAndMounts(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame1", "tok1")
	reg.StoreMounts("frame1", nil)
	reg.Delete("frame1")

	if _, ok := reg.Lookup("tok1"); ok {
		t.Error("token should be gone after Delete")
	}
	if _, ok := reg.GetMounts("frame1"); ok {
		t.Error("mounts should be gone after Delete")
	}
}

func TestGetMounts(t *testing.T) {
	reg := framereg.New()
	if _, ok := reg.GetMounts("f"); ok {
		t.Fatal("GetMounts on empty reg should return false")
	}
	reg.StoreMounts("f", nil)
	_, ok := reg.GetMounts("f")
	if !ok {
		t.Fatal("GetMounts after StoreMounts should return true")
	}
}

func TestRegisterWithMountsAtomic(t *testing.T) {
	reg := framereg.New()
	mounts := pathmap.Mounts{{Host: "/h", Container: "/c"}}
	reg.RegisterWithMounts("f1", "tok1", mounts)

	got, ok := reg.Lookup("tok1")
	if !ok || got != "f1" {
		t.Fatalf("Lookup after RegisterWithMounts: got (%q, %v)", got, ok)
	}
	ms, ok := reg.GetMounts("f1")
	if !ok || len(ms) == 0 {
		t.Fatalf("GetMounts after RegisterWithMounts: got (%v, %v)", ms, ok)
	}

	// Replace with new token — old token should be invalidated.
	reg.RegisterWithMounts("f1", "tok2", nil)
	if _, ok := reg.Lookup("tok1"); ok {
		t.Error("old token should be invalidated after re-register")
	}
	if _, ok := reg.Lookup("tok2"); !ok {
		t.Error("new token should be valid")
	}
}

// TestConcurrentReadWrite verifies the -race detector is satisfied: one
// writer goroutine (event loop) registers tokens while many reader goroutines
// (container endpoint handlers) call Lookup and GetMounts concurrently.
func TestConcurrentReadWrite(t *testing.T) {
	reg := framereg.New()
	const readers = 8
	const iters = 200

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// writer (event loop)
	wg.Go(func() {
		for range iters {
			reg.Register(state.FrameID("frame"), "token")
			reg.StoreMounts("frame", nil)
		}
		close(stop)
	})

	// readers (container endpoint handlers)
	for range readers {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					reg.Lookup("token")
					reg.GetMounts("frame")
				}
			}
		})
	}
	wg.Wait()
}
