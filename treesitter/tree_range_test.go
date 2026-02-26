package treesitter_test

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

type treeEnv struct {
	mgr   *treesitter.Manager
	store *document.Store
	tree  *treesitter.Tree
}

func (e *treeEnv) Close() {
	e.mgr.Close()
}

func makeTreeEnv(t *testing.T, src string) *treeEnv {
	t.Helper()

	cfg := treesitter.Config{
		Languages: map[string]*tree_sitter.Language{".json": jsonLang()},
	}
	store := document.NewStore()
	mgr := treesitter.NewManager(cfg, store)

	uri := protocol.DocumentURI("file:///test.json")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri,
			LanguageID: "json",
			Version:    1,
			Text:       src,
		},
	})

	tree := mgr.GetTree(uri)
	if tree == nil {
		mgr.Close()
		t.Fatal("expected tree after open")
	}
	return &treeEnv{mgr: mgr, store: store, tree: tree}
}

func TestTreeSource(t *testing.T) {
	src := `{"key": "value"}`
	env := makeTreeEnv(t, src)
	defer env.Close()

	if string(env.tree.Source()) != src {
		t.Errorf("Source() = %q, want %q", env.tree.Source(), src)
	}
}

func TestTreeEncoder(t *testing.T) {
	env := makeTreeEnv(t, `{"key": "value"}`)
	defer env.Close()

	enc := env.tree.Encoder()
	if enc == nil {
		t.Fatal("Encoder() returned nil")
	}
}

func TestTreeNodeRangeASCII(t *testing.T) {
	env := makeTreeEnv(t, `{"key": "value"}`)
	defer env.Close()

	root := env.tree.RootNode()
	if root == nil {
		t.Fatal("nil root")
	}

	r := env.tree.NodeRange(root)
	if r.Start.Line != 0 || r.Start.Character != 0 {
		t.Errorf("root start = %+v, want 0:0", r.Start)
	}
	if r.End.Character != 16 {
		t.Errorf("root end char = %d, want 16", r.End.Character)
	}
}

func TestTreeNodeRangeUTF8(t *testing.T) {
	// "café" key: c(1) a(1) f(1) é(2 bytes) — 5 bytes total, 4 UTF-16 units
	src := `{"café": "val"}`
	env := makeTreeEnv(t, src)
	defer env.Close()

	root := env.tree.RootNode()
	obj := root.NamedChild(0) // object
	pair := obj.NamedChild(0) // first pair
	key := pair.NamedChild(0) // string key including quotes

	treeRange := env.tree.NodeRange(key)
	freeRange := treesitter.NodeRange(key)

	// Tree.NodeRange should give UTF-16 correct positions
	// The key starts at byte 1 (after {) and the tree-sitter node includes quotes
	// Free NodeRange uses raw byte columns — they should differ when non-ASCII is involved
	// But the key "café" starts at col 1 which is ASCII, so start columns should match
	if treeRange.Start.Character != freeRange.Start.Character {
		t.Logf("note: start chars differ (tree=%d, free=%d) — expected for this position",
			treeRange.Start.Character, freeRange.Start.Character)
	}

	// {"café": "val"}
	// byte 0: {
	// byte 1: "  ← key node start
	// byte 2: c
	// byte 3: a
	// byte 4: f
	// byte 5-6: é (2 bytes)
	// byte 7: "  ← end of key string
	// key range in bytes: col 1..8 (exclusive end)
	// UTF-16: 1..7 (since é is 1 UTF-16 unit, not 2 bytes)

	if treeRange.End.Character == freeRange.End.Character {
		t.Errorf("expected Tree.NodeRange and NodeRange to differ for non-ASCII content, both gave %d",
			treeRange.End.Character)
	}

	if treeRange.End.Character != 7 {
		t.Errorf("Tree.NodeRange end char = %d, want 7 (UTF-16)", treeRange.End.Character)
	}
}

func TestTreeChildFieldRange(t *testing.T) {
	env := makeTreeEnv(t, `{"key": "value"}`)
	defer env.Close()

	root := env.tree.RootNode()
	obj := root.NamedChild(0)
	pair := obj.NamedChild(0)

	keyRange := env.tree.ChildFieldRange(pair, "key")
	if keyRange.IsZero() {
		t.Error("key field range should not be zero")
	}

	noRange := env.tree.ChildFieldRange(pair, "nonexistent")
	if !noRange.IsZero() {
		t.Errorf("nonexistent field range should be zero, got %+v", noRange)
	}
}
