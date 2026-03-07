package treesitter

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestByteColumnPoint(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		byteOffset int
		wantRow    uint
		wantCol    uint
	}{
		{
			name:       "start of file",
			src:        "hello\nworld\n",
			byteOffset: 0,
			wantRow:    0,
			wantCol:    0,
		},
		{
			name:       "middle of first line",
			src:        "hello\nworld\n",
			byteOffset: 3,
			wantRow:    0,
			wantCol:    3,
		},
		{
			name:       "start of second line",
			src:        "hello\nworld\n",
			byteOffset: 6,
			wantRow:    1,
			wantCol:    0,
		},
		{
			name:       "middle of second line",
			src:        "hello\nworld\n",
			byteOffset: 9,
			wantRow:    1,
			wantCol:    3,
		},
		{
			name:       "non-ASCII multibyte char",
			src:        "café\nbar",
			byteOffset: 6, // after "café\n" (c=1, a=1, f=1, é=2, \n=1 = 6)
			wantRow:    1,
			wantCol:    0,
		},
		{
			name:       "offset past end clamped",
			src:        "abc",
			byteOffset: 100,
			wantRow:    0,
			wantCol:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := byteColumnPoint(tt.byteOffset, []byte(tt.src))
			want := tree_sitter.Point{Row: tt.wantRow, Column: tt.wantCol}
			if got != want {
				t.Errorf("byteColumnPoint(%d, %q) = {Row:%d, Col:%d}, want {Row:%d, Col:%d}",
					tt.byteOffset, tt.src, got.Row, got.Column, want.Row, want.Column)
			}
		})
	}
}
