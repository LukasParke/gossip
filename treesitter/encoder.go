package treesitter

import (
	"unicode/utf16"
	"unicode/utf8"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/protocol"
)

// Encoder converts tree-sitter byte-column positions into LSP UTF-16
// character positions. Tree-sitter reports column offsets as byte counts
// within a line, while LSP (by default) uses UTF-16 code unit counts.
// For pure ASCII sources, byte and UTF-16 columns are identical and
// the fast-path NodeRange free function can be used instead. Encoder
// is required for correctness when source text may contain non-ASCII
// characters (e.g., accented letters, emoji, CJK).
//
// An Encoder is cheap to construct: it splits the source into lines once
// and does per-line byte→UTF-16 conversion on demand.
type Encoder struct {
	lines [][]byte
}

// NewEncoder creates an Encoder for the given source text.
func NewEncoder(src []byte) *Encoder {
	return &Encoder{lines: splitLines(src)}
}

// Position converts a tree-sitter Point (row + byte column) into an LSP
// Position (line + UTF-16 character offset).
func (e *Encoder) Position(p tree_sitter.Point) protocol.Position {
	row := uint32(p.Row)
	byteCol := int(p.Column)

	if int(row) >= len(e.lines) {
		if len(e.lines) == 0 {
			return protocol.Position{Line: row, Character: uint32(byteCol)}
		}
		lastLine := uint32(len(e.lines) - 1)
		return protocol.Position{
			Line:      lastLine,
			Character: uint32(bytesToUTF16(e.lines[lastLine])),
		}
	}

	line := e.lines[row]
	if byteCol > len(line) {
		byteCol = len(line)
	}

	return protocol.Position{
		Line:      row,
		Character: uint32(bytesToUTF16(line[:byteCol])),
	}
}

// NodeRange converts a tree-sitter node's positions to an LSP Range with
// correct UTF-16 character offsets. This is the document-aware equivalent
// of the free function NodeRange.
func (e *Encoder) NodeRange(node *tree_sitter.Node) protocol.Range {
	if node == nil {
		return protocol.Range{}
	}
	return protocol.Range{
		Start: e.Position(node.StartPosition()),
		End:   e.Position(node.EndPosition()),
	}
}

// ChildFieldRange returns the LSP Range for the named field child of node.
// Returns a zero Range if the field does not exist.
func (e *Encoder) ChildFieldRange(node *tree_sitter.Node, field string) protocol.Range {
	child := node.ChildByFieldName(field)
	if child == nil {
		return protocol.Range{}
	}
	return e.NodeRange(child)
}

// PointRange returns a zero-width LSP Range at the start of the given node.
func (e *Encoder) PointRange(node *tree_sitter.Node) protocol.Range {
	if node == nil {
		return protocol.Range{}
	}
	pos := e.Position(node.StartPosition())
	return protocol.PointRange(pos)
}

// Point converts an LSP Position (line + UTF-16 character offset) back to a
// tree-sitter Point (row + byte column). This is the inverse of Position.
func (e *Encoder) Point(pos protocol.Position) tree_sitter.Point {
	row := int(pos.Line)
	utf16Col := int(pos.Character)

	if row >= len(e.lines) {
		return tree_sitter.Point{Row: uint(pos.Line), Column: uint(utf16Col)}
	}

	line := e.lines[row]
	return tree_sitter.Point{
		Row:    uint(pos.Line),
		Column: uint(utf16ToBytes(line, utf16Col)),
	}
}

// utf16ToBytes converts a UTF-16 code unit offset within a line to a byte offset.
func utf16ToBytes(line []byte, utf16Offset int) int {
	u16 := 0
	byteOff := 0
	for byteOff < len(line) && u16 < utf16Offset {
		r, size := decodeRune(line[byteOff:])
		u16len := utf16.RuneLen(r)
		if u16len < 0 {
			u16len = 1
		}
		u16 += u16len
		byteOff += size
	}
	return byteOff
}

// decodeRune wraps utf8.DecodeRune with RuneError handling.
func decodeRune(b []byte) (rune, int) {
	r, size := utf8.DecodeRune(b)
	if r == utf8.RuneError && size == 1 {
		return r, 1
	}
	return r, size
}

// bytesToUTF16 counts the number of UTF-16 code units needed to represent
// the given byte slice (assumed valid UTF-8).
func bytesToUTF16(b []byte) int {
	u16 := 0
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			u16++
			i++
			continue
		}
		u16len := utf16.RuneLen(r)
		if u16len < 0 {
			u16len = 1
		}
		u16 += u16len
		i += size
	}
	return u16
}

// splitLines splits src into lines, preserving the byte content of each line
// but stripping the trailing newline. Handles \n, \r\n, and \r.
func splitLines(src []byte) [][]byte {
	if len(src) == 0 {
		return [][]byte{{}}
	}

	var lines [][]byte
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			lines = append(lines, src[start:i])
			start = i + 1
		} else if src[i] == '\r' {
			lines = append(lines, src[start:i])
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	lines = append(lines, src[start:])
	return lines
}
