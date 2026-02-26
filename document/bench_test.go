package document

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LukasParke/gossip/protocol"
)

func BenchmarkStoreOpen(b *testing.B) {
	store := NewStore()
	text := strings.Repeat("line of code\n", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uri := protocol.DocumentURI(fmt.Sprintf("file:///bench_%d.go", i))
		store.Open(&protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				URI:        uri,
				LanguageID: "go",
				Version:    1,
				Text:       text,
			},
		})
	}
}

func BenchmarkStoreGet(b *testing.B) {
	store := NewStore()
	uri := protocol.DocumentURI("file:///bench.go")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri,
			LanguageID: "go",
			Version:    1,
			Text:       "package main\nfunc main() {}\n",
		},
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc := store.Get(uri)
		if doc == nil {
			b.Fatal("unexpected nil")
		}
	}
}

func BenchmarkStoreChange(b *testing.B) {
	for _, lines := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("Lines_%d", lines), func(b *testing.B) {
			store := NewStore()
			uri := protocol.DocumentURI("file:///bench.go")
			text := strings.Repeat("package main\n", lines)
			store.Open(&protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:        uri,
					LanguageID: "go",
					Version:    1,
					Text:       text,
				},
			})
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				store.Change(&protocol.DidChangeTextDocumentParams{
					TextDocument: protocol.VersionedTextDocumentIdentifier{
						TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
						Version:                int32(i + 2),
					},
					ContentChanges: []protocol.TextDocumentContentChangeEvent{
						{
							Range: &protocol.Range{
								Start: protocol.Position{Line: 50, Character: 0},
								End:   protocol.Position{Line: 50, Character: 12},
							},
							Text: "func main()",
						},
					},
				})
			}
		})
	}
}

func BenchmarkWordAt(b *testing.B) {
	store := NewStore()
	uri := protocol.DocumentURI("file:///bench.go")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri,
			LanguageID: "go",
			Version:    1,
			Text:       "package main\n\nfunc hello() string {\n\treturn \"world\"\n}\n",
		},
	})
	doc := store.Get(uri)
	pos := protocol.Position{Line: 2, Character: 7}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doc.WordAt(pos)
	}
}
