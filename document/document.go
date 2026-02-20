package document

import (
	"sync"

	"github.com/LukasParke/gossip/protocol"
)

// Document represents a single managed text document.
type Document struct {
	mu         sync.RWMutex
	uri        protocol.DocumentURI
	languageID string
	version    int32
	text       string

	// tree holds the tree-sitter parse tree (set by treesitter.Manager).
	// Typed as interface{} to avoid import cycle; treesitter package casts it.
	tree interface{}
	// onEdit is called by the treesitter manager to get edit ranges
	onTreeEdit func(edits []EditRange)
}

// New creates a new Document from an LSP TextDocumentItem.
func New(item protocol.TextDocumentItem) *Document {
	return &Document{
		uri:        item.URI,
		languageID: item.LanguageID,
		version:    item.Version,
		text:       item.Text,
	}
}

// URI returns the document's URI.
func (d *Document) URI() protocol.DocumentURI {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.uri
}

// LanguageID returns the LSP language identifier (e.g., "go", "python").
func (d *Document) LanguageID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.languageID
}

// Version returns the document's current version number.
func (d *Document) Version() int32 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.version
}

// Text returns the full text content of the document.
func (d *Document) Text() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.text
}

// LineAt returns the text of the given zero-based line number.
func (d *Document) LineAt(line uint32) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return LineAt(d.text, line)
}

// WordAt returns the word under the given position.
func (d *Document) WordAt(pos protocol.Position) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return WordAt(d.text, pos)
}

// OffsetAt converts an LSP position to a byte offset in the document text.
func (d *Document) OffsetAt(pos protocol.Position) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return OffsetAt(d.text, pos)
}

// PositionAt converts a byte offset to an LSP position.
func (d *Document) PositionAt(offset int) protocol.Position {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return PositionAt(d.text, offset)
}

// SetTree sets the tree-sitter tree for this document (called by treesitter.Manager).
func (d *Document) SetTree(tree interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tree = tree
}

// RawTree returns the underlying tree-sitter tree as interface{}.
func (d *Document) RawTree() interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.tree
}

// SetOnTreeEdit sets the callback for tree-sitter edit notifications.
func (d *Document) SetOnTreeEdit(fn func(edits []EditRange)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onTreeEdit = fn
}

// ApplyChanges applies incremental edits and updates the document version.
func (d *Document) ApplyChanges(version int32, changes []protocol.TextDocumentContentChangeEvent) []EditRange {
	d.mu.Lock()
	newText, edits := ApplyChangesWithEdits(d.text, changes)
	d.text = newText
	d.version = version
	cb := d.onTreeEdit
	d.mu.Unlock()

	// Call outside the lock -- the callback (handleEdits) may need to read doc.Text().
	if cb != nil && len(edits) > 0 {
		cb(edits)
	}

	return edits
}
