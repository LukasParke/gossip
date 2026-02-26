package treesitter

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// FirstChildOfKind returns the first named child of node whose Kind()
// matches kind, or nil if no such child exists.
func FirstChildOfKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(uint(i))
		if child != nil && child.Kind() == kind {
			return child
		}
	}
	return nil
}

// ChildrenOfKind returns all named children of node whose Kind() matches kind.
func ChildrenOfKind(node *tree_sitter.Node, kind string) []*tree_sitter.Node {
	if node == nil {
		return nil
	}
	count := int(node.NamedChildCount())
	var result []*tree_sitter.Node
	for i := 0; i < count; i++ {
		child := node.NamedChild(uint(i))
		if child != nil && child.Kind() == kind {
			result = append(result, child)
		}
	}
	return result
}

// FirstChildOfKindAll is like FirstChildOfKind but searches all children
// (named and anonymous).
func FirstChildOfKindAll(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child != nil && child.Kind() == kind {
			return child
		}
	}
	return nil
}

// WalkNamedChildren iterates over all named children of node, calling fn for
// each one. Iteration stops early if fn returns false.
func WalkNamedChildren(node *tree_sitter.Node, fn func(child *tree_sitter.Node) bool) {
	if node == nil {
		return
	}
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(uint(i))
		if child != nil && !fn(child) {
			return
		}
	}
}

// WalkChildren iterates over all children (named and anonymous) of node,
// calling fn for each one. Iteration stops early if fn returns false.
func WalkChildren(node *tree_sitter.Node, fn func(child *tree_sitter.Node) bool) {
	if node == nil {
		return
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child != nil && !fn(child) {
			return
		}
	}
}

// Ancestors returns the chain of parent nodes from node up to (but not
// including) the root. The first element is the immediate parent.
func Ancestors(node *tree_sitter.Node) []*tree_sitter.Node {
	if node == nil {
		return nil
	}
	var result []*tree_sitter.Node
	for p := node.Parent(); p != nil; p = p.Parent() {
		result = append(result, p)
	}
	return result
}

// FindAncestor walks up from node and returns the first ancestor whose Kind()
// matches kind, or nil if none is found.
func FindAncestor(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == kind {
			return p
		}
	}
	return nil
}

// HasError reports whether any node in the subtree rooted at node is an ERROR
// or is missing.
func HasError(node *tree_sitter.Node) bool {
	if node == nil {
		return false
	}
	return node.HasError()
}
