package treesitter

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/protocol"
)

func TestEncoderASCII(t *testing.T) {
	src := []byte("hello world\nsecond line\n")
	enc := NewEncoder(src)

	tests := []struct {
		name string
		p    tree_sitter.Point
		want protocol.Position
	}{
		{"origin", tree_sitter.Point{Row: 0, Column: 0}, protocol.Position{Line: 0, Character: 0}},
		{"mid first line", tree_sitter.Point{Row: 0, Column: 5}, protocol.Position{Line: 0, Character: 5}},
		{"start second line", tree_sitter.Point{Row: 1, Column: 0}, protocol.Position{Line: 1, Character: 0}},
		{"mid second line", tree_sitter.Point{Row: 1, Column: 7}, protocol.Position{Line: 1, Character: 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enc.Position(tt.p)
			if got != tt.want {
				t.Errorf("Position(%+v) = %+v, want %+v", tt.p, got, tt.want)
			}
		})
	}
}

func TestEncoderUTF8TwoByteChars(t *testing.T) {
	// "café" = c(1) a(1) f(1) é(2 bytes UTF-8, 1 UTF-16 code unit) = 5 bytes, 4 UTF-16 units
	src := []byte("café\n")
	enc := NewEncoder(src)

	tests := []struct {
		name     string
		byteCol  uint
		wantChar uint32
	}{
		{"before c", 0, 0},
		{"before a", 1, 1},
		{"before f", 2, 2},
		{"before é (byte 3)", 3, 3},
		{"after é (byte 5)", 5, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enc.Position(tree_sitter.Point{Row: 0, Column: tt.byteCol})
			if got.Character != tt.wantChar {
				t.Errorf("byte col %d → UTF-16 char %d, want %d", tt.byteCol, got.Character, tt.wantChar)
			}
		})
	}
}

func TestEncoderUTF8FourByteChars(t *testing.T) {
	// "a😀b" = a(1 byte, 1 u16) 😀(4 bytes, 2 u16) b(1 byte, 1 u16)
	// byte offsets: a=0, 😀=1..4, b=5
	// UTF-16 units: a=0, 😀=1..2, b=3
	src := []byte("a😀b\n")
	enc := NewEncoder(src)

	tests := []struct {
		name     string
		byteCol  uint
		wantChar uint32
	}{
		{"before a", 0, 0},
		{"before emoji", 1, 1},
		{"after emoji", 5, 3},
		{"after b", 6, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enc.Position(tree_sitter.Point{Row: 0, Column: tt.byteCol})
			if got.Character != tt.wantChar {
				t.Errorf("byte col %d → UTF-16 char %d, want %d", tt.byteCol, got.Character, tt.wantChar)
			}
		})
	}
}

func TestEncoderMultipleLines(t *testing.T) {
	// Line 0: "José" (4 chars, 5 bytes: J o s é(2))
	// Line 1: "naïve" (5 chars, 6 bytes: n a ï(2) v e)
	src := []byte("José\nnaïve\n")
	enc := NewEncoder(src)

	// End of "José": byte col 5, UTF-16 col 4
	got := enc.Position(tree_sitter.Point{Row: 0, Column: 5})
	if got.Character != 4 {
		t.Errorf("end of José: got char %d, want 4", got.Character)
	}

	// End of "naïve": byte col 6, UTF-16 col 5
	got = enc.Position(tree_sitter.Point{Row: 1, Column: 6})
	if got.Character != 5 {
		t.Errorf("end of naïve: got char %d, want 5", got.Character)
	}
}

func TestEncoderEmptySource(t *testing.T) {
	enc := NewEncoder(nil)
	got := enc.Position(tree_sitter.Point{Row: 0, Column: 0})
	if got.Line != 0 || got.Character != 0 {
		t.Errorf("nil source: got %+v, want 0:0", got)
	}

	enc = NewEncoder([]byte{})
	got = enc.Position(tree_sitter.Point{Row: 0, Column: 0})
	if got.Line != 0 || got.Character != 0 {
		t.Errorf("empty source: got %+v, want 0:0", got)
	}
}

