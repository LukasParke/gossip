package jsonschema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

// ValidateOptions configures the validation behavior.
type ValidateOptions struct {
	Source         string                      // diagnostic source (e.g. "openapi-structure")
	Code           string                      // diagnostic code (e.g. "oas3-schema"); if empty, Source is used
	Severity       protocol.DiagnosticSeverity // default severity
	MaxDiagnostics int                         // cap (0 = unlimited)
}

// ValidationResult holds the output of a validation run.
type ValidationResult struct {
	Diagnostics []protocol.Diagnostic
}

// Validate walks a tree-sitter AST depth-first and validates it against the
// given JSON Schema, producing LSP diagnostics with exact source positions.
func Validate(tree *treesitter.Tree, schema *CompiledSchema, opts ValidateOptions) *ValidationResult {
	if tree == nil || schema == nil || schema.Root == nil {
		return &ValidationResult{}
	}

	v := &validator{
		tree:   tree,
		opts:   opts,
		schema: schema,
	}

	root := tree.RootNode()
	if root == nil {
		return &ValidationResult{}
	}

	// For JSON, the root is "program" → first child is the document value
	// For YAML, the root is "stream" → first child is "document" → first child is the value
	valueNode := findDocumentValue(root)
	if valueNode == nil {
		return &ValidationResult{}
	}

	v.validate(valueNode, schema.Root)

	return &ValidationResult{Diagnostics: v.diags}
}

type validator struct {
	tree   *treesitter.Tree
	opts   ValidateOptions
	schema *CompiledSchema
	diags  []protocol.Diagnostic
}

func (v *validator) addDiag(node *tree_sitter.Node, msg string, data any) {
	if v.opts.MaxDiagnostics > 0 && len(v.diags) >= v.opts.MaxDiagnostics {
		return
	}

	rng := v.tree.NodeRange(node)
	// Constrain multi-line ranges to a single line so diagnostics don't
	// highlight leading whitespace on subsequent lines.
	if rng.End.Line > rng.Start.Line {
		rng.End = protocol.Position{Line: rng.Start.Line, Character: rng.Start.Character + 1000}
	}

	d := protocol.Diagnostic{
		Range:    rng,
		Severity: v.opts.Severity,
		Source:   v.opts.Source,
		Message:  msg,
		Data:     data,
	}
	if v.opts.Code != "" {
		d.Code = v.opts.Code
	} else if v.opts.Source != "" {
		d.Code = v.opts.Source
	}
	v.diags = append(v.diags, d)
}

func (v *validator) validate(node *tree_sitter.Node, schema *SchemaNode) {
	if node == nil || schema == nil {
		return
	}

	// Handle composition keywords first
	if len(schema.AllOf) > 0 {
		for _, sub := range schema.AllOf {
			v.validate(node, sub)
		}
	}
	if len(schema.AnyOf) > 0 {
		v.validateAnyOf(node, schema.AnyOf)
	}
	if len(schema.OneOf) > 0 {
		v.validateOneOf(node, schema.OneOf)
	}
	if schema.Not != nil {
		v.validateNot(node, schema.Not)
	}
	if schema.If != nil {
		v.validateConditional(node, schema)
	}

	kind := node.Kind()

	if NodeKindIsObject(kind) {
		v.validateObject(node, schema)
	} else if NodeKindIsArray(kind) {
		v.validateArray(node, schema)
	} else if NodeKindIsScalar(kind) {
		v.validateScalar(node, schema)
	}
}

