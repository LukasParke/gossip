package treesitter

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/protocol"
)

// NodeAt returns the most specific (deepest) node at the given LSP position.
// The position's UTF-16 character offset is correctly converted to tree-sitter
// byte columns using the tree's source text.
func (t *Tree) NodeAt(pos protocol.Position) *tree_sitter.Node {
	if t == nil || t.raw == nil {
		return nil
	}
	root := t.raw.RootNode()
	point := t.Encoder().Point(pos)
	node := root.DescendantForPointRange(point, point)
	return node
}

// NamedNodeAt returns the most specific named node at the given position.
// The position's UTF-16 character offset is correctly converted to tree-sitter
// byte columns using the tree's source text.
func (t *Tree) NamedNodeAt(pos protocol.Position) *tree_sitter.Node {
	if t == nil || t.raw == nil {
		return nil
	}
	root := t.raw.RootNode()
	point := t.Encoder().Point(pos)
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
// all captures as (capture-name, node) pairs. Capture.Node is only valid for
// the lifetime of the Tree; do not retain captures after the next edit or close.
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
		captures = t.queryRange(query, captureNames, r, captures)
	}

	return captures, nil
}

func (t *Tree) queryRange(query *tree_sitter.Query, captureNames []string, r protocol.Range, captures []Capture) []Capture {
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	enc := t.Encoder()
	cursor.SetPointRange(enc.Point(r.Start), enc.Point(r.End))
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
	return captures
}

// ErrorsInRanges returns all ERROR nodes within the given LSP ranges.
// This is a convenience wrapper around QueryCapturesInRanges with the
// standard "(ERROR) @error" pattern.
func (t *Tree) ErrorsInRanges(lang *tree_sitter.Language, ranges []protocol.Range) ([]Capture, error) {
	return t.QueryCapturesInRanges(lang, "(ERROR) @error", ranges)
}

// MissingNodes walks the parse tree and returns all nodes where IsMissing()
// is true. Tree-sitter inserts MISSING nodes during error recovery when an
// expected token is absent (e.g., a closing brace). These cannot be found
// via query patterns -- a tree walk is required.
func (t *Tree) MissingNodes() []*tree_sitter.Node {
	if t == nil || t.raw == nil {
		return nil
	}
	var missing []*tree_sitter.Node
	walkMissing(t.raw.RootNode(), &missing)
	return missing
}

func walkMissing(node *tree_sitter.Node, out *[]*tree_sitter.Node) {
	if node == nil {
		return
	}
	if node.IsMissing() {
		*out = append(*out, node)
		return
	}
	for i := uint(0); i < uint(node.ChildCount()); i++ {
		walkMissing(node.Child(i), out)
	}
}

// Capture represents a single tree-sitter query capture from QueryCaptures or
// QueryCapturesInRanges. Node points into the Tree's underlying parse tree and
// is only valid until the Tree is replaced (next edit) or closed.
type Capture struct {
	Name string
	Node *tree_sitter.Node
	Text string
}

// NodeRange converts a tree-sitter node's range to an LSP Range by directly
// casting byte columns to character offsets. This is correct when the source
// text is pure ASCII (the common case for code, YAML keys, JSON keys, etc.).
//
// For sources containing non-ASCII characters (accented text, emoji, CJK),
// byte columns and UTF-16 character offsets diverge. Use Tree.NodeRange or
// Encoder.NodeRange instead for guaranteed correctness.
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

// NodeStartRange returns a zero-width LSP Range at the start position of the
// given node. Uses byte-column fast path; see NodeRange for caveats.
func NodeStartRange(node *tree_sitter.Node) protocol.Range {
	if node == nil {
		return protocol.Range{}
	}
	start := node.StartPosition()
	pos := protocol.Position{Line: uint32(start.Row), Character: uint32(start.Column)}
	return protocol.PointRange(pos)
}

// ChildFieldRange returns the LSP Range of the named field child of node.
// Returns a zero Range if the field does not exist. Uses byte-column fast
// path; see NodeRange for caveats.
func ChildFieldRange(node *tree_sitter.Node, field string) protocol.Range {
	if node == nil {
		return protocol.Range{}
	}
	child := node.ChildByFieldName(field)
	return NodeRange(child)
}
