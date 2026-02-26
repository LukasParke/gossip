package document

import (
	"sync"
	"testing"

	"github.com/LukasParke/gossip/protocol"
)

func TestStoreOpenGetClose(t *testing.T) {
	store := NewStore()

	// Open a document
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main",
		},
	})

	doc := store.Get("file:///test.go")
	if doc == nil {
		t.Fatal("Get returned nil after Open")
	}
	if doc.URI() != "file:///test.go" {
		t.Errorf("URI = %q, want file:///test.go", doc.URI())
	}
	if doc.LanguageID() != "go" {
		t.Errorf("LanguageID = %q, want go", doc.LanguageID())
	}
	if doc.Version() != 1 {
		t.Errorf("Version = %d, want 1", doc.Version())
	}
	if doc.Text() != "package main" {
		t.Errorf("Text = %q, want package main", doc.Text())
	}

	// Close the document
	store.Close(&protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.go"},
	})

	doc = store.Get("file:///test.go")
	if doc != nil {
		t.Errorf("Get returned non-nil after Close, got %v", doc)
	}
}

func TestStoreChange(t *testing.T) {
	store := NewStore()

	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main",
		},
	})

	// Apply incremental change via full replacement
	store.Change(&protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: "file:///test.go"},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{Text: "package foo"},
		},
	})

	doc := store.Get("file:///test.go")
	if doc == nil {
		t.Fatal("Get returned nil")
	}
	if doc.Text() != "package foo" {
		t.Errorf("Text = %q, want package foo", doc.Text())
	}
	if doc.Version() != 2 {
		t.Errorf("Version = %d, want 2", doc.Version())
	}
}

func TestStoreURIs(t *testing.T) {
	store := NewStore()

	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///a.go",
			LanguageID: "go",
			Version:    1,
			Text:       "a",
		},
	})
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///b.go",
			LanguageID: "go",
			Version:    1,
			Text:       "b",
		},
	})
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///c.go",
			LanguageID: "go",
			Version:    1,
			Text:       "c",
		},
	})

	uris := store.URIs()
	if len(uris) != 3 {
		t.Errorf("URIs() returned %d items, want 3", len(uris))
	}

	uriSet := make(map[protocol.DocumentURI]bool)
	for _, u := range uris {
		uriSet[u] = true
	}
	for _, want := range []protocol.DocumentURI{"file:///a.go", "file:///b.go", "file:///c.go"} {
		if !uriSet[want] {
			t.Errorf("URIs() missing %q", want)
		}
	}

	store.Close(&protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///b.go"},
	})

	uris = store.URIs()
	if len(uris) != 2 {
		t.Errorf("URIs() after close returned %d items, want 2", len(uris))
	}
	uriSet = make(map[protocol.DocumentURI]bool)
	for _, u := range uris {
		uriSet[u] = true
	}
	if uriSet["file:///b.go"] {
		t.Error("URIs() still contains closed file:///b.go")
	}
}

func TestStoreOnOpen(t *testing.T) {
	store := NewStore()

	var receivedDoc *Document
	store.OnOpen(func(doc *Document) {
		receivedDoc = doc
	})

	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main",
		},
	})

	if receivedDoc == nil {
		t.Fatal("OnOpen callback was not called")
	}
	if receivedDoc.URI() != "file:///test.go" {
		t.Errorf("callback doc URI = %q, want file:///test.go", receivedDoc.URI())
	}
	if receivedDoc.Text() != "package main" {
		t.Errorf("callback doc Text = %q, want package main", receivedDoc.Text())
	}
}

func TestStoreOnClose(t *testing.T) {
	store := NewStore()

	var receivedURI protocol.DocumentURI
	store.OnClose(func(uri protocol.DocumentURI) {
		receivedURI = uri
	})

	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.go",
			LanguageID: "go",
			Version:    1,
			Text:       "x",
		},
	})

	store.Close(&protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.go"},
	})

	if receivedURI != "file:///test.go" {
		t.Errorf("OnClose callback URI = %q, want file:///test.go", receivedURI)
	}
}

