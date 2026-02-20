package treesitter

import (
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/gossip-lsp/gossip/document"
	"github.com/gossip-lsp/gossip/protocol"
)

// TreeUpdateFunc is called after a tree is parsed or re-parsed.
type TreeUpdateFunc func(uri protocol.DocumentURI, tree *Tree)

// Manager manages tree-sitter parsers and trees for all open documents.
// It is tied to a document.Store and automatically parses on open and re-parses
// incrementally on change.
type Manager struct {
	registry *Registry
	store    *document.Store

	mu      sync.RWMutex
	parsers map[protocol.DocumentURI]*tree_sitter.Parser
	trees   map[protocol.DocumentURI]*Tree

	onTreeUpdate TreeUpdateFunc
}

// NewManager creates a new tree-sitter manager tied to a document store.
func NewManager(cfg Config, store *document.Store) *Manager {
	m := &Manager{
		registry: NewRegistry(cfg),
		store:    store,
		parsers:  make(map[protocol.DocumentURI]*tree_sitter.Parser),
		trees:    make(map[protocol.DocumentURI]*Tree),
	}

	store.OnOpen(m.handleOpen)
	store.OnClose(m.handleClose)

	return m
}

// OnTreeUpdate registers a callback that fires after every parse/reparse.
func (m *Manager) OnTreeUpdate(fn TreeUpdateFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTreeUpdate = fn
}

// Registry returns the language registry.
func (m *Manager) Registry() *Registry {
	return m.registry
}

// GetTree returns the current tree for the given document URI.
func (m *Manager) GetTree(uri protocol.DocumentURI) *Tree {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trees[uri]
}

// handleOpen is called when a document is opened. It creates a parser and
// performs the initial full parse.
func (m *Manager) handleOpen(doc *document.Document) {
	uri := doc.URI()
	lang, err := m.registry.LanguageForURI(string(uri), doc.LanguageID())
	if err != nil {
		return
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(lang); err != nil {
		parser.Close()
		return
	}

	src := []byte(doc.Text())
	tree := parser.Parse(src, nil)

	diff := &TreeDiff{IsFullReparse: true, AffectedKinds: make(map[string]bool)}
	collectAllKinds(tree.RootNode(), diff.AffectedKinds)

	wrapped := &Tree{raw: tree, src: src, Diff: diff}

	m.mu.Lock()
	m.parsers[uri] = parser
	m.trees[uri] = wrapped
	cb := m.onTreeUpdate
	m.mu.Unlock()

	doc.SetTree(wrapped)
	doc.SetOnTreeEdit(func(edits []document.EditRange) {
		m.handleEdits(uri, edits)
	})

	if cb != nil {
		cb(uri, wrapped)
	}
}

// handleClose is called when a document is closed. It cleans up the parser and tree.
func (m *Manager) handleClose(uri protocol.DocumentURI) {
	m.mu.Lock()
	if parser, ok := m.parsers[uri]; ok {
		parser.Close()
		delete(m.parsers, uri)
	}
	if tree, ok := m.trees[uri]; ok {
		tree.Close()
		delete(m.trees, uri)
	}
	m.mu.Unlock()
}

// handleEdits performs incremental re-parsing after document edits.
func (m *Manager) handleEdits(uri protocol.DocumentURI, edits []document.EditRange) {
	m.mu.Lock()
	defer m.mu.Unlock()

	parser, ok := m.parsers[uri]
	if !ok {
		return
	}
	oldTree, ok := m.trees[uri]
	if !ok || oldTree.raw == nil {
		return
	}

	doc := m.store.Get(uri)
	if doc == nil {
		return
	}

	for _, edit := range edits {
		oldTree.raw.Edit(&tree_sitter.InputEdit{
			StartByte:  uint(edit.StartByte),
			OldEndByte: uint(edit.OldEndByte),
			NewEndByte: uint(edit.NewEndByte),
			StartPosition: tree_sitter.Point{
				Row:    uint(edit.StartPos.Line),
				Column: uint(edit.StartPos.Character),
			},
			OldEndPosition: tree_sitter.Point{
				Row:    uint(edit.OldEndPos.Line),
				Column: uint(edit.OldEndPos.Character),
			},
			NewEndPosition: tree_sitter.Point{
				Row:    uint(edit.NewEndPos.Line),
				Column: uint(edit.NewEndPos.Character),
			},
		})
	}

	src := []byte(doc.Text())
	newTree := parser.Parse(src, oldTree.raw)

	diff := computeTreeDiff(oldTree.raw, newTree)

	oldTree.Close()
	wrapped := &Tree{raw: newTree, src: src, Diff: diff}
	m.trees[uri] = wrapped

	doc.SetTree(wrapped)

	cb := m.onTreeUpdate
	if cb != nil {
		cb(uri, wrapped)
	}
}

// computeTreeDiff builds a TreeDiff from the old (edited) and new (reparsed) trees.
func computeTreeDiff(oldRaw, newRaw *tree_sitter.Tree) *TreeDiff {
	tsRanges := oldRaw.ChangedRanges(newRaw)

	lspRanges := make([]protocol.Range, len(tsRanges))
	for i, r := range tsRanges {
		lspRanges[i] = protocol.Range{
			Start: protocol.Position{Line: uint32(r.StartPoint.Row), Character: uint32(r.StartPoint.Column)},
			End:   protocol.Position{Line: uint32(r.EndPoint.Row), Character: uint32(r.EndPoint.Column)},
		}
	}

	diff := &TreeDiff{
		ChangedRanges: lspRanges,
		AffectedKinds: make(map[string]bool),
	}

	root := newRaw.RootNode()
	if root == nil {
		return diff
	}

	seen := make(map[uintptr]bool)
	for _, r := range tsRanges {
		startPoint := tree_sitter.Point{Row: r.StartPoint.Row, Column: r.StartPoint.Column}
		endPoint := tree_sitter.Point{Row: r.EndPoint.Row, Column: r.EndPoint.Column}
		node := root.NamedDescendantForPointRange(startPoint, endPoint)
		if node == nil {
			continue
		}

		collectSubtreeKinds(node, diff.AffectedKinds)

		ancestor := findScopeAncestor(node)
		id := ancestor.Id()
		if !seen[id] {
			seen[id] = true
			diff.AffectedNodes = append(diff.AffectedNodes, ancestor)
		}
	}

	return diff
}

// findScopeAncestor walks up the tree to find the nearest "scope boundary" --
// the highest named parent that isn't the root.
func findScopeAncestor(node *tree_sitter.Node) *tree_sitter.Node {
	best := node
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.Parent() == nil {
			break
		}
		if p.IsNamed() {
			best = p
		}
	}
	return best
}

// collectSubtreeKinds recursively collects all node kinds in a subtree.
func collectSubtreeKinds(node *tree_sitter.Node, kinds map[string]bool) {
	if node == nil {
		return
	}
	kinds[node.Kind()] = true
	for i := 0; i < int(node.ChildCount()); i++ {
		collectSubtreeKinds(node.Child(uint(i)), kinds)
	}
}

// collectAllKinds initialises the kind set with every kind in the tree.
// Used for the initial full-parse diff.
func collectAllKinds(root *tree_sitter.Node, kinds map[string]bool) {
	if kinds == nil {
		return
	}
	if root == nil {
		return
	}
	collectSubtreeKinds(root, kinds)
}

// Close releases all parsers and trees.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for uri, parser := range m.parsers {
		parser.Close()
		delete(m.parsers, uri)
	}
	for uri, tree := range m.trees {
		tree.Close()
		delete(m.trees, uri)
	}
}