func (v *validator) validateObject(node *tree_sitter.Node, schema *SchemaNode) {
	pairs := ExtractPairs(node, v.tree)
	presentKeys := make(map[string]bool, len(pairs))

	for _, pair := range pairs {
		presentKeys[pair.KeyText] = true

		if propSchema, ok := schema.Properties[pair.KeyText]; ok {
			// Known property: validate its value
			if pair.ValueNode != nil {
				valueNode := unwrapValue(pair.ValueNode)
				if valueNode != nil {
					v.validate(valueNode, propSchema)
				}
			}
			continue
		}

		// Check if it's an extension key (x-*)
		if strings.HasPrefix(pair.KeyText, "x-") {
			continue
		}

		// Check pattern properties — validate value against the matching schema
		if ppSchema := matchingPatternProperty(pair.KeyText, schema.PatternProperties); ppSchema != nil {
			if pair.ValueNode != nil {
				valueNode := unwrapValue(pair.ValueNode)
				if valueNode != nil {
					v.validate(valueNode, ppSchema)
				}
			}
			continue
		}

		// additionalProperties as a sub-schema: validate value against it
		if schema.AdditionalProperties != nil {
			if pair.ValueNode != nil {
				valueNode := unwrapValue(pair.ValueNode)
				if valueNode != nil {
					v.validate(valueNode, schema.AdditionalProperties)
				}
			}
			continue
		}

		// additionalProperties: true → allow
		if schema.AdditionalPropertiesAllowed {
			continue
		}

		// additionalProperties not specified and no properties defined → allow
		if !schema.AdditionalPropertiesBanned && len(schema.Properties) == 0 {
			continue
		}

		// Unknown key — report diagnostic
		validKeys := propertyNames(schema)
		suggestion := SuggestKey(pair.KeyText, validKeys)

		var msg string
		var data any
		if suggestion != nil {
			msg = fmt.Sprintf("Unknown property '%s'; did you mean '%s'?", pair.KeyText, suggestion.Suggested)
			data = InvalidKeyData{Kind: "invalid_key", SuggestTo: suggestion.Suggested}
		} else {
			msg = fmt.Sprintf("Unknown property '%s' is not allowed here", pair.KeyText)
			data = InvalidKeyData{Kind: "invalid_key"}
		}
		v.addDiag(pair.KeyNode, msg, data)
	}

	// Check required fields — target the parent key or object opening rather than the whole object
	if len(schema.Required) > 0 {
		target := objectDiagTarget(node)
		for _, req := range schema.Required {
			if !presentKeys[req] {
				v.addDiag(target, fmt.Sprintf("Required property '%s' is missing", req), nil)
			}
		}
	}
}

func (v *validator) validateArray(node *tree_sitter.Node, schema *SchemaNode) {
	count := int(node.ChildCount())

	// Count actual element children (skip punctuation)
	var elements []*tree_sitter.Node
	for i := uint(0); i < uint(count); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		kind := child.Kind()
		// Skip punctuation tokens
		if kind == "[" || kind == "]" || kind == "," || kind == "-" {
			continue
		}
		elements = append(elements, child)
	}

	if schema.MinItems != nil && len(elements) < *schema.MinItems {
		v.addDiag(node, fmt.Sprintf("Array must have at least %d item(s)", *schema.MinItems), nil)
	}
	if schema.MaxItems != nil && len(elements) > *schema.MaxItems {
		v.addDiag(node, fmt.Sprintf("Array must have at most %d item(s)", *schema.MaxItems), nil)
	}

	if schema.Items != nil {
		for _, elem := range elements {
			valueNode := unwrapValue(elem)
			if valueNode != nil {
				v.validate(valueNode, schema.Items)
			}
		}
	}
}

