package treesitter

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/protocol"
)

// NodeAt returns the most specific (deepest) node at the given LSP position.
func (t *Tree) NodeAt(pos protocol.Position) *tree_sitter.Node {
	if t == nil || t.raw == nil {
		return nil
	}
	root := t.raw.RootNode()
	point := tree_sitter.Point{Row: uint(pos.Line), Column: uint(pos.Character)}
	node := root.DescendantForPointRange(point, point)
	return node
}

// NamedNodeAt returns the most specific named node at the given position.
func (t *Tree) NamedNodeAt(pos protocol.Position) *tree_sitter.Node {
	if t == nil || t.raw == nil {
		return nil
	}
	root := t.raw.RootNode()
	point := tree_sitter.Point{Row: uint(pos.Line), Column: uint(pos.Character)}
	node := root.NamedDescendantForPointRange(point, point)
	return node
}

// NodeText returns the text content of a node using the stored source.
func (t *Tree) NodeText(node *tree_sitter.Node) string {
	if t == nil || node == nil || t.src == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if int(start) >= len(t.src) || int(end) > len(t.src) {
		return ""
	}
	return string(t.src[start:end])
}

// QueryCaptures runs a tree-sitter query pattern against the tree and returns
// all captures as (capture-name, node) pairs.
func (t *Tree) QueryCaptures(lang *tree_sitter.Language, pattern string) ([]Capture, error) {
	if t == nil || t.raw == nil {
		return nil, nil
	}

	query, err := tree_sitter.NewQuery(lang, pattern)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	captureNames := query.CaptureNames()
	matches := cursor.Matches(query, t.raw.RootNode(), t.src)
	var captures []Capture
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		for _, cap := range match.Captures {
			name := ""
			if int(cap.Index) < len(captureNames) {
				name = captureNames[cap.Index]
			}
			captures = append(captures, Capture{
				Name: name,
				Node: &cap.Node,
				Text: t.NodeText(&cap.Node),
			})
		}
	}
	return captures, nil
}

// QueryCapturesInRanges runs a tree-sitter query pattern restricted to the
// given LSP ranges. Each range is queried independently and the results are
// concatenated. This is the primary mechanism for incremental analysis --
// pass tree.Diff.ChangedRanges to only scan the structurally changed regions.
func (t *Tree) QueryCapturesInRanges(lang *tree_sitter.Language, pattern string, ranges []protocol.Range) ([]Capture, error) {
	if t == nil || t.raw == nil || len(ranges) == 0 {
		return nil, nil
	}

	query, err := tree_sitter.NewQuery(lang, pattern)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	captureNames := query.CaptureNames()
	var captures []Capture

	for _, r := range ranges {
		cursor := tree_sitter.NewQueryCursor()
		cursor.SetPointRange(
			tree_sitter.Point{Row: uint(r.Start.Line), Column: uint(r.Start.Character)},
			tree_sitter.Point{Row: uint(r.End.Line), Column: uint(r.End.Character)},
		)
		matches := cursor.Matches(query, t.raw.RootNode(), t.src)
		for {
			match := matches.Next()
			if match == nil {
				break
			}
			for _, cap := range match.Captures {
				name := ""
				if int(cap.Index) < len(captureNames) {
					name = captureNames[cap.Index]
				}
				captures = append(captures, Capture{
					Name: name,
					Node: &cap.Node,
					Text: t.NodeText(&cap.Node),
				})
			}
		}
		cursor.Close()
	}

	return captures, nil
}

// ErrorsInRanges returns all ERROR nodes within the given LSP ranges.
// This is a convenience wrapper around QueryCapturesInRanges with the
// standard "(ERROR) @error" pattern.
func (t *Tree) ErrorsInRanges(lang *tree_sitter.Language, ranges []protocol.Range) ([]Capture, error) {
	return t.QueryCapturesInRanges(lang, "(ERROR) @error", ranges)
}

// Capture represents a single tree-sitter query capture.
type Capture struct {
	Name string
	Node *tree_sitter.Node
	Text string
}

// NodeRange converts a tree-sitter node's range to an LSP Range.
func NodeRange(node *tree_sitter.Node) protocol.Range {
	if node == nil {
		return protocol.Range{}
	}
	start := node.StartPosition()
	end := node.EndPosition()
	return protocol.Range{
		Start: protocol.Position{Line: uint32(start.Row), Character: uint32(start.Column)},
		End:   protocol.Position{Line: uint32(end.Row), Character: uint32(end.Column)},
	}
}
