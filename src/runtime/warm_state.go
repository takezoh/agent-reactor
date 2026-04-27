package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-roost/state"
)

// WarmFrameState holds per-frame data that only survives warm restarts.
// On cold start the entire warm/ directory is wiped. On warm start each
// file is read back and its fields are re-registered with the in-memory
// subsystems (tokenStore, container endpoint).
type WarmFrameState struct {
	FrameID        string `json:"frame_id"`
	ContainerToken string `json:"container_token,omitempty"`
}

// warmFrameStore writes one JSON file per frame under <dataDir>/warm/.
// Atomic write (tmp → rename) mirrors the sessions persist pattern.
type warmFrameStore struct {
	dir string // <dataDir>/warm
}

func newWarmFrameStore(dataDir string) (*warmFrameStore, error) {
	dir := filepath.Join(dataDir, "warm")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("warm: mkdir: %w", err)
	}
	return &warmFrameStore{dir: dir}, nil
}

// Save writes or overwrites the warm state for one frame. 0o600: host-only.
func (s *warmFrameStore) Save(st WarmFrameState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("warm: marshal %s: %w", st.FrameID, err)
	}
	target := filepath.Join(s.dir, filepath.Base(st.FrameID)+".json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("warm: write %s: %w", st.FrameID, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("warm: rename %s: %w", st.FrameID, err)
	}
	return nil
}

// Delete removes the warm state file for a frame. No-op if absent.
func (s *warmFrameStore) Delete(frameID state.FrameID) error {
	err := os.Remove(filepath.Join(s.dir, filepath.Base(string(frameID))+".json"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("warm: delete %s: %w", frameID, err)
	}
	return nil
}

// LoadAll reads every frame state file in the directory.
// Files that fail to parse are skipped with a warning; .tmp leftovers and
// non-.json files are ignored. Returns (nil, nil) when the directory does
// not exist.
func (s *warmFrameStore) LoadAll() ([]WarmFrameState, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("warm: readdir: %w", err)
	}

	var out []WarmFrameState
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, name))
		if err != nil {
			slog.Warn("warm: read failed, skipping", "file", name, "err", err)
			continue
		}
		var st WarmFrameState
		if err := json.Unmarshal(data, &st); err != nil {
			slog.Warn("warm: unmarshal failed, skipping", "file", name, "err", err)
			continue
		}
		out = append(out, st)
	}
	return out, nil
}
