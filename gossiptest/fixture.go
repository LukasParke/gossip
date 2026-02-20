package gossiptest

import (
	"fmt"
	"strings"

	"github.com/LukasParke/gossip/protocol"
)

// FileURI creates a file:// URI from a path.
func FileURI(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("file://%s", path)
}

// Pos creates a protocol.Position from line and character (0-indexed).
func Pos(line, char uint32) protocol.Position {
	return protocol.Position{Line: line, Character: char}
}

// Rng creates a protocol.Range from start and end positions.
func Rng(startLine, startChar, endLine, endChar uint32) protocol.Range {
	return protocol.Range{
		Start: Pos(startLine, startChar),
		End:   Pos(endLine, endChar),
	}
}
