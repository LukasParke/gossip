package document

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/LukasParke/gossip/protocol"
)

// OffsetAt converts an LSP Position (line, UTF-16 character offset) to a byte
// offset in the document text. Returns -1 if the position is out of range.
func OffsetAt(text string, pos protocol.Position) int {
	line := int(pos.Line)
	char := int(pos.Character)

	offset := 0
	for l := 0; l < line; l++ {
		nl := strings.IndexByte(text[offset:], '\n')
		if nl < 0 {
			return len(text)
		}
		offset += nl + 1
	}

	lineStart := offset
	nl := strings.IndexByte(text[lineStart:], '\n')
	var lineText string
	if nl < 0 {
		lineText = text[lineStart:]
	} else {
		lineText = text[lineStart : lineStart+nl]
	}

	return lineStart + utf16OffsetToBytes(lineText, char)
}

// PositionAt converts a byte offset to an LSP Position.
func PositionAt(text string, offset int) protocol.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}

	line := uint32(0)
	lineStart := 0
	for i := 0; i < offset; i++ {
		if text[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}

	lineText := text[lineStart:offset]
	char := bytesToUTF16Offset(lineText)

	return protocol.Position{Line: line, Character: uint32(char)}
}

// utf16OffsetToBytes converts a UTF-16 character offset within a line to a byte offset.
func utf16OffsetToBytes(line string, utf16Offset int) int {
	u16 := 0
	byteOffset := 0
	for byteOffset < len(line) && u16 < utf16Offset {
		r, size := utf8.DecodeRuneInString(line[byteOffset:])
		if r == utf8.RuneError && size == 1 {
			u16++
			byteOffset++
			continue
		}
		u16len := utf16.RuneLen(r)
		if u16len < 0 {
			u16len = 1
		}
		u16 += u16len
		byteOffset += size
	}
	return byteOffset
}

// bytesToUTF16Offset converts a byte-length string to its UTF-16 length.
func bytesToUTF16Offset(s string) int {
	u16 := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
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

// LineAt returns the text of the given line (0-indexed), without trailing newline.
func LineAt(text string, line uint32) string {
	l := int(line)
	offset := 0
	for i := 0; i < l; i++ {
		nl := strings.IndexByte(text[offset:], '\n')
		if nl < 0 {
			return ""
		}
		offset += nl + 1
	}
	end := strings.IndexByte(text[offset:], '\n')
	if end < 0 {
		return text[offset:]
	}
	return text[offset : offset+end]
}

// WordAt returns the word at the given position. A "word" is delimited by
// whitespace, punctuation, or document boundaries.
func WordAt(text string, pos protocol.Position) string {
	offset := OffsetAt(text, pos)
	if offset < 0 || offset >= len(text) {
		return ""
	}

	start := offset
	for start > 0 && isWordChar(text[start-1]) {
		start--
	}
	end := offset
	for end < len(text) && isWordChar(text[end]) {
		end++
	}
	return text[start:end]
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
