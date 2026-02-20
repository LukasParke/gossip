// Minimal gossip LSP server: hover support in ~15 lines.
package main

import (
	"log"

	"github.com/gossip-lsp/gossip"
	"github.com/gossip-lsp/gossip/protocol"
)

func main() {
	s := gossip.NewServer("minimal-lsp", "0.1.0")

	s.OnHover(func(ctx *gossip.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
		doc := ctx.Documents.Get(p.TextDocument.URI)
		if doc == nil {
			return nil, nil
		}
		word := doc.WordAt(p.Position)
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "**" + word + "**"},
		}, nil
	})

	if err := gossip.Serve(s, gossip.WithStdio()); err != nil {
		log.Fatal(err)
	}
}
