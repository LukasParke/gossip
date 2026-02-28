package jsonschema

import (
	"fmt"
	"strings"
)

// CompletionItemKind mirrors the LSP CompletionItemKind to avoid a
// circular dependency on the protocol package.
type CompletionItemKind int

const (
	KindProperty   CompletionItemKind = 10
	KindModule     CompletionItemKind = 9
	KindVariable   CompletionItemKind = 6
	KindEnumMember CompletionItemKind = 20
	KindValue      CompletionItemKind = 12
)

// CompletionItem represents a single completion suggestion derived from a schema.
type CompletionItem struct {
	Label         string
	Detail        string
	Documentation string
	Kind          CompletionItemKind
	InsertText    string
	Required      bool
}

// SchemaCompletions returns completion items for valid properties at the given
// JSON Pointer path within the schema. The path segments identify the current
// cursor position within the document structure (e.g., ["paths", "/users", "get"]).
func SchemaCompletions(schema *CompiledSchema, pathSegments []string) []CompletionItem {
	if schema == nil || schema.Root == nil {
		return nil
	}

	node := resolvePathInSchema(schema.Root, pathSegments, schema.Defs)
	if node == nil {
		return nil
	}

	return completionsFromNode(node)
}

func completionsFromNode(node *SchemaNode) []CompletionItem {
	if node == nil {
		return nil
	}

	var items []CompletionItem

	requiredSet := make(map[string]bool, len(node.Required))
	for _, r := range node.Required {
		requiredSet[r] = true
	}

	for name, prop := range node.Properties {
		item := CompletionItem{
			Label:    name,
			Kind:     schemaCompletionKind(prop),
			Required: requiredSet[name],
		}
		if prop.Description != "" {
			item.Documentation = prop.Description
		}
		if prop.Type != "" {
			item.Detail = prop.Type
			if prop.Format != "" {
				item.Detail += " (" + prop.Format + ")"
			}
		}
		item.InsertText = buildInsertText(name, prop)
		items = append(items, item)
	}

	return items
}

// SchemaValueCompletions returns completion items for valid values of a
// property at the given path.
func SchemaValueCompletions(schema *CompiledSchema, pathSegments []string) []CompletionItem {
	if schema == nil || schema.Root == nil {
		return nil
	}

	node := resolvePathInSchema(schema.Root, pathSegments, schema.Defs)
	if node == nil {
		return nil
	}

	var items []CompletionItem

	if len(node.Enum) > 0 {
		for _, v := range node.Enum {
			s := fmt.Sprintf("%v", v)
			items = append(items, CompletionItem{
				Label:  s,
				Kind:   KindValue,
				Detail: "enum value",
			})
		}
	}

	if node.Const != nil {
		s := fmt.Sprintf("%v", node.Const)
		items = append(items, CompletionItem{
			Label:  s,
			Kind:   KindValue,
			Detail: "const value",
		})
	}

	if node.Type == "boolean" {
		items = append(items,
			CompletionItem{Label: "true", Kind: KindValue, Detail: "boolean"},
			CompletionItem{Label: "false", Kind: KindValue, Detail: "boolean"},
		)
	}

	return items
}

func resolvePathInSchema(node *SchemaNode, path []string, defs map[string]*SchemaNode) *SchemaNode {
	if node == nil {
		return nil
	}

	// Resolve refs
	if node.Ref != "" && defs != nil {
		defName := refToDefName(node.Ref)
		if resolved, ok := defs[defName]; ok {
			node = resolved
		}
	}

	if len(path) == 0 {
		return node
	}

	segment := path[0]
	rest := path[1:]

	if prop, ok := node.Properties[segment]; ok {
		return resolvePathInSchema(prop, rest, defs)
	}

	if node.Items != nil {
		return resolvePathInSchema(node.Items, rest, defs)
	}

	// Try anyOf/oneOf
	for _, sub := range node.AnyOf {
		if result := resolvePathInSchema(sub, path, defs); result != nil {
			return result
		}
	}
	for _, sub := range node.OneOf {
		if result := resolvePathInSchema(sub, path, defs); result != nil {
			return result
		}
	}

	if node.AdditionalProperties != nil {
		return resolvePathInSchema(node.AdditionalProperties, rest, defs)
	}

	return nil
}

func schemaCompletionKind(node *SchemaNode) CompletionItemKind {
	if node == nil {
		return KindProperty
	}
	switch node.Type {
	case "object":
		return KindModule
	case "array":
		return KindVariable
	case "string":
		if len(node.Enum) > 0 {
			return KindEnumMember
		}
		return KindProperty
	case "boolean":
		return KindValue
	case "number", "integer":
		return KindValue
	default:
		return KindProperty
	}
}

func buildInsertText(name string, node *SchemaNode) string {
	if node == nil {
		return name + ": "
	}
	switch node.Type {
	case "object":
		return name + ":\n  "
	case "array":
		return name + ":\n  - "
	case "boolean":
		return name + ": "
	default:
		if len(node.Enum) == 1 {
			return name + ": " + fmt.Sprintf("%v", node.Enum[0])
		}
		return name + ": "
	}
}

// DescribeSchema returns a markdown description of a schema node at the
// given path, suitable for hover display.
func DescribeSchema(schema *CompiledSchema, pathSegments []string) string {
	if schema == nil || schema.Root == nil {
		return ""
	}

	node := resolvePathInSchema(schema.Root, pathSegments, schema.Defs)
	if node == nil {
		return ""
	}

	return describeNode(pathSegments, node)
}

func describeNode(path []string, node *SchemaNode) string {
	var sb strings.Builder

	name := "root"
	if len(path) > 0 {
		name = path[len(path)-1]
	}

	sb.WriteString("**" + name + "**")

	if node.Type != "" {
		sb.WriteString(" `" + node.Type)
		if node.Format != "" {
			sb.WriteString(" (" + node.Format + ")")
		}
		sb.WriteString("`")
	}

	if node.Deprecated {
		sb.WriteString(" *(deprecated)*")
	}

	sb.WriteString("\n\n")

	if node.Description != "" {
		sb.WriteString(node.Description + "\n\n")
	}

	if len(node.Enum) > 0 {
		sb.WriteString("**Values:** ")
		vals := make([]string, len(node.Enum))
		for i, v := range node.Enum {
			vals[i] = fmt.Sprintf("`%v`", v)
		}
		sb.WriteString(strings.Join(vals, ", ") + "\n\n")
	}

	if node.Minimum != nil {
		sb.WriteString(fmt.Sprintf("**Minimum:** %v\n", *node.Minimum))
	}
	if node.Maximum != nil {
		sb.WriteString(fmt.Sprintf("**Maximum:** %v\n", *node.Maximum))
	}
	if node.MinLength != nil {
		sb.WriteString(fmt.Sprintf("**Min length:** %d\n", *node.MinLength))
	}
	if node.MaxLength != nil {
		sb.WriteString(fmt.Sprintf("**Max length:** %d\n", *node.MaxLength))
	}
	if node.Pattern != "" {
		sb.WriteString(fmt.Sprintf("**Pattern:** `%s`\n", node.Pattern))
	}

	if node.Default != nil {
		sb.WriteString(fmt.Sprintf("**Default:** `%v`\n", node.Default))
	}

	return strings.TrimSpace(sb.String())
}
