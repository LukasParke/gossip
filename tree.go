package gossip

import (
	"github.com/gossip-lsp/gossip/document"
	"github.com/gossip-lsp/gossip/protocol"
	"github.com/gossip-lsp/gossip/treesitter"
)

// TreeFor returns the tree-sitter tree for the given document, or nil if
// tree-sitter is not enabled or no tree exists for this document.
func TreeFor(doc *document.Document) *treesitter.Tree {
	if doc == nil {
		return nil
	}
	raw := doc.RawTree()
	if raw == nil {
		return nil
	}
	if t, ok := raw.(*treesitter.Tree); ok {
		return t
	}
	return nil
}

// TreeAt is a shortcut that gets the tree for a document at the given URI from the context.
func TreeAt(ctx *Context, uri protocol.DocumentURI) *treesitter.Tree {
	doc := ctx.Documents.Get(uri)
	return TreeFor(doc)
}
