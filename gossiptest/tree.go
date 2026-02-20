package gossiptest

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseString parses source code with the given tree-sitter language and
// returns the parse tree. It fails the test if parsing produces an error tree.
func ParseString(t testing.TB, lang *tree_sitter.Language, src string) *tree_sitter.Tree {
	t.Helper()
	parser := tree_sitter.NewParser()
	t.Cleanup(func() { parser.Close() })

	if err := parser.SetLanguage(lang); err != nil {
		t.Fatalf("setting tree-sitter language: %v", err)
	}

	tree := parser.Parse([]byte(src), nil)
	t.Cleanup(func() { tree.Close() })
	return tree
}

// AssertNodeKind asserts that a tree-sitter node has the expected kind.
func AssertNodeKind(t testing.TB, node *tree_sitter.Node, kind string) {
	t.Helper()
	if node == nil {
		t.Fatalf("node is nil, expected kind %q", kind)
	}
	if node.Kind() != kind {
		t.Errorf("node kind = %q, want %q", node.Kind(), kind)
	}
}

// AssertNoErrors asserts that the parse tree contains no ERROR nodes.
func AssertNoErrors(t testing.TB, tree *tree_sitter.Tree) {
	t.Helper()
	if tree == nil {
		t.Fatal("tree is nil")
	}
	root := tree.RootNode()
	if root.HasError() {
		t.Error("parse tree contains errors")
	}
}
