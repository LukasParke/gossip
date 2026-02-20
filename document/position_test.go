package document

import (
	"testing"

	"github.com/gossip-lsp/gossip/protocol"
)

func TestOffsetAt(t *testing.T) {
	text := "hello\nworld\nfoo"
	tests := []struct {
		pos  protocol.Position
		want int
	}{
		{protocol.Position{Line: 0, Character: 0}, 0},
		{protocol.Position{Line: 0, Character: 5}, 5},
		{protocol.Position{Line: 1, Character: 0}, 6},
		{protocol.Position{Line: 1, Character: 5}, 11},
		{protocol.Position{Line: 2, Character: 0}, 12},
		{protocol.Position{Line: 2, Character: 3}, 15},
	}
	for _, tt := range tests {
		got := OffsetAt(text, tt.pos)
		if got != tt.want {
			t.Errorf("OffsetAt(%v) = %d, want %d", tt.pos, got, tt.want)
		}
	}
}

func TestPositionAt(t *testing.T) {
	text := "hello\nworld\nfoo"
	tests := []struct {
		offset int
		want   protocol.Position
	}{
		{0, protocol.Position{Line: 0, Character: 0}},
		{5, protocol.Position{Line: 0, Character: 5}},
		{6, protocol.Position{Line: 1, Character: 0}},
		{11, protocol.Position{Line: 1, Character: 5}},
		{12, protocol.Position{Line: 2, Character: 0}},
	}
	for _, tt := range tests {
		got := PositionAt(text, tt.offset)
		if got != tt.want {
			t.Errorf("PositionAt(%d) = %v, want %v", tt.offset, got, tt.want)
		}
	}
}

func TestUTF16Handling(t *testing.T) {
	// '😀' is U+1F600, encoded as a surrogate pair (2 UTF-16 code units)
	text := "a😀b"
	// UTF-16 offsets: a=0, 😀=1-2, b=3
	offset := OffsetAt(text, protocol.Position{Line: 0, Character: 3})
	if text[offset] != 'b' {
		t.Errorf("expected 'b' at UTF-16 offset 3, got %q (byte offset %d)", text[offset], offset)
	}
}

func TestWordAt(t *testing.T) {
	text := "hello world foo_bar"
	tests := []struct {
		pos  protocol.Position
		want string
	}{
		{protocol.Position{Line: 0, Character: 2}, "hello"},
		{protocol.Position{Line: 0, Character: 8}, "world"},
		{protocol.Position{Line: 0, Character: 15}, "foo_bar"},
	}
	for _, tt := range tests {
		got := WordAt(text, tt.pos)
		if got != tt.want {
			t.Errorf("WordAt(%v) = %q, want %q", tt.pos, got, tt.want)
		}
	}
}

func TestApplyChanges(t *testing.T) {
	text := "hello world"
	changes := []protocol.TextDocumentContentChangeEvent{
		{
			Range: &protocol.Range{
				Start: protocol.Position{Line: 0, Character: 6},
				End:   protocol.Position{Line: 0, Character: 11},
			},
			Text: "gossip",
		},
	}
	got := ApplyChanges(text, changes)
	want := "hello gossip"
	if got != want {
		t.Errorf("ApplyChanges = %q, want %q", got, want)
	}
}