func TestStoreConcurrent(t *testing.T) {
	store := NewStore()
	const goroutines = 20
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			uri := protocol.DocumentURI("file:///concurrent_" + string(rune('a'+n)) + ".go")
			store.Open(&protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:        uri,
					LanguageID: "go",
					Version:    1,
					Text:       "package main",
				},
			})
			doc := store.Get(uri)
			if doc != nil {
				store.Change(&protocol.DidChangeTextDocumentParams{
					TextDocument: protocol.VersionedTextDocumentIdentifier{
						TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
						Version:                2,
					},
					ContentChanges: []protocol.TextDocumentContentChangeEvent{
						{Text: "package modified"},
					},
				})
			}
			store.Close(&protocol.DidCloseTextDocumentParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			})
		}(i)
	}

	wg.Wait()
}

func TestDocumentApplyChangesIncremental(t *testing.T) {
	doc := New(protocol.TextDocumentItem{
		URI:        "file:///test.go",
		LanguageID: "go",
		Version:    1,
		Text:       "package main\n\nfunc main() {\n}\n",
	})

	// Replace "main" with "foo" on line 0 (characters 8-12)
	doc.ApplyChanges(2, []protocol.TextDocumentContentChangeEvent{
		{
			Range: &protocol.Range{
				Start: protocol.Position{Line: 0, Character: 8},
				End:   protocol.Position{Line: 0, Character: 12},
			},
			Text: "foo",
		},
	})

	if doc.Text() != "package foo\n\nfunc main() {\n}\n" {
		t.Errorf("Text = %q, want package foo\\n\\nfunc main() {\\n}\\n", doc.Text())
	}
	if doc.Version() != 2 {
		t.Errorf("Version = %d, want 2", doc.Version())
	}
}

func TestDocumentApplyChangesFullReplacement(t *testing.T) {
	doc := New(protocol.TextDocumentItem{
		URI:        "file:///test.go",
		LanguageID: "go",
		Version:    1,
		Text:       "package main",
	})

	doc.ApplyChanges(2, []protocol.TextDocumentContentChangeEvent{
		{Range: nil, Text: "package bar\n\nfunc init() {}\n"},
	})

	if doc.Text() != "package bar\n\nfunc init() {}\n" {
		t.Errorf("Text = %q, want package bar\\n\\nfunc init() {}\\n", doc.Text())
	}
	if doc.Version() != 2 {
		t.Errorf("Version = %d, want 2", doc.Version())
	}
}

func TestDocumentEdgeCases(t *testing.T) {
	// Empty text
	doc := New(protocol.TextDocumentItem{
		URI:        "file:///empty.go",
		LanguageID: "go",
		Version:    1,
		Text:       "",
	})
	if doc.Text() != "" {
		t.Errorf("empty Text = %q", doc.Text())
	}
	if ln := doc.LineAt(0); ln != "" {
		t.Errorf("empty LineAt(0) = %q, want empty string", ln)
	}
	pos := doc.PositionAt(100)
	if pos.Line != 0 || pos.Character != 0 {
		t.Errorf("PositionAt(100) beyond EOF = %+v, want Line=0 Character=0", pos)
	}
	off := doc.OffsetAt(protocol.Position{Line: 10, Character: 5})
	if off != 0 {
		t.Errorf("OffsetAt beyond EOF = %d, want 0", off)
	}

	// Single line no newline
	doc = New(protocol.TextDocumentItem{
		URI:        "file:///single.go",
		LanguageID: "go",
		Version:    1,
		Text:       "hello",
	})
	if doc.LineAt(0) != "hello" {
		t.Errorf("single line LineAt(0) = %q, want hello", doc.LineAt(0))
	}
	if doc.LineAt(1) != "" {
		t.Errorf("single line LineAt(1) = %q, want empty (no line 1)", doc.LineAt(1))
	}

	// Position beyond EOF
	doc = New(protocol.TextDocumentItem{
		URI:        "file:///short.go",
		LanguageID: "go",
		Version:    1,
		Text:       "ab",
	})
	off = doc.OffsetAt(protocol.Position{Line: 0, Character: 100})
	if off != 2 {
		t.Errorf("OffsetAt beyond line = %d, want 2 (len)", off)
	}
	off = doc.OffsetAt(protocol.Position{Line: 5, Character: 0})
	if off != 2 {
		t.Errorf("OffsetAt beyond EOF (line 5) = %d, want 2", off)
	}
}