func (v *validator) validateScalar(node *tree_sitter.Node, schema *SchemaNode) {
	text := nodeText(node, v.tree.Source())

	// Type checking
	if schema.Type != "" {
		actualType := inferScalarType(node, text)
		if actualType != "" && actualType != schema.Type {
			// Allow integer where number is expected, and number where integer is expected
			// (JSON has no integer type; tree-sitter reports all numbers as "number")
			numCompat := (schema.Type == "number" && actualType == "integer") ||
				(schema.Type == "integer" && actualType == "number")
			if !numCompat {
				v.addDiag(node, fmt.Sprintf("Expected %s, got %s", schema.Type, actualType), nil)
			}
		}
	}

	// Const checking
	if schema.Const != nil {
		constStr := fmt.Sprintf("%v", schema.Const)
		scalarVal := unquoteScalar(text)
		if scalarVal != constStr {
			v.addDiag(node, fmt.Sprintf("Value must be %s", constStr), nil)
		}
	}

	// Enum checking
	if len(schema.Enum) > 0 {
		scalarVal := unquoteScalar(text)
		found := false
		for _, allowed := range schema.Enum {
			if fmt.Sprintf("%v", allowed) == scalarVal {
				found = true
				break
			}
		}
		if !found {
			enumStrs := make([]string, 0, len(schema.Enum))
			for _, e := range schema.Enum {
				enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
			}
			v.addDiag(node, fmt.Sprintf("Value '%s' is not valid; expected one of: %s",
				scalarVal, strings.Join(enumStrs, ", ")), nil)
		}
	}

	// String constraints
	if schema.Type == "string" || inferScalarType(node, text) == "string" {
		strVal := unquoteScalar(text)
		if schema.MinLength != nil && len(strVal) < *schema.MinLength {
			v.addDiag(node, fmt.Sprintf("Value must be at least %d character(s)", *schema.MinLength), nil)
		}
		if schema.MaxLength != nil && len(strVal) > *schema.MaxLength {
			v.addDiag(node, fmt.Sprintf("Value must be at most %d character(s)", *schema.MaxLength), nil)
		}
		if schema.patternRe != nil && !schema.patternRe.MatchString(strVal) {
			v.addDiag(node, fmt.Sprintf("Value does not match pattern '%s'", schema.Pattern), nil)
		}
	}

	// Numeric constraints
	if schema.Type == "number" || schema.Type == "integer" {
		numVal := unquoteScalar(text)
		if f, err := strconv.ParseFloat(numVal, 64); err == nil {
			if schema.Minimum != nil && f < *schema.Minimum {
				v.addDiag(node, fmt.Sprintf("Value must be >= %g", *schema.Minimum), nil)
			}
			if schema.Maximum != nil && f > *schema.Maximum {
				v.addDiag(node, fmt.Sprintf("Value must be <= %g", *schema.Maximum), nil)
			}
			if schema.ExclusiveMinimum != nil && f <= *schema.ExclusiveMinimum {
				v.addDiag(node, fmt.Sprintf("Value must be > %g", *schema.ExclusiveMinimum), nil)
			}
			if schema.ExclusiveMaximum != nil && f >= *schema.ExclusiveMaximum {
				v.addDiag(node, fmt.Sprintf("Value must be < %g", *schema.ExclusiveMaximum), nil)
			}
			if schema.MultipleOf != nil && *schema.MultipleOf != 0 {
				remainder := f / *schema.MultipleOf
				if remainder != float64(int64(remainder)) {
					v.addDiag(node, fmt.Sprintf("Value must be a multiple of %g", *schema.MultipleOf), nil)
				}
			}
		}
	}
}

func (v *validator) validateAnyOf(node *tree_sitter.Node, schemas []*SchemaNode) {
	var bestDiags []protocol.Diagnostic
	for _, sub := range schemas {
		trial := &validator{
			tree:   v.tree,
			opts:   v.opts,
			schema: v.schema,
		}
		trial.validate(node, sub)
		if len(trial.diags) == 0 {
			return // at least one matches
		}
		// Track the alternative with the fewest errors (closest match).
		if bestDiags == nil || len(trial.diags) < len(bestDiags) {
			bestDiags = trial.diags
		}
	}
	// Report the diagnostics from the closest-matching alternative so the
	// user gets specific, actionable feedback rather than a generic message.
	v.diags = append(v.diags, bestDiags...)
}

func (v *validator) validateOneOf(node *tree_sitter.Node, schemas []*SchemaNode) {
	matchCount := 0
	for _, sub := range schemas {
		trial := &validator{
			tree:   v.tree,
			opts:   v.opts,
			schema: v.schema,
		}
		trial.validate(node, sub)
		if len(trial.diags) == 0 {
			matchCount++
		}
	}
	if matchCount == 0 {
		v.addDiag(node, "Value must match exactly one schema (oneOf), but matched none", nil)
	}
}

func (v *validator) validateNot(node *tree_sitter.Node, notSchema *SchemaNode) {
	trial := &validator{
		tree:   v.tree,
		opts:   v.opts,
		schema: v.schema,
	}
	trial.validate(node, notSchema)
	if len(trial.diags) == 0 {
		v.addDiag(node, "Value must not match the excluded schema", nil)
	}
}

