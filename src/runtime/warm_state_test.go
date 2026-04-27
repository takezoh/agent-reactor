package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/state"
)

func TestWarmFrameStoreSaveLoadAll(t *testing.T) {
	dir := t.TempDir()
	s, err := newWarmFrameStore(dir)
	if err != nil {
		t.Fatalf("newWarmFrameStore: %v", err)
	}

	want := WarmFrameState{FrameID: "f1", ContainerToken: "tok123"}
	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("LoadAll: got %d entries, want 1", len(all))
	}
	if all[0] != want {
		t.Fatalf("LoadAll: got %+v, want %+v", all[0], want)
	}
}

func TestWarmFrameStoreDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := newWarmFrameStore(dir)
	if err != nil {
		t.Fatalf("newWarmFrameStore: %v", err)
	}

	_ = s.Save(WarmFrameState{FrameID: "f2", ContainerToken: "tok"})
	if err := s.Delete(state.FrameID("f2")); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	all, _ := s.LoadAll()
	if len(all) != 0 {
		t.Fatalf("expected empty after Delete, got %d entries", len(all))
	}
}

func TestWarmFrameStoreDeleteNonExistent(t *testing.T) {
	dir := t.TempDir()
	s, _ := newWarmFrameStore(dir)
	// Must not error on absent file.
	if err := s.Delete(state.FrameID("ghost")); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}

func TestWarmFrameStoreFilePermissions(t *testing.T) {
	dir := t.TempDir()
	s, _ := newWarmFrameStore(dir)
	_ = s.Save(WarmFrameState{FrameID: "f3", ContainerToken: "tok"})

	info, err := os.Stat(filepath.Join(s.dir, "f3.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected 0600, got %04o", perm)
	}
}

func TestWarmFrameStoreLoadAllSkipsTmpAndBadJSON(t *testing.T) {
	dir := t.TempDir()
	s, _ := newWarmFrameStore(dir)

	// Write a valid entry.
	_ = s.Save(WarmFrameState{FrameID: "good", ContainerToken: "tok"})

	// Simulate abandoned .tmp file (rename didn't complete).
	_ = os.WriteFile(filepath.Join(s.dir, "f-crash.json.tmp"), []byte(`{}`), 0o600)

	// Simulate corrupted JSON.
	_ = os.WriteFile(filepath.Join(s.dir, "bad.json"), []byte(`not json`), 0o600)

	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 1 || all[0].FrameID != "good" {
		t.Fatalf("expected only 'good' entry, got %+v", all)
	}
}

func TestWarmFrameStoreLoadAllMissingDir(t *testing.T) {
	dir := t.TempDir()
	s := &warmFrameStore{dir: filepath.Join(dir, "nonexistent")}
	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty, got %d", len(all))
	}
}
