package web

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Events pushed to browsers when definitions change on disk.
const (
	eventComponents = "components_changed"
	eventPresets    = "presets_changed"
)

var (
	// watchRetry is how often directories that do not exist yet are retried.
	watchRetry = 2 * time.Second
	// watchDebounce coalesces the burst of events an editor save produces.
	watchDebounce = 150 * time.Millisecond
)

// watchSources watches the components/ and presets/ subdirectories of each
// directory source and sends an event name to out when a definition is
// created, changed or removed. A directory that does not exist yet is picked
// up once it appears; its appearance counts as a change.
func watchSources(ctx context.Context, dirs []string, out chan<- string, log *slog.Logger) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	eventFor := make(map[string]string) // watched directory → event name
	for _, d := range dirs {
		eventFor[filepath.Join(d, "components")] = eventComponents
		eventFor[filepath.Join(d, "presets")] = eventPresets
	}

	watched := make(map[string]bool)
	pending := make(map[string]bool)
	ensure := func(initial bool) {
		for dir, ev := range eventFor {
			if watched[dir] {
				continue
			}
			if err := w.Add(dir); err != nil {
				continue // missing for now; a missing directory behaves as empty
			}
			watched[dir] = true
			if !initial {
				pending[ev] = true
			}
		}
	}
	ensure(true)

	retry := time.NewTicker(watchRetry)
	defer retry.Stop()
	var flush *time.Timer
	var flushC <-chan time.Time
	schedule := func() {
		if flush == nil {
			flush = time.NewTimer(watchDebounce)
			flushC = flush.C
		}
	}
	defer func() {
		if flush != nil {
			flush.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-w.Events:
			if !ok {
				return nil
			}
			if ev, self := eventFor[e.Name]; self {
				// The watched directory itself went away; retry re-adds it.
				if e.Has(fsnotify.Remove) || e.Has(fsnotify.Rename) {
					delete(watched, e.Name)
					pending[ev] = true
					schedule()
				}
				continue
			}
			ev, ok := eventFor[filepath.Dir(e.Name)]
			if !ok || e.Op == fsnotify.Chmod || !strings.HasSuffix(e.Name, ".yaml") {
				continue
			}
			pending[ev] = true
			schedule()
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Warn("file watcher", "err", err)
		case <-retry.C:
			ensure(false)
			if len(pending) > 0 {
				schedule()
			}
		case <-flushC:
			flush, flushC = nil, nil
			for ev := range pending {
				select {
				case out <- ev:
				case <-ctx.Done():
					return nil
				}
				delete(pending, ev)
			}
		}
	}
}
