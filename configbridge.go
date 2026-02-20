package gossip

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	gossipconfig "github.com/gossip-lsp/gossip/config"
)

// configHolder is stored on the server as an interface{} to allow generic config types.
type configHolder interface {
	startWatcher(logger *slog.Logger, rootDir string) error
	close()
}

// typedConfigHolder is the generic implementation of configHolder.
type typedConfigHolder[T any] struct {
	store   *gossipconfig.Store[T]
	bridge  *gossipconfig.WorkspaceBridge[T]
	watcher *gossipconfig.Watcher

	filename string
	defaults *T
}

// WithConfig enables a typed configuration system with hot-reload.
// The filename is relative to the workspace root (e.g., ".my-lang.toml").
// The defaults value is used when no config file exists.
func WithConfig[T any](filename string, defaults T) Option {
	return func(s *Server) {
		initial := defaults
		holder := &typedConfigHolder[T]{
			store:    gossipconfig.NewStore(&initial),
			filename: filename,
			defaults: &defaults,
		}
		s.configHolder = holder
	}
}

// Config retrieves the current typed config from the context.
// T must match the type used in WithConfig.
func Config[T any](ctx *Context) *T {
	if ctx.server.configHolder == nil {
		return nil
	}
	if h, ok := ctx.server.configHolder.(*typedConfigHolder[T]); ok {
		return h.store.Get()
	}
	return nil
}

// OnConfigChange registers a callback for config changes. Must be called
// with the same type T used in WithConfig.
func OnConfigChange[T any](s *Server, fn func(ctx *Context, old, new_ *T)) {
	if s.configHolder == nil {
		return
	}
	if h, ok := s.configHolder.(*typedConfigHolder[T]); ok {
		h.store.OnChange(func(old, new_ *T) {
			ctx := newContext(context.Background(), s)
			fn(ctx, old, new_)
		})
	}
}

func (h *typedConfigHolder[T]) startWatcher(logger *slog.Logger, rootDir string) error {
	fullPath := filepath.Join(rootDir, h.filename)
	h.bridge = gossipconfig.NewWorkspaceBridge(h.store, fullPath, h.defaults)

	// Load initial config from file if it exists
	if _, err := os.Stat(fullPath); err == nil {
		if err := h.bridge.HandleChange(); err != nil {
			logger.Warn("failed to load initial config", "path", fullPath, "error", err)
		}
	}

	watcher, err := gossipconfig.NewWatcher(fullPath, func() {
		if err := h.bridge.HandleChange(); err != nil {
			logger.Warn("failed to reload config", "path", fullPath, "error", err)
		}
	}, gossipconfig.WithWatcherLogger(logger))
	if err != nil {
		// File watching is best-effort; log and continue if it fails
		logger.Warn("failed to start config watcher", "path", fullPath, "error", err)
		return nil
	}
	h.watcher = watcher
	return nil
}

func (h *typedConfigHolder[T]) close() {
	if h.watcher != nil {
		h.watcher.Close()
	}
}