func (v *validator) validateConditional(node *tree_sitter.Node, schema *SchemaNode) {
	trial := &validator{
		tree:   v.tree,
		opts:   v.opts,
		schema: v.schema,
	}
	trial.validate(node, schema.If)

	// Only count constraint/type errors as failures, not unknown-key diagnostics,
	// because the if-schema typically defines a subset of properties.
	realFailures := 0
	for _, d := range trial.diags {
		if _, ok := d.Data.(InvalidKeyData); !ok {
			realFailures++
		}
	}

	if realFailures == 0 {
		if schema.Then != nil {
			v.validate(node, schema.Then)
		}
	} else {
		if schema.Else != nil {
			v.validate(node, schema.Else)
		}
	}
}

// objectDiagTarget returns the best node for diagnosing a missing-property
// error. If the object is the value of a key-value pair, use the key node
// (e.g., "info" for `info: { ... }`). Otherwise fall back to the first named
// child of the object, or the object itself.
func objectDiagTarget(objectNode *tree_sitter.Node) *tree_sitter.Node {
	parent := objectNode.Parent()
	if parent != nil {
		kind := parent.Kind()
		// YAML: block_mapping_pair / flow_pair; JSON: pair
		if kind == "block_mapping_pair" || kind == "flow_pair" || kind == "pair" {
			key := parent.ChildByFieldName("key")
			if key != nil {
				return key
			}
		}
	}
	if objectNode.NamedChildCount() > 0 {
		return objectNode.NamedChild(0)
	}
	return objectNode
}

// --- helpers ---

func findDocumentValue(root *tree_sitter.Node) *tree_sitter.Node {
	if root == nil {
		return nil
	}
	kind := root.Kind()

	// JSON: root is usually the value itself, or wrapped in "program"/"value"
	// YAML: root is "stream" -> "document" -> value
	switch kind {
	case "object", "array", "block_mapping", "block_sequence", "flow_mapping", "flow_sequence":
		return root
	}

	count := root.ChildCount()
	for i := uint(0); i < uint(count); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}
		found := findDocumentValue(child)
		if found != nil {
			return found
		}
	}
	return nil
}

// unwrapValue descends through wrapper nodes (e.g. YAML "block_node")
// to find the actual value node.
func unwrapValue(node *tree_sitter.Node) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	kind := node.Kind()
	if NodeKindIsObject(kind) || NodeKindIsArray(kind) || NodeKindIsScalar(kind) {
		return node
	}
	// Descend through wrapper nodes
	count := node.ChildCount()
	for i := uint(0); i < uint(count); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if result := unwrapValue(child); result != nil {
			return result
		}
	}
	return nil
}

func inferScalarType(node *tree_sitter.Node, text string) string {
	kind := node.Kind()
	switch kind {
	case "string", "string_content", "double_quote_scalar", "single_quote_scalar",
		"string_scalar", "block_scalar":
		return "string"
	case "number", "integer_scalar", "float_scalar":
		return "number"
	case "true", "false", "boolean_scalar":
		return "boolean"
	case "null", "null_scalar":
		return "null"
	case "plain_scalar", "flow_scalar":
		return inferPlainScalarType(text)
	}
	return ""
}

func inferPlainScalarType(text string) string {
	if text == "true" || text == "false" {
		return "boolean"
	}
	if text == "null" || text == "~" {
		return "null"
	}
	if _, err := strconv.ParseFloat(text, 64); err == nil {
		return "number"
	}
	return "string"
}

func unquoteScalar(text string) string {
	if len(text) >= 2 {
		if (text[0] == '"' && text[len(text)-1] == '"') ||
			(text[0] == '\'' && text[len(text)-1] == '\'') {
			return text[1 : len(text)-1]
		}
	}
	return text
}

func propertyNames(schema *SchemaNode) []string {
	if schema == nil || schema.Properties == nil {
		return nil
	}
	names := make([]string, 0, len(schema.Properties))
	for k := range schema.Properties {
		names = append(names, k)
	}
	return names
}

// matchingPatternProperty returns the schema for the first pattern property
// whose regex matches the given key, or nil if no pattern matches.
func matchingPatternProperty(key string, patterns map[string]*SchemaNode) *SchemaNode {
	for pattern, schema := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			if re.MatchString(key) {
				return schema
			}
		}
	}
	return nil
}
