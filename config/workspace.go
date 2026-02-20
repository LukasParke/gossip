package config

// WorkspaceBridge bridges LSP workspace/didChangeConfiguration notifications
// to the config store. When the editor sends settings updates, this bridge
// triggers the same reload pipeline as file changes.
//
// Usage: register a DidChangeConfiguration handler that calls bridge.HandleChange().
type WorkspaceBridge[T any] struct {
	store    *Store[T]
	filePath string
	defaults *T
}

// NewWorkspaceBridge creates a bridge between workspace configuration and the store.
func NewWorkspaceBridge[T any](store *Store[T], filePath string, defaults *T) *WorkspaceBridge[T] {
	return &WorkspaceBridge[T]{
		store:    store,
		filePath: filePath,
		defaults: defaults,
	}
}

// HandleChange reloads the config from the TOML file and swaps it into the store.
// This is called both by the file watcher and by workspace/didChangeConfiguration.
func (b *WorkspaceBridge[T]) HandleChange() error {
	cfg, err := LoadTOML[T](b.filePath, b.defaults)
	if err != nil {
		return err
	}
	b.store.Swap(cfg)
	return nil
}
