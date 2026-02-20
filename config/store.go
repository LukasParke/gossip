// Package config provides a generic, hot-reloadable configuration system
// for gossip LSP servers. It supports TOML files, workspace/configuration
// bridge, and fsnotify-based file watching.
package config

import (
	"sync"
	"sync/atomic"
)

// Store holds the current configuration value with atomic read/swap semantics.
// T must be a struct type.
type Store[T any] struct {
	value atomic.Pointer[T]

	mu        sync.RWMutex
	listeners []func(old, new_ *T)
}

// NewStore creates a config store with the given initial value.
func NewStore[T any](initial *T) *Store[T] {
	s := &Store[T]{}
	s.value.Store(initial)
	return s
}

// Get returns the current config value (zero-lock read).
func (s *Store[T]) Get() *T {
	return s.value.Load()
}

// Swap atomically replaces the config and notifies all listeners.
func (s *Store[T]) Swap(new_ *T) *T {
	old := s.value.Swap(new_)

	s.mu.RLock()
	listeners := s.listeners
	s.mu.RUnlock()

	for _, fn := range listeners {
		fn(old, new_)
	}
	return old
}

// OnChange registers a listener called whenever the config changes.
func (s *Store[T]) OnChange(fn func(old, new_ *T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}
