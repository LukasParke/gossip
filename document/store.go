// Package document provides a thread-safe document store and position
// utilities for LSP text document management. Documents are automatically
// tracked via didOpen/didChange/didClose notifications and support
// incremental text synchronization.
package document

import (
	"sync"

	"github.com/LukasParke/gossip/protocol"
)

// Store is a thread-safe store of open text documents. It automatically
// tracks documents via didOpen/didChange/didClose notifications.
type Store struct {
	mu   sync.RWMutex
	docs map[protocol.DocumentURI]*Document

	onOpenCallbacks  []func(doc *Document)
	onCloseCallbacks []func(uri protocol.DocumentURI)
}

// NewStore creates a new empty document store.
func NewStore() *Store {
	return &Store{
		docs: make(map[protocol.DocumentURI]*Document),
	}
}

// OnOpen registers a callback called when a document is opened. Multiple
// callbacks can be registered; they fire in registration order.
func (s *Store) OnOpen(fn func(doc *Document)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onOpenCallbacks = append(s.onOpenCallbacks, fn)
}

// OnClose registers a callback called when a document is closed. Multiple
// callbacks can be registered; they fire in registration order.
func (s *Store) OnClose(fn func(uri protocol.DocumentURI)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCloseCallbacks = append(s.onCloseCallbacks, fn)
}

// Get returns the document for the given URI, or nil if not found.
func (s *Store) Get(uri protocol.DocumentURI) *Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

// URIs returns all open document URIs.
func (s *Store) URIs() []protocol.DocumentURI {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uris := make([]protocol.DocumentURI, 0, len(s.docs))
	for uri := range s.docs {
		uris = append(uris, uri)
	}
	return uris
}

// Open adds a document to the store from a didOpen notification.
func (s *Store) Open(params *protocol.DidOpenTextDocumentParams) {
	doc := New(params.TextDocument)

	s.mu.Lock()
	s.docs[params.TextDocument.URI] = doc
	callbacks := make([]func(doc *Document), len(s.onOpenCallbacks))
	copy(callbacks, s.onOpenCallbacks)
	s.mu.Unlock()

	for _, cb := range callbacks {
		cb(doc)
	}
}

// Change applies edits from a didChange notification.
func (s *Store) Change(params *protocol.DidChangeTextDocumentParams) {
	s.mu.RLock()
	doc := s.docs[params.TextDocument.URI]
	s.mu.RUnlock()

	if doc != nil {
		doc.ApplyChanges(params.TextDocument.Version, params.ContentChanges)
	}
}

// Close removes a document from the store.
func (s *Store) Close(params *protocol.DidCloseTextDocumentParams) {
	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	callbacks := make([]func(uri protocol.DocumentURI), len(s.onCloseCallbacks))
	copy(callbacks, s.onCloseCallbacks)
	s.mu.Unlock()

	for _, cb := range callbacks {
		cb(params.TextDocument.URI)
	}
}
