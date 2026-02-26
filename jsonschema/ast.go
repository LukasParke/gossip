package jsonschema

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/treesitter"
)

// KeyValuePair represents a key-value pair extracted from a tree-sitter AST node.
type KeyValuePair struct {
	KeyNode   *tree_sitter.Node
	ValueNode *tree_sitter.Node
	KeyText   string
}

// ExtractPairs returns key-value pairs from an object/mapping AST node.
// Supports JSON (object/pair) and YAML (block_mapping/block_mapping_pair,
// flow_mapping/flow_pair) node kinds uniformly.
func ExtractPairs(node *tree_sitter.Node, tree *treesitter.Tree) []KeyValuePair {
	if node == nil {
		return nil
	}

	src := tree.Source()
	kind := node.Kind()

	switch kind {
	case "object":
		return extractJSON(node, src)
	case "block_mapping":
		return extractYAML(node, src, "block_mapping_pair")
	case "flow_mapping":
		return extractYAML(node, src, "flow_pair")
	default:
		return nil
	}
}

func extractJSON(node *tree_sitter.Node, src []byte) []KeyValuePair {
	count := node.ChildCount()
	var pairs []KeyValuePair

	for i := uint(0); i < uint(count); i++ {
		child := node.Child(i)
		if child == nil || child.Kind() != "pair" {
			continue
		}

		keyNode := child.ChildByFieldName("key")
		valueNode := child.ChildByFieldName("value")
		if keyNode == nil {
			continue
		}

		keyText := nodeText(keyNode, src)
		// JSON keys are quoted strings; strip quotes
		if len(keyText) >= 2 && keyText[0] == '"' && keyText[len(keyText)-1] == '"' {
			keyText = keyText[1 : len(keyText)-1]
		}

		pairs = append(pairs, KeyValuePair{
			KeyNode:   keyNode,
			ValueNode: valueNode,
			KeyText:   keyText,
		})
	}
	return pairs
}

func extractYAML(node *tree_sitter.Node, src []byte, pairKind string) []KeyValuePair {
	count := node.ChildCount()
	var pairs []KeyValuePair

	for i := uint(0); i < uint(count); i++ {
		child := node.Child(i)
		if child == nil || child.Kind() != pairKind {
			continue
		}

		keyNode := child.ChildByFieldName("key")
		valueNode := child.ChildByFieldName("value")
		if keyNode == nil {
			continue
		}

		pairs = append(pairs, KeyValuePair{
			KeyNode:   keyNode,
			ValueNode: valueNode,
			KeyText:   nodeText(keyNode, src),
		})
	}
	return pairs
}

func nodeText(node *tree_sitter.Node, src []byte) string {
	start := node.StartByte()
	end := node.EndByte()
	if start >= uint(len(src)) || end > uint(len(src)) || start >= end {
		return ""
	}
	return string(src[start:end])
}

// NodeKindIsObject returns true if the node kind represents a JSON/YAML object.
func NodeKindIsObject(kind string) bool {
	switch kind {
	case "object", "block_mapping", "flow_mapping":
		return true
	}
	return false
}

// NodeKindIsArray returns true if the node kind represents a JSON/YAML array.
func NodeKindIsArray(kind string) bool {
	switch kind {
	case "array", "block_sequence", "flow_sequence":
		return true
	}
	return false
}

// NodeKindIsScalar returns true if the node kind represents a JSON/YAML scalar value.
func NodeKindIsScalar(kind string) bool {
	switch kind {
	case "string", "string_content", "number", "true", "false", "null",
		"string_scalar", "block_scalar", "flow_scalar", "double_quote_scalar",
		"single_quote_scalar", "plain_scalar", "integer_scalar", "float_scalar",
		"boolean_scalar", "null_scalar":
		return true
	}
	return false
}
