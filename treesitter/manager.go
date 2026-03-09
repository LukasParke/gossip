package treesitter

import (
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
)

// TreeUpdateFunc is called after a tree is parsed or re-parsed.
type TreeUpdateFunc func(uri protocol.DocumentURI, tree *Tree)

// Manager manages tree-sitter parsers and trees for all open documents.
// It is tied to a document.Store and automatically parses on open and re-parses
// incrementally on change.
//
// Parsers are created per-document on open and released on close. Each document
// has its own tree-sitter Parser instance. OnTreeUpdate callbacks are invoked
// in registration order after every parse or reparse. Call Close when shutting
// down to release all parsers and trees.
type Manager struct {
	registry *Registry
	store    *document.Store

	mu      sync.RWMutex
	parsers map[protocol.DocumentURI]*tree_sitter.Parser
	trees   map[protocol.DocumentURI]*Tree

	onTreeUpdate []TreeUpdateFunc
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

// OnTreeUpdate registers a callback that fires after every parse or reparse.
// Multiple callbacks can be registered; they are invoked in registration order,
// synchronously within the document store's edit handler.
func (m *Manager) OnTreeUpdate(fn TreeUpdateFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTreeUpdate = append(m.onTreeUpdate, fn)
}

// Registry returns the language registry.
func (m *Manager) Registry() *Registry {
	return m.registry
}

// GetTree returns the current tree for the given document URI.
// The URI is normalized before lookup.
func (m *Manager) GetTree(uri protocol.DocumentURI) *Tree {
	key := protocol.NormalizeURI(uri)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trees[key]
}

// handleOpen is called when a document is opened. It creates a parser and
// performs the initial full parse.
func (m *Manager) handleOpen(doc *document.Document) {
	uri := protocol.NormalizeURI(doc.URI())
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
	callbacks := make([]TreeUpdateFunc, len(m.onTreeUpdate))
	copy(callbacks, m.onTreeUpdate)
	m.mu.Unlock()

	doc.SetTree(wrapped)
	doc.SetOnTreeEdit(func(edits []document.EditRange) {
		m.handleEdits(uri, edits)
	})

	for _, cb := range callbacks {
		cb(uri, wrapped)
	}
}

// handleClose is called when a document is closed. It cleans up the parser and tree.
func (m *Manager) handleClose(uri protocol.DocumentURI) {
	key := protocol.NormalizeURI(uri)
	m.mu.Lock()
	if parser, ok := m.parsers[key]; ok {
		parser.Close()
		delete(m.parsers, key)
	}
	if tree, ok := m.trees[key]; ok {
		tree.Close()
		delete(m.trees, key)
	}
	m.mu.Unlock()
}

// handleEdits performs incremental re-parsing after document edits.
func (m *Manager) handleEdits(uri protocol.DocumentURI, edits []document.EditRange) {
	m.mu.Lock()
	parser, ok := m.parsers[uri]
	if !ok {
		m.mu.Unlock()
		return
	}
	oldTree, ok := m.trees[uri]
	if !ok || oldTree.raw == nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	doc := m.store.Get(uri)
	if doc == nil {
		return
	}

	// Use the old tree's source for converting UTF-16 positions to byte
	// columns for StartPosition and OldEndPosition. The Encoder handles
	// the UTF-16 → byte column conversion that tree-sitter requires.
	oldEnc := NewEncoder(oldTree.src)

	for _, edit := range edits {
		oldTree.raw.Edit(&tree_sitter.InputEdit{
			StartByte:      uint(edit.StartByte),
			OldEndByte:     uint(edit.OldEndByte),
			NewEndByte:     uint(edit.NewEndByte),
			StartPosition:  oldEnc.Point(edit.StartPos),
			OldEndPosition: oldEnc.Point(edit.OldEndPos),
			NewEndPosition: byteColumnPoint(edit.NewEndByte, []byte(doc.Text())),
		})
	}

	src := []byte(doc.Text())
	newTree := parser.Parse(src, oldTree.raw)
	diffEdits := make([]DiffEdit, 0, len(edits))
	for _, edit := range edits {
		diffEdits = append(diffEdits, DiffEdit{
			Start:  edit.StartPos,
			OldEnd: edit.OldEndPos,
			NewEnd: edit.NewEndPos,
		})
	}
	diff := computeTreeDiff(oldTree.raw, newTree, diffEdits)
	oldTree.Close()
	wrapped := &Tree{raw: newTree, src: src, Diff: diff}

	m.mu.Lock()
	m.trees[uri] = wrapped
	callbacks := make([]TreeUpdateFunc, len(m.onTreeUpdate))
	copy(callbacks, m.onTreeUpdate)
	m.mu.Unlock()

	doc.SetTree(wrapped)

	for _, cb := range callbacks {
		cb(uri, wrapped)
	}
}

// byteColumnPoint computes a tree-sitter Point (row + byte column) from a
// byte offset in the given source text. This avoids the UTF-16 conversion
// issue when building Points for the new text coordinate space.
func byteColumnPoint(byteOffset int, src []byte) tree_sitter.Point {
	if byteOffset > len(src) {
		byteOffset = len(src)
	}
	row := uint(0)
	lastNewline := -1
	for i := 0; i < byteOffset; i++ {
		if src[i] == '\n' {
			row++
			lastNewline = i
		}
	}
	col := uint(byteOffset - lastNewline - 1)
	return tree_sitter.Point{Row: row, Column: col}
}

// computeTreeDiff builds a TreeDiff from the old (edited) and new (reparsed) trees.
func computeTreeDiff(oldRaw, newRaw *tree_sitter.Tree, edits []DiffEdit) *TreeDiff {
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
		Edits:         edits,
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

// Close releases all parsers and trees. Call this during server shutdown to
// avoid leaking resources. After Close, the Manager must not be used.
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
