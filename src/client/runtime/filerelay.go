package runtime

import (
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

// FileRelay watches log and session files via fsnotify, reads new
// bytes when they change, and broadcasts them as EvtLogLine or
// EvtSessionFileLine events to IPC subscribers. This replaces the
// TUI's 200ms polling loop — the TUI just receives and renders.
//
// FileRelay owns its own fsnotify watcher (separate from the
// runtime's transcript watcher, which feeds the driver state
// machine). Each file is tracked independently with an offset and
// a "dirty" flag. A background goroutine runs a 100ms sweep that
// reads all dirty files and broadcasts new content in one batch.
type FileRelay struct {
	mu      sync.Mutex
	watcher *fsnotify.Watcher
	files   map[string]*relayFile
	// send posts an internal event onto the runtime event loop. FileRelay
	// holds only this bound function (not *Runtime) so its background sweep
	// goroutine cannot touch loop-owned state (conns / Subscribers) directly.
	// Returns true on accept, false on a saturated internalCh. FileRelay uses
	// the return value to roll back dirty/offset state so the next sweep tick
	// re-reads and re-broadcasts the same content (no permanent log line loss).
	send func(internalEvent) bool

	stop chan struct{}
	wg   sync.WaitGroup
}

type relayFile struct {
	path    string
	frameID state.FrameID // empty for app log
	kind    string        // "log" or "transcript"
	offset  int64
	dirty   bool
}

const relaySweepInterval = 100 * time.Millisecond

// NewFileRelay creates and starts a file relay for the given runtime.
func NewFileRelay(rt *Runtime) (*FileRelay, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	fr := &FileRelay{
		watcher: w,
		files:   map[string]*relayFile{},
		send:    rt.enqueueInternal,
		stop:    make(chan struct{}),
	}
	fr.wg.Add(2)
	go fr.watchLoop()
	go fr.sweepLoop()
	return fr, nil
}

// WatchLog registers the app log file for push relay.
func (fr *FileRelay) WatchLog(path string) {
	fr.add(path, "", "log")
}

// WatchFile registers a session file (transcript, event-log, etc.) for push relay.
func (fr *FileRelay) WatchFile(frameID state.FrameID, path string, kind string) {
	fr.add(path, frameID, kind)
}

// UnwatchFile removes all files associated with a frame from the relay.
func (fr *FileRelay) UnwatchFile(frameID state.FrameID) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	for path, f := range fr.files {
		if f.frameID == frameID {
			_ = fr.watcher.Remove(path)
			delete(fr.files, path)
		}
	}
}

// Unwatch removes a file from the relay by path.
func (fr *FileRelay) Unwatch(path string) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if _, ok := fr.files[path]; ok {
		_ = fr.watcher.Remove(path)
		delete(fr.files, path)
	}
}

// Close shuts down the relay, waiting for both goroutines to exit.
func (fr *FileRelay) Close() {
	close(fr.stop)
	fr.wg.Wait()
	fr.watcher.Close()
}

func (fr *FileRelay) add(path string, frameID state.FrameID, kind string) {
	if path == "" {
		return
	}
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if _, ok := fr.files[path]; ok {
		return
	}
	// Seek to end — we don't backfill from the relay. The TUI does
	// its own backfill on tab switch via direct file reads.
	var offset int64
	info, statErr := os.Stat(path)
	if statErr == nil {
		offset = info.Size()
	} else if os.IsNotExist(statErr) {
		// Touch the file so fsnotify.Add succeeds. The parent directory
		// must already exist (FileEventLog.Append creates it on first
		// write, and syncRelayWatches is called after the file-create
		// effect in the normal flow). If the directory is missing we
		// skip the touch and rely on the next reconciliation cycle.
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			f.Close()
		}
	}
	fr.files[path] = &relayFile{
		path:    path,
		frameID: frameID,
		kind:    kind,
		offset:  offset,
	}
	if err := fr.watcher.Add(path); err != nil {
		slog.Debug("filerelay: watch failed", "path", path, "err", err)
	}
}

// watchLoop listens for fsnotify events and marks files as dirty.
func (fr *FileRelay) watchLoop() {
	defer fr.wg.Done()
	for {
		select {
		case <-fr.stop:
			return
		case ev, ok := <-fr.watcher.Events:
			if !ok {
				return
			}
			if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) {
				continue
			}
			fr.mu.Lock()
			if f, ok := fr.files[ev.Name]; ok {
				f.dirty = true
			}
			fr.mu.Unlock()
		case err, ok := <-fr.watcher.Errors:
			if !ok {
				return
			}
			slog.Debug("filerelay: fsnotify error", "err", err)
		}
	}
}

// sweepLoop runs every relaySweepInterval and reads all dirty files.
func (fr *FileRelay) sweepLoop() {
	defer fr.wg.Done()
	ticker := time.NewTicker(relaySweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-fr.stop:
			return
		case <-ticker.C:
			fr.sweep()
		}
	}
}

func (fr *FileRelay) sweep() {
	fr.mu.Lock()
	// Snapshot dirty files under lock, then release for I/O. Capture the
	// pre-read offset alongside each file so a broadcast drop can roll it back.
	type pending struct {
		f         *relayFile
		oldOffset int64
	}
	var dirty []pending
	for _, f := range fr.files {
		if f.dirty {
			f.dirty = false
			dirty = append(dirty, pending{f: f, oldOffset: f.offset})
		}
	}
	fr.mu.Unlock()

	for _, p := range dirty {
		content, newOffset := readFrom(p.f.path, p.oldOffset)
		if content == "" {
			continue
		}
		fr.mu.Lock()
		p.f.offset = newOffset
		fr.mu.Unlock()

		if !fr.broadcast(p.f, content) {
			// internalCh saturation dropped our broadcast. Roll back so the
			// next sweep tick re-reads and re-broadcasts the same content.
			// dirty=true means watchLoop's subsequent Write events keep us
			// armed even between retries.
			fr.mu.Lock()
			p.f.offset = p.oldOffset
			p.f.dirty = true
			fr.mu.Unlock()
		}
	}
}

func readFrom(path string, offset int64) (string, int64) {
	file, err := os.Open(path)
	if err != nil {
		return "", offset
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", offset
	}
	// Truncation detection
	if info.Size() < offset {
		offset = 0
	}
	if info.Size() == offset {
		return "", offset
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", offset
	}
	data, err := io.ReadAll(io.LimitReader(file, info.Size()-offset))
	if err != nil {
		return "", offset
	}
	return string(data), offset + int64(len(data))
}

// broadcast posts the read content to the runtime event loop. Returns true
// when the event was accepted, false on a saturated internalCh — the sweep
// caller uses this signal to roll back dirty/offset so the next tick retries.
// Encode errors are treated as "delivered" (no retry possible): the line is
// lost but the offset advance is correct because the bytes were consumed.
func (fr *FileRelay) broadcast(f *relayFile, content string) bool {
	var event proto.ServerEvent
	if f.frameID == "" {
		event = proto.EvtLogLine{Path: f.path, Line: content}
	} else {
		event = proto.EvtSessionFileLine{
			SessionID: string(f.frameID),
			Kind:      f.kind,
			Line:      content,
		}
	}
	wire, err := proto.EncodeEvent(event)
	if err != nil {
		return true
	}
	return fr.send(internalBroadcastWire{wire: wire, eventName: event.EventName()})
}
