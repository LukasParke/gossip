package config

import (
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a config file for changes and triggers a reload callback.
// Events are debounced to avoid rapid re-reads (e.g., editors that write
// files via rename).
type Watcher struct {
	path     string
	debounce time.Duration
	onReload func()
	logger   *slog.Logger

	watcher  *fsnotify.Watcher
	stopOnce sync.Once
	stop     chan struct{}
}

// WatcherOption configures a Watcher.
type WatcherOption func(*Watcher)

// WithDebounce sets the debounce duration (default 100ms).
func WithDebounce(d time.Duration) WatcherOption {
	return func(w *Watcher) { w.debounce = d }
}

// WithWatcherLogger sets the logger for the watcher.
func WithWatcherLogger(l *slog.Logger) WatcherOption {
	return func(w *Watcher) { w.logger = l }
}

// NewWatcher creates a file watcher that calls onReload when the file changes.
func NewWatcher(path string, onReload func(), opts ...WatcherOption) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		path:     path,
		debounce: 100 * time.Millisecond,
		onReload: onReload,
		logger:   slog.Default(),
		watcher:  fsw,
		stop:     make(chan struct{}),
	}
	for _, o := range opts {
		o(w)
	}

	if err := fsw.Add(path); err != nil {
		fsw.Close()
		return nil, err
	}

	go w.run()
	return w, nil
}

func (w *Watcher) run() {
	var timer *time.Timer
	for {
		select {
		case <-w.stop:
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(w.debounce, func() {
					w.logger.Debug("config file changed, reloading", "path", w.path)
					w.onReload()
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("config watcher error", "error", err)
		}
	}
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	w.stopOnce.Do(func() { close(w.stop) })
	return w.watcher.Close()
}
