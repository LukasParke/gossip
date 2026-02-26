// Package treesitter provides native tree-sitter integration for the gossip LSP framework.
// It ties a parser-per-document lifecycle to the document store, with automatic
// incremental re-parsing on edits and query helpers.
package treesitter

import (
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/protocol"
)

// Config configures the tree-sitter integration.
type Config struct {
	// Languages maps file extensions (e.g., ".go", ".rs") to tree-sitter languages.
	// This is the simplest way to register languages and is backward compatible.
	Languages map[string]*tree_sitter.Language

	// Matchers provides advanced file-to-language matching beyond extensions.
	// Matchers are evaluated in order; the first match wins.
	Matchers []LanguageMatcher
}

// LanguageMatcher associates a tree-sitter language with one or more matching
// strategies. Evaluation order: Filenames (exact), LanguageID, Pattern (glob),
// Extensions. At least one of Extensions, Filenames, Pattern, or LanguageID
// should be set for the matcher to be useful.
type LanguageMatcher struct {
	Language   *tree_sitter.Language
	Extensions []string // e.g., [".yml", ".yaml"]
	Filenames  []string // exact filenames, e.g., ["Dockerfile", "Makefile"]
	Pattern    string   // glob pattern, e.g., ".github/workflows/*.yml"
	LanguageID string   // LSP languageId, e.g., "yaml"
}

// Tree wraps a tree-sitter Tree with convenience methods for LSP use.
//
// Thread-safety: A Tree is valid and safe to use from the moment it is passed
// to a TreeUpdateFunc callback until the next edit or document close. After
// an edit, the Manager replaces the tree with a fresh one; the old tree is
// closed and must not be accessed. Callers must not retain Tree references
// across tree-update callbacks without re-fetching via Manager.GetTree.
type Tree struct {
	raw  *tree_sitter.Tree
	src  []byte
	Diff *TreeDiff

	encOnce sync.Once
	enc     *Encoder
}

// NewTree creates a Tree from a parsed tree-sitter tree and its source bytes.
// This is useful for testing; in normal operation, the Manager creates Trees.
func NewTree(raw *tree_sitter.Tree, src []byte) *Tree {
	return &Tree{raw: raw, src: src}
}

// TreeDiff describes the structural difference between the previous and current
// parse trees. It is computed automatically on every edit and set on the Tree.
//
// Lifetime: Diff is valid for the same duration as the containing Tree —
// until the next edit or document close.
type TreeDiff struct {
	// ChangedRanges are the LSP ranges where the syntax tree structurally changed.
	ChangedRanges []protocol.Range

	// AffectedKinds is the set of node kinds that appear in the changed subtrees.
	AffectedKinds map[string]bool

	// AffectedNodes are the top-level named nodes whose subtrees contain changes.
	AffectedNodes []*tree_sitter.Node

	// IsFullReparse is true on initial open or full-text replacement.
	IsFullReparse bool
}

// AffectsKind reports whether the diff touches any node of the given kind.
// The kind string should match tree-sitter node kinds (e.g., "function_declaration",
// "ERROR"). Returns false if d is nil. Use this to short-circuit analyzers when
// the edit does not affect nodes they care about.
func (d *TreeDiff) AffectsKind(kind string) bool {
	if d == nil {
		return false
	}
	return d.AffectedKinds[kind]
}

// Raw returns the underlying tree-sitter Tree.
func (t *Tree) Raw() *tree_sitter.Tree {
	if t == nil {
		return nil
	}
	return t.raw
}

// Source returns the source bytes that were used to parse this tree.
// The returned slice must not be modified.
func (t *Tree) Source() []byte {
	if t == nil {
		return nil
	}
	return t.src
}

// RootNode returns the root node of the parse tree.
func (t *Tree) RootNode() *tree_sitter.Node {
	if t == nil || t.raw == nil {
		return nil
	}
	return t.raw.RootNode()
}

// Encoder returns the cached Encoder for this tree's source text that correctly
// converts between tree-sitter byte columns and LSP UTF-16 character positions.
// The Encoder is created lazily on first call and cached for the tree's lifetime.
// Safe for concurrent use.
func (t *Tree) Encoder() *Encoder {
	if t == nil {
		return NewEncoder(nil)
	}
	t.encOnce.Do(func() {
		t.enc = NewEncoder(t.src)
	})
	return t.enc
}

// NodeRange converts a tree-sitter node to an LSP Range using document-aware
// UTF-16 encoding. Unlike the free function NodeRange, this correctly handles
// non-ASCII source text.
func (t *Tree) NodeRange(node *tree_sitter.Node) protocol.Range {
	return t.Encoder().NodeRange(node)
}

// ChildFieldRange returns the LSP Range of a named field child within node,
// using document-aware UTF-16 encoding. Returns a zero Range if the field
// does not exist.
func (t *Tree) ChildFieldRange(node *tree_sitter.Node, field string) protocol.Range {
	return t.Encoder().ChildFieldRange(node, field)
}

// Close releases the tree-sitter tree resources.
func (t *Tree) Close() {
	if t != nil && t.raw != nil {
		t.raw.Close()
	}
}
