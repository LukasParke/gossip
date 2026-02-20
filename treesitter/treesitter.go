// Package treesitter provides native tree-sitter integration for the gossip LSP framework.
// It ties a parser-per-document lifecycle to the document store, with automatic
// incremental re-parsing on edits and query helpers.
package treesitter

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/gossip-lsp/gossip/protocol"
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
// strategies. At least one of Extensions, Filenames, Pattern, or LanguageID
// must be set.
type LanguageMatcher struct {
	Language   *tree_sitter.Language
	Extensions []string // e.g., [".yml", ".yaml"]
	Filenames  []string // exact filenames, e.g., ["Dockerfile", "Makefile"]
	Pattern    string   // glob pattern, e.g., ".github/workflows/*.yml"
	LanguageID string   // LSP languageId, e.g., "yaml"
}

// Tree wraps a tree-sitter Tree with convenience methods for LSP use.
type Tree struct {
	raw  *tree_sitter.Tree
	src  []byte
	Diff *TreeDiff
}

// TreeDiff describes the structural difference between the previous and current
// parse trees. It is computed automatically on every edit and set on the Tree.
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

// RootNode returns the root node of the parse tree.
func (t *Tree) RootNode() *tree_sitter.Node {
	if t == nil || t.raw == nil {
		return nil
	}
	return t.raw.RootNode()
}

// Close releases the tree-sitter tree resources.
func (t *Tree) Close() {
	if t != nil && t.raw != nil {
		t.raw.Close()
	}
}
