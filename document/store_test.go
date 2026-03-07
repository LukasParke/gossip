package document

import (
	"testing"

	"github.com/LukasParke/gossip/protocol"
)

func TestStore_NormalizedLookup(t *testing.T) {
	store := NewStore()

	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///home/user/test.yaml",
			LanguageID: "yaml",
			Version:    1,
			Text:       "openapi: '3.1.0'",
		},
	})

	// Lookup with the exact same URI.
	doc := store.Get("file:///home/user/test.yaml")
	if doc == nil {
		t.Fatal("expected document for exact URI")
	}

	// Lookup with dot-segment variant should resolve to the same document.
	doc2 := store.Get("file:///home/user/sub/../test.yaml")
	if doc2 == nil {
		t.Fatal("expected document for dot-segment variant URI")
	}

	if doc != doc2 {
		t.Fatal("expected same document for both URI variants")
	}
}

func TestStore_CloseWithVariantURI(t *testing.T) {
	store := NewStore()

	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///home/user/test.yaml",
			LanguageID: "yaml",
			Version:    1,
			Text:       "content",
		},
	})

	// Close with a dot-segment variant — should remove the document.
	store.Close(&protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: "file:///home/user/./test.yaml",
		},
	})

	doc := store.Get("file:///home/user/test.yaml")
	if doc != nil {
		t.Fatal("expected document to be removed after close with variant URI")
	}
}
