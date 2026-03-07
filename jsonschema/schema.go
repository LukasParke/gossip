// Package jsonschema provides a tree-sitter-aware JSON Schema validator.
// It validates YAML/JSON documents against JSON Schema definitions,
// producing LSP diagnostics with exact source positions.
//
// This package is a generic JSON Schema validator -- it has no knowledge
// of OpenAPI or any specific schema domain.
package jsonschema

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// CompiledSchema is a fully resolved JSON Schema ready for validation.
// All internal $ref pointers are resolved at load time.
type CompiledSchema struct {
	Root *SchemaNode
	Defs map[string]*SchemaNode
}

// SchemaNode represents a single node in a JSON Schema tree.
type SchemaNode struct {
	Type        string                 `json:"type,omitempty"`
	Properties  map[string]*SchemaNode `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Enum        []any                  `json:"enum,omitempty"`
	Const       any                    `json:"const,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Default     any                    `json:"default,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty"`
	Examples    []any                  `json:"examples,omitempty"`

	// Numeric constraints
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`

	// String constraints
	MinLength *int `json:"minLength,omitempty"`
	MaxLength *int `json:"maxLength,omitempty"`

	// Array constraints
	Items    *SchemaNode `json:"items,omitempty"`
	MinItems *int        `json:"minItems,omitempty"`
	MaxItems *int        `json:"maxItems,omitempty"`

	// Object constraints
	AdditionalProperties        *SchemaNode            `json:"-"`
	AdditionalPropertiesBanned  bool                   `json:"-"`
	AdditionalPropertiesAllowed bool                   `json:"-"`
	PatternProperties           map[string]*SchemaNode `json:"patternProperties,omitempty"`

	// Composition
	OneOf []*SchemaNode `json:"oneOf,omitempty"`
	AnyOf []*SchemaNode `json:"anyOf,omitempty"`
	AllOf []*SchemaNode `json:"allOf,omitempty"`
	Not   *SchemaNode   `json:"not,omitempty"`

	// Conditional
	If   *SchemaNode `json:"if,omitempty"`
	Then *SchemaNode `json:"then,omitempty"`
	Else *SchemaNode `json:"else,omitempty"`

	// Reference (resolved during load)
	Ref string `json:"$ref,omitempty"`

	// Internal: compiled pattern regex
	patternRe *regexp.Regexp
}

// rawSchema mirrors JSON Schema structure for unmarshalling, including $ref
// and additionalProperties which need special handling.
type rawSchema struct {
	SchemaNode
	Ref                  string           `json:"$ref,omitempty"`
	Defs                 map[string]*rawSchema `json:"$defs,omitempty"`
	Definitions          map[string]*rawSchema `json:"definitions,omitempty"`
	AdditionalProperties json.RawMessage  `json:"additionalProperties,omitempty"`
	Properties           map[string]*rawSchema `json:"properties,omitempty"`
	Items                *rawSchema       `json:"items,omitempty"`
	OneOf                []*rawSchema     `json:"oneOf,omitempty"`
	AnyOf                []*rawSchema     `json:"anyOf,omitempty"`
	AllOf                []*rawSchema     `json:"allOf,omitempty"`
	Not                  *rawSchema       `json:"not,omitempty"`
	If                   *rawSchema       `json:"if,omitempty"`
	Then                 *rawSchema       `json:"then,omitempty"`
	Else                 *rawSchema       `json:"else,omitempty"`
	PatternProperties    map[string]*rawSchema `json:"patternProperties,omitempty"`
}

// Load parses JSON Schema bytes into a CompiledSchema with all $ref pointers resolved.
func Load(data []byte) (*CompiledSchema, error) {
	var raw rawSchema
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("jsonschema: parse error: %w", err)
	}

	defs := make(map[string]*SchemaNode)

	// First pass: convert all $defs to SchemaNode (unresolved)
	rawDefs := raw.Defs
	if rawDefs == nil {
		rawDefs = raw.Definitions
	}
	for name, def := range rawDefs {
		defs[name] = convertRaw(def, nil)
	}

	// Second pass: resolve $ref pointers in defs using DFS. Each def is
	// resolved at most once, and dependencies are resolved before the
	// referencing node copies their content. Circular references are
	// handled by marking defs "in progress" and using the current
	// (partially resolved) state — the shallow copy shares pointers,
	// so later resolution of the cycle target is visible.
	state := make(map[string]resolveState, len(defs))
	for name := range defs {
		resolveDef(name, defs, state)
	}

	root := convertRaw(&raw, nil)
	resolveRefs(root, defs, state)

	return &CompiledSchema{Root: root, Defs: defs}, nil
}

// LoadFile reads and parses a JSON Schema file.
func LoadFile(path string) (*CompiledSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jsonschema: read file: %w", err)
	}
	return Load(data)
}

// MustLoad parses JSON Schema bytes, panicking on error.
func MustLoad(data []byte) *CompiledSchema {
	s, err := Load(data)
	if err != nil {
		panic(err)
	}
	return s
}

func convertRaw(raw *rawSchema, defs map[string]*SchemaNode) *SchemaNode {
	if raw == nil {
		return nil
	}

	node := &SchemaNode{
		Type:             raw.SchemaNode.Type,
		Required:         raw.SchemaNode.Required,
		Enum:             raw.SchemaNode.Enum,
		Const:            raw.SchemaNode.Const,
		Title:            raw.SchemaNode.Title,
		Description:      raw.SchemaNode.Description,
		Format:           raw.SchemaNode.Format,
		Pattern:          raw.SchemaNode.Pattern,
		Default:          raw.SchemaNode.Default,
		Deprecated:       raw.SchemaNode.Deprecated,
		Examples:         raw.SchemaNode.Examples,
		Minimum:          raw.SchemaNode.Minimum,
		Maximum:          raw.SchemaNode.Maximum,
		ExclusiveMinimum: raw.SchemaNode.ExclusiveMinimum,
		ExclusiveMaximum: raw.SchemaNode.ExclusiveMaximum,
		MultipleOf:       raw.SchemaNode.MultipleOf,
		MinLength:        raw.SchemaNode.MinLength,
		MaxLength:        raw.SchemaNode.MaxLength,
		MinItems:         raw.SchemaNode.MinItems,
		MaxItems:         raw.SchemaNode.MaxItems,
		Ref:              raw.Ref,
	}

	if raw.Pattern != "" {
		if re, err := regexp.Compile(raw.Pattern); err == nil {
			node.patternRe = re
		}
	}

	if raw.Properties != nil {
		node.Properties = make(map[string]*SchemaNode, len(raw.Properties))
		for k, v := range raw.Properties {
			node.Properties[k] = convertRaw(v, defs)
		}
	}

	if raw.PatternProperties != nil {
		node.PatternProperties = make(map[string]*SchemaNode, len(raw.PatternProperties))
		for k, v := range raw.PatternProperties {
			node.PatternProperties[k] = convertRaw(v, defs)
		}
	}

	// Handle additionalProperties: can be boolean or schema
	if len(raw.AdditionalProperties) > 0 {
		var boolVal bool
		if err := json.Unmarshal(raw.AdditionalProperties, &boolVal); err == nil {
			if boolVal {
				node.AdditionalPropertiesAllowed = true
			} else {
				node.AdditionalPropertiesBanned = true
			}
		} else {
			var apRaw rawSchema
			if err := json.Unmarshal(raw.AdditionalProperties, &apRaw); err == nil {
				node.AdditionalProperties = convertRaw(&apRaw, defs)
			}
		}
	}

	node.Items = convertRaw(raw.Items, defs)
	node.Not = convertRaw(raw.Not, defs)
	node.If = convertRaw(raw.If, defs)
	node.Then = convertRaw(raw.Then, defs)
	node.Else = convertRaw(raw.Else, defs)

	for _, s := range raw.OneOf {
		node.OneOf = append(node.OneOf, convertRaw(s, defs))
	}
	for _, s := range raw.AnyOf {
		node.AnyOf = append(node.AnyOf, convertRaw(s, defs))
	}
	for _, s := range raw.AllOf {
		node.AllOf = append(node.AllOf, convertRaw(s, defs))
	}

	return node
}

type resolveState int

const (
	unresolved resolveState = iota
	inProgress
	resolved
)

// resolveDef ensures the named def is fully resolved. It resolves
// dependencies first (DFS), handling cycles by allowing partial state.
func resolveDef(name string, defs map[string]*SchemaNode, state map[string]resolveState) {
	switch state[name] {
	case resolved, inProgress:
		return
	}
	state[name] = inProgress
	node := defs[name]
	if node != nil {
		resolveRefs(node, defs, state)
	}
	state[name] = resolved
}

func resolveRefs(node *SchemaNode, defs map[string]*SchemaNode, state map[string]resolveState) {
	if node == nil {
		return
	}

	if node.Ref != "" {
		name := refToDefName(node.Ref)
		if _, ok := defs[name]; ok {
			// Ensure the target def is resolved before copying.
			resolveDef(name, defs, state)
			*node = *defs[name]
			return
		}
	}

	for _, prop := range node.Properties {
		resolveRefs(prop, defs, state)
	}
	for _, pp := range node.PatternProperties {
		resolveRefs(pp, defs, state)
	}
	resolveRefs(node.AdditionalProperties, defs, state)
	resolveRefs(node.Items, defs, state)
	resolveRefs(node.Not, defs, state)
	resolveRefs(node.If, defs, state)
	resolveRefs(node.Then, defs, state)
	resolveRefs(node.Else, defs, state)
	for _, s := range node.OneOf {
		resolveRefs(s, defs, state)
	}
	for _, s := range node.AnyOf {
		resolveRefs(s, defs, state)
	}
	for _, s := range node.AllOf {
		resolveRefs(s, defs, state)
	}
}

// refToDefName extracts the definition name from a $ref pointer.
// e.g. "#/$defs/__schema0" -> "__schema0"
func refToDefName(ref string) string {
	const defsPrefix = "#/$defs/"
	const definitionsPrefix = "#/definitions/"
	if len(ref) > len(defsPrefix) && ref[:len(defsPrefix)] == defsPrefix {
		return ref[len(defsPrefix):]
	}
	if len(ref) > len(definitionsPrefix) && ref[:len(definitionsPrefix)] == definitionsPrefix {
		return ref[len(definitionsPrefix):]
	}
	return ref
}
