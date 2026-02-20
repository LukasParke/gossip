package gossiptest_test

import (
	"testing"

	"github.com/gossip-lsp/gossip"
	"github.com/gossip-lsp/gossip/gossiptest"
	"github.com/gossip-lsp/gossip/protocol"
)

func TestClientHover(t *testing.T) {
	s := gossip.NewServer("test-server", "0.1.0")
	s.OnHover(func(ctx *gossip.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
		doc := ctx.Documents.Get(p.TextDocument.URI)
		if doc == nil {
			return nil, nil
		}
		word := doc.WordAt(p.Position)
		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: "**" + word + "**",
			},
		}, nil
	})

	c := gossiptest.NewClient(t, s)
	c.Open("file:///test.txt", "hello world")

	hover, err := c.Hover("file:///test.txt", gossiptest.Pos(0, 2))
	if err != nil {
		t.Fatalf("hover error: %v", err)
	}
	gossiptest.AssertHoverContains(t, hover, "hello")
}

func TestClientCompletion(t *testing.T) {
	s := gossip.NewServer("test-server", "0.1.0")
	s.OnCompletion(func(ctx *gossip.Context, p *protocol.CompletionParams) (*protocol.CompletionList, error) {
		return &protocol.CompletionList{
			Items: []protocol.CompletionItem{
				{Label: "foo"},
				{Label: "bar"},
			},
		}, nil
	})

	c := gossiptest.NewClient(t, s)
	c.Open("file:///test.txt", "")

	result, err := c.Completion("file:///test.txt", gossiptest.Pos(0, 0))
	if err != nil {
		t.Fatalf("completion error: %v", err)
	}
	gossiptest.AssertCompletionContains(t, result, "foo")
	gossiptest.AssertCompletionContains(t, result, "bar")
}
