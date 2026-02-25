package index

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors filesystem changes and updates the index.
type Watcher struct {
	idx       *Index
	watcher   *fsnotify.Watcher
	debounce  time.Duration
	onChange  func(path string)
	stopCh    chan struct{}
}

// NewWatcher creates a filesystem watcher that updates the given index.
func NewWatcher(idx *Index) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		idx:      idx,
		watcher:  fw,
		debounce: 500 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}, nil
}

// SetOnChange sets a callback that fires when a path changes.
func (w *Watcher) SetOnChange(fn func(path string)) {
	w.onChange = fn
}

// Watch starts watching the given directories for changes.
func (w *Watcher) Watch(dirs ...string) error {
	for _, dir := range dirs {
		if err := w.watcher.Add(dir); err != nil {
			log.Printf("watcher: cannot watch %s: %v", dir, err)
		}
	}

	go w.loop()
	return nil
}

// Stop stops the watcher.
func (w *Watcher) Stop() error {
	close(w.stopCh)
	return w.watcher.Close()
}

// loop processes filesystem events with debouncing.
func (w *Watcher) loop() {
	// Collect events and process in batches.
	pending := make(map[string]fsnotify.Event)
	timer := time.NewTimer(w.debounce)
	timer.Stop()

	for {
		select {
		case <-w.stopCh:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			pending[event.Name] = event
			timer.Reset(w.debounce)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)

		case <-timer.C:
			w.processBatch(pending)
			pending = make(map[string]fsnotify.Event)
		}
	}
}

// processBatch handles a batch of filesystem events.
func (w *Watcher) processBatch(events map[string]fsnotify.Event) {
	var toUpsert []FileEntry
	var toRemove []string

	for path, event := range events {
		if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			toRemove = append(toRemove, path)
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			toRemove = append(toRemove, path)
			continue
		}

		name := filepath.Base(path)
		parent := filepath.Dir(path)

		entry := FileEntry{
			Path:        path,
			Name:        name,
			ParentPath:  parent,
			Extension:   strings.TrimPrefix(filepath.Ext(name), "."),
			Size:        info.Size(),
			IsDir:       info.IsDir(),
			ModTime:     info.ModTime().Unix(),
			Permissions: int(info.Mode().Perm()),
			Hidden:      strings.HasPrefix(name, "."),
		}
		toUpsert = append(toUpsert, entry)

		// Watch new directories.
		if info.IsDir() && event.Op&fsnotify.Create != 0 {
			w.watcher.Add(path)
		}
	}

	if len(toUpsert) > 0 {
		if err := w.idx.UpsertBatch(toUpsert); err != nil {
			log.Printf("watcher upsert error: %v", err)
		}
	}

	for _, path := range toRemove {
		w.idx.RemovePath(path)
	}

	// Notify frontend about changes.
	if w.onChange != nil {
		for path := range events {
			w.onChange(path)
		}
	}
}
