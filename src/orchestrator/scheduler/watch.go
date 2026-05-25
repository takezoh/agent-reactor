package scheduler

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// watchWorkflow watches the file at path for changes and sends on ch.
// The parent directory is watched so atomic saves (rename-over) are detected.
// Events are coalesced: if ch already has a pending signal, the new one is dropped.
func watchWorkflow(_ context.Context, path string, ch chan<- struct{}) (io.Closer, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(filepath.Dir(path)); err != nil {
		_ = w.Close()
		return nil, err
	}
	base := filepath.Base(path)
	go func() {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Base(ev.Name) != base {
					continue
				}
				if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Rename) {
					continue
				}
				select {
				case ch <- struct{}{}:
				default:
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				slog.Debug("scheduler: fsnotify error", "err", err)
			}
		}
	}()
	return w, nil
}
