package treesitter_test

import (
	"testing"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"

	"github.com/LukasParke/gossip/treesitter"
)

func parseJSON(src string) (*tree_sitter.Tree, *tree_sitter.Language) {
	lang := tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_json.Language()))
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)
	return parser.Parse([]byte(src), nil), lang
}

func TestFirstChildOfKind(t *testing.T) {
	// {"a": 1, "b": "hello"}
	tree, _ := parseJSON(`{"a": 1, "b": "hello"}`)
	defer tree.Close()

	root := tree.RootNode()
	obj := root.NamedChild(0) // object node

	// object has "pair" children
	pair := treesitter.FirstChildOfKind(obj, "pair")
	if pair == nil {
		t.Fatal("expected to find a 'pair' child")
	}
	if pair.Kind() != "pair" {
		t.Errorf("kind = %s, want pair", pair.Kind())
	}

	// no "array" child
	arr := treesitter.FirstChildOfKind(obj, "array")
	if arr != nil {
		t.Error("should not find 'array' child in object")
	}

	// nil safety
	if treesitter.FirstChildOfKind(nil, "pair") != nil {
		t.Error("nil node should return nil")
	}
}

func TestChildrenOfKind(t *testing.T) {
	tree, _ := parseJSON(`{"a": 1, "b": 2, "c": 3}`)
	defer tree.Close()

	root := tree.RootNode()
	obj := root.NamedChild(0)

	pairs := treesitter.ChildrenOfKind(obj, "pair")
	if len(pairs) != 3 {
		t.Errorf("got %d pairs, want 3", len(pairs))
	}

	if treesitter.ChildrenOfKind(nil, "pair") != nil {
		t.Error("nil node should return nil")
	}
}

func TestFirstChildOfKindAll(t *testing.T) {
	// This searches all children including anonymous nodes (brackets, commas)
	tree, _ := parseJSON(`{"key": "value"}`)
	defer tree.Close()

	root := tree.RootNode()
	obj := root.NamedChild(0)

	// "{" is an anonymous child
	brace := treesitter.FirstChildOfKindAll(obj, "{")
	if brace == nil {
		t.Fatal("expected to find '{' anonymous child")
	}

	if treesitter.FirstChildOfKindAll(nil, "{") != nil {
		t.Error("nil node should return nil")
	}
}

func TestWalkNamedChildren(t *testing.T) {
	tree, _ := parseJSON(`[1, 2, 3]`)
	defer tree.Close()

	root := tree.RootNode()
	arr := root.NamedChild(0)

	var kinds []string
	treesitter.WalkNamedChildren(arr, func(child *tree_sitter.Node) bool {
		kinds = append(kinds, child.Kind())
		return true
	})
	if len(kinds) != 3 {
		t.Errorf("walked %d named children, want 3", len(kinds))
	}

	// early stop
	count := 0
	treesitter.WalkNamedChildren(arr, func(child *tree_sitter.Node) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("early stop: walked %d, want 1", count)
	}
}

func TestWalkChildren(t *testing.T) {
	tree, _ := parseJSON(`[1, 2]`)
	defer tree.Close()

	root := tree.RootNode()
	arr := root.NamedChild(0)

	count := 0
	treesitter.WalkChildren(arr, func(child *tree_sitter.Node) bool {
		count++
		return true
	})
	// [, 1, ",", 2, ] — includes anonymous nodes
	if count < 3 {
		t.Errorf("expected at least 3 children (including anonymous), got %d", count)
	}
}

func TestFindAncestor(t *testing.T) {
	tree, _ := parseJSON(`{"key": "value"}`)
	defer tree.Close()

	root := tree.RootNode()
	obj := root.NamedChild(0)
	pair := obj.NamedChild(0)
	key := pair.NamedChild(0)

	found := treesitter.FindAncestor(key, "pair")
	if found == nil {
		t.Fatal("expected to find 'pair' ancestor")
	}
	if found.Kind() != "pair" {
		t.Errorf("ancestor kind = %s, want pair", found.Kind())
	}

	missing := treesitter.FindAncestor(key, "array")
	if missing != nil {
		t.Error("should not find 'array' ancestor")
	}

	if treesitter.FindAncestor(nil, "pair") != nil {
		t.Error("nil node should return nil")
	}
}

func TestAncestors(t *testing.T) {
	tree, _ := parseJSON(`{"key": "value"}`)
	defer tree.Close()

	root := tree.RootNode()
	obj := root.NamedChild(0)
	pair := obj.NamedChild(0)
	key := pair.NamedChild(0)

	anc := treesitter.Ancestors(key)
	if len(anc) < 2 {
		t.Fatalf("expected at least 2 ancestors, got %d", len(anc))
	}
	// first ancestor should be the pair
	if anc[0].Kind() != "pair" {
		t.Errorf("first ancestor = %s, want pair", anc[0].Kind())
	}

	if treesitter.Ancestors(nil) != nil {
		t.Error("nil node should return nil")
	}
}

func TestHasError(t *testing.T) {
	// Valid JSON
	tree, _ := parseJSON(`{"key": "value"}`)
	defer tree.Close()
	if treesitter.HasError(tree.RootNode()) {
		t.Error("valid JSON should not have errors")
	}

	// Invalid JSON
	tree2, _ := parseJSON(`{"key": }`)
	defer tree2.Close()
	if !treesitter.HasError(tree2.RootNode()) {
		t.Error("invalid JSON should have errors")
	}

	if treesitter.HasError(nil) {
		t.Error("nil should return false")
	}
}