func TestEncoderOutOfBoundsRow(t *testing.T) {
	src := []byte("hello\n")
	enc := NewEncoder(src)

	// Row 5 doesn't exist — should clamp to last line
	got := enc.Position(tree_sitter.Point{Row: 5, Column: 0})
	if got.Line != 1 {
		t.Errorf("out of bounds row: got line %d, want 1", got.Line)
	}
}

func TestEncoderOutOfBoundsColumn(t *testing.T) {
	src := []byte("hi\n")
	enc := NewEncoder(src)

	// Column 100 on a 2-char line should clamp
	got := enc.Position(tree_sitter.Point{Row: 0, Column: 100})
	if got.Character != 2 {
		t.Errorf("out of bounds col: got char %d, want 2", got.Character)
	}
}

func TestEncoderNodeRange(t *testing.T) {
	enc := NewEncoder([]byte("café\n"))
	// Nil node returns zero range
	r := enc.NodeRange(nil)
	if !r.IsZero() {
		t.Errorf("nil node: got %+v, want zero", r)
	}
}

func TestEncoderPointRange(t *testing.T) {
	enc := NewEncoder([]byte("test\n"))
	r := enc.PointRange(nil)
	if !r.IsZero() {
		t.Errorf("nil node: got %+v, want zero", r)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // number of lines
	}{
		{"empty", "", 1},
		{"no newline", "hello", 1},
		{"one newline", "hello\n", 2},
		{"two lines", "hello\nworld", 2},
		{"crlf", "hello\r\nworld", 2},
		{"cr only", "hello\rworld", 2},
		{"trailing newline", "a\nb\n", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines([]byte(tt.input))
			if len(lines) != tt.want {
				t.Errorf("splitLines(%q) got %d lines, want %d", tt.input, len(lines), tt.want)
			}
		})
	}
}

func TestEncoderPointRoundTrip(t *testing.T) {
	// Round-trip at character boundaries: byte col → UTF-16 → byte col
	// "a😀café" layout:
	//   a: byte 0 (1 byte, 1 u16)
	//   😀: bytes 1-4 (4 bytes, 2 u16)
	//   c: byte 5 (1 byte, 1 u16)
	//   a: byte 6 (1 byte, 1 u16)
	//   f: byte 7 (1 byte, 1 u16)
	//   é: bytes 8-9 (2 bytes, 1 u16)
	//   end: byte 10
	src := []byte("a😀café\n")
	enc := NewEncoder(src)

	// Only test at valid UTF-8 character start positions
	charBoundaries := []int{0, 1, 5, 6, 7, 8, 10}
	for _, byteCol := range charBoundaries {
		pos := enc.Position(tree_sitter.Point{Row: 0, Column: uint(byteCol)})
		point := enc.Point(pos)
		if point.Column != uint(byteCol) {
			t.Errorf("round-trip byte col %d → UTF-16 %d → byte col %d",
				byteCol, pos.Character, point.Column)
		}
	}
}

func TestEncoderPointUTF8(t *testing.T) {
	// "café" = c(1) a(1) f(1) é(2) = 5 bytes, 4 UTF-16 units
	src := []byte("café\n")
	enc := NewEncoder(src)

	tests := []struct {
		name      string
		utf16Char uint32
		wantByte  uint
	}{
		{"before c", 0, 0},
		{"before a", 1, 1},
		{"before f", 2, 2},
		{"before é", 3, 3},
		{"after é", 4, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := enc.Point(protocol.Position{Line: 0, Character: tt.utf16Char})
			if p.Column != tt.wantByte {
				t.Errorf("UTF-16 char %d → byte col %d, want %d",
					tt.utf16Char, p.Column, tt.wantByte)
			}
		})
	}
}

func TestBytesToUTF16(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"ascii", "hello", 5},
		{"2-byte utf8", "café", 4},
		{"4-byte utf8 (emoji)", "😀", 2},
		{"mixed", "a😀b", 4},
		{"empty", "", 0},
		{"cjk", "日本", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bytesToUTF16([]byte(tt.in))
			if got != tt.want {
				t.Errorf("bytesToUTF16(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
