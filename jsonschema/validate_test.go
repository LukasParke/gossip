package jsonschema

import (
	"strings"
	"testing"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	tree_sitter_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"

	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

func jsonLang() *tree_sitter.Language {
	return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_json.Language()))
}

func yamlLang() *tree_sitter.Language {
	return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_yaml.Language()))
}

func parseTree(t *testing.T, src string, lang *tree_sitter.Language) *treesitter.Tree {
	t.Helper()
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatalf("SetLanguage: %v", err)
	}
	raw := parser.Parse([]byte(src), nil)
	if raw == nil {
		t.Fatal("parse returned nil")
	}
	return treesitter.NewTree(raw, []byte(src))
}

func defaultOpts() ValidateOptions {
	return ValidateOptions{
		Source:   "test",
		Severity: protocol.SeverityError,
	}
}

func TestValidateJSON_ValidObject(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name"]
	}`))

	tree := parseTree(t, `{"name": "Alice", "age": 30}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d:", len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestValidateJSON_MissingRequired(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name", "age"]
	}`))

	tree := parseTree(t, `{"name": "Alice"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "Required property 'age' is missing") {
			found = true
		}
	}
	if !found {
		t.Error("expected diagnostic about missing 'age'")
	}
}

func TestValidateJSON_UnknownKey(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"summary": {"type": "string"},
			"description": {"type": "string"}
		}
	}`))

	tree := parseTree(t, `{"sumary": "test"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "did you mean 'summary'") {
			found = true
			data, ok := d.Data.(InvalidKeyData)
			if !ok {
				t.Error("expected InvalidKeyData in diagnostic data")
			} else if data.SuggestTo != "summary" {
				t.Errorf("expected suggestTo='summary', got %q", data.SuggestTo)
			}
		}
	}
	if !found {
		t.Error("expected suggestion diagnostic for 'sumary'")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestValidateJSON_ExtensionKeysAllowed(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`))

	tree := parseTree(t, `{"name": "test", "x-custom": "value"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "x-custom") {
			t.Error("x-custom extension should not produce a diagnostic")
		}
	}
}

func TestValidateJSON_EnumConstraint(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"method": {"type": "string", "enum": ["GET", "POST", "PUT", "DELETE"]}
		}
	}`))

	tree := parseTree(t, `{"method": "PATCH"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "not valid") && strings.Contains(d.Message, "PATCH") {
			found = true
		}
	}
	if !found {
		t.Error("expected enum violation diagnostic for 'PATCH'")
	}
}

func TestValidateYAML_ValidObject(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"openapi": {"type": "string"},
			"info": {
				"type": "object",
				"properties": {
					"title": {"type": "string"},
					"version": {"type": "string"}
				},
				"required": ["title", "version"]
			}
		},
		"required": ["openapi", "info"]
	}`))

	yamlSrc := `openapi: "3.1.0"
info:
  title: "My API"
  version: "1.0.0"`

	tree := parseTree(t, yamlSrc, yamlLang())
	result := Validate(tree, schema, defaultOpts())

	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for valid YAML, got %d:", len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestValidateYAML_MissingRequired(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"openapi": {"type": "string"},
			"info": {"type": "object"}
		},
		"required": ["openapi", "info"]
	}`))

	yamlSrc := `openapi: "3.1.0"`

	tree := parseTree(t, yamlSrc, yamlLang())
	result := Validate(tree, schema, defaultOpts())

	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "Required property 'info' is missing") {
			found = true
		}
	}
	if !found {
		t.Error("expected diagnostic about missing 'info'")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestValidateMaxDiagnostics(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"a": {"type": "string"}
		},
		"required": ["a", "b", "c", "d", "e"]
	}`))

	tree := parseTree(t, `{}`, jsonLang())
	opts := ValidateOptions{
		Source:         "test",
		Severity:       protocol.SeverityError,
		MaxDiagnostics: 2,
	}
	result := Validate(tree, schema, opts)

	if len(result.Diagnostics) > 2 {
		t.Errorf("expected at most 2 diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestValidateNilInputs(t *testing.T) {
	schema := MustLoad([]byte(`{"type": "object"}`))
	opts := defaultOpts()

	result := Validate(nil, schema, opts)
	if len(result.Diagnostics) != 0 {
		t.Error("expected 0 diagnostics for nil tree")
	}

	tree := parseTree(t, `{}`, jsonLang())
	result = Validate(tree, nil, opts)
	if len(result.Diagnostics) != 0 {
		t.Error("expected 0 diagnostics for nil schema")
	}
}

// --- additionalProperties tests ---

func TestAdditionalProperties_True(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {"name": {"type": "string"}},
		"additionalProperties": true
	}`))

	tree := parseTree(t, `{"name": "Alice", "extra": "ok"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "extra") {
			t.Errorf("additionalProperties:true should allow 'extra', got: %s", d.Message)
		}
	}
}

func TestAdditionalProperties_False(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {"name": {"type": "string"}},
		"additionalProperties": false
	}`))

	tree := parseTree(t, `{"name": "Alice", "extra": "bad"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "extra") && strings.Contains(d.Message, "not allowed") {
			found = true
		}
	}
	if !found {
		t.Error("expected diagnostic for unknown key 'extra' with additionalProperties:false")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestAdditionalProperties_Schema(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {"name": {"type": "string"}},
		"additionalProperties": {"type": "integer"}
	}`))

	// Valid: extra value is an integer
	tree := parseTree(t, `{"name": "Alice", "count": 5}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "count") {
			t.Errorf("additionalProperties schema allows integer, got: %s", d.Message)
		}
	}

	// Invalid: extra value is a string, schema expects integer
	tree = parseTree(t, `{"name": "Alice", "count": "not-a-number"}`, jsonLang())
	result = Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "integer") {
			found = true
		}
	}
	if !found {
		t.Error("expected type mismatch diagnostic for string value against integer additionalProperties schema")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// --- const tests ---

func TestConst_Match(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"version": {"type": "string", "const": "1.0"}
		}
	}`))

	tree := parseTree(t, `{"version": "1.0"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for matching const, got %d", len(result.Diagnostics))
	}
}

func TestConst_Mismatch(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"version": {"type": "string", "const": "1.0"}
		}
	}`))

	tree := parseTree(t, `{"version": "2.0"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "must be") {
			found = true
		}
	}
	if !found {
		t.Error("expected const mismatch diagnostic")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// --- exclusiveMinimum / exclusiveMaximum tests ---

func TestExclusiveMinimum(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {"type": "number", "exclusiveMinimum": 0}
		}
	}`))

	// Exactly 0 should fail
	tree := parseTree(t, `{"value": 0}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "> 0") {
			found = true
		}
	}
	if !found {
		t.Error("expected exclusiveMinimum violation for value=0")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}

	// 1 should pass
	tree = parseTree(t, `{"value": 1}`, jsonLang())
	result = Validate(tree, schema, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for value=1, got %d", len(result.Diagnostics))
	}
}

func TestExclusiveMaximum(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {"type": "number", "exclusiveMaximum": 100}
		}
	}`))

	// Exactly 100 should fail
	tree := parseTree(t, `{"value": 100}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "< 100") {
			found = true
		}
	}
	if !found {
		t.Error("expected exclusiveMaximum violation for value=100")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}

	// 99 should pass
	tree = parseTree(t, `{"value": 99}`, jsonLang())
	result = Validate(tree, schema, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for value=99, got %d", len(result.Diagnostics))
	}
}

// --- multipleOf tests ---

func TestMultipleOf_Valid(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {"type": "integer", "multipleOf": 5}
		}
	}`))

	tree := parseTree(t, `{"value": 15}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for 15 (multiple of 5), got %d", len(result.Diagnostics))
	}
}

func TestMultipleOf_Invalid(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {"type": "integer", "multipleOf": 5}
		}
	}`))

	tree := parseTree(t, `{"value": 7}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "multiple of 5") {
			found = true
		}
	}
	if !found {
		t.Error("expected multipleOf violation for 7")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// --- if/then/else tests ---

func TestIfThenElse_ThenBranch(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"kind": {"type": "string"},
			"value": {}
		},
		"if": {
			"properties": {"kind": {"const": "number"}}
		},
		"then": {
			"properties": {"value": {"type": "integer"}}
		},
		"else": {
			"properties": {"value": {"type": "string"}}
		}
	}`))

	// kind=number → if matches → then branch → value should be integer, but "hello" is string
	tree := parseTree(t, `{"kind": "number", "value": "hello"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "integer") {
			found = true
		}
	}
	if !found {
		t.Error("expected type mismatch from 'then' branch")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestIfThenElse_ElseBranch(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"kind": {"type": "string"},
			"value": {}
		},
		"if": {
			"properties": {"kind": {"const": "number"}}
		},
		"then": {
			"properties": {"value": {"type": "integer"}}
		},
		"else": {
			"properties": {"value": {"type": "string"}}
		}
	}`))

	// kind=text → if does NOT match (const mismatch) → else branch → value should be string, 42 is not
	tree := parseTree(t, `{"kind": "text", "value": 42}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "string") {
			found = true
		}
	}
	if !found {
		t.Error("expected type mismatch from 'else' branch")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// --- not tests ---

func TestNot_Violation(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"status": {
				"type": "string",
				"not": {"const": "deprecated"}
			}
		}
	}`))

	tree := parseTree(t, `{"status": "deprecated"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "must not match") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'not' schema violation diagnostic")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestNot_Valid(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"status": {
				"type": "string",
				"not": {"const": "deprecated"}
			}
		}
	}`))

	tree := parseTree(t, `{"status": "active"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// --- anyOf / oneOf failure tests ---

func TestAnyOf_NoMatch_Suppressed(t *testing.T) {
	// anyOf failures are intentionally suppressed to avoid noise in complex
	// schemas (e.g., OpenAPI). This test verifies the behavior is silent.
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {
				"anyOf": [
					{"type": "string"},
					{"type": "integer"}
				]
			}
		}
	}`))

	tree := parseTree(t, `{"value": true}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "anyOf") {
			t.Errorf("anyOf failures should be suppressed, got: %s", d.Message)
		}
	}
}

func TestAnyOf_Match(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {
				"anyOf": [
					{"type": "string"},
					{"type": "integer"}
				]
			}
		}
	}`))

	tree := parseTree(t, `{"value": "hello"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for anyOf match, got %d", len(result.Diagnostics))
	}
}

func TestOneOf_NoMatch(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {
				"oneOf": [
					{"type": "string", "minLength": 5},
					{"type": "integer"}
				]
			}
		}
	}`))

	tree := parseTree(t, `{"value": true}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "oneOf") {
			found = true
		}
	}
	if !found {
		t.Error("expected oneOf failure diagnostic")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// --- YAML-specific tests for new features ---

func TestYAML_AdditionalProperties_False(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {"name": {"type": "string"}},
		"additionalProperties": false
	}`))

	tree := parseTree(t, "name: Alice\nextra: bad", yamlLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "extra") {
			found = true
		}
	}
	if !found {
		t.Error("expected diagnostic for 'extra' in YAML with additionalProperties:false")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestYAML_ExclusiveMinimum(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"count": {"type": "number", "exclusiveMinimum": 0}
		}
	}`))

	tree := parseTree(t, "count: 0", yamlLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "> 0") {
			found = true
		}
	}
	if !found {
		t.Error("expected exclusiveMinimum violation in YAML")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

// --- Edge case tests ---

func TestValidate_NilSchemaNode(t *testing.T) {
	tree := parseTree(t, `{"name": "test"}`, jsonLang())

	// Validate with a compiled schema whose Root is nil
	result := Validate(tree, &CompiledSchema{Root: nil}, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for nil Root, got %d", len(result.Diagnostics))
	}

	// Also verify nil tree + nil schema
	result = Validate(nil, nil, defaultOpts())
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for double nil, got %d", len(result.Diagnostics))
	}
}

func TestValidate_EmptyObjectAgainstRequired(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"},
			"email": {"type": "string"}
		},
		"required": ["name", "age", "email"]
	}`))

	tree := parseTree(t, `{}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	if len(result.Diagnostics) != 3 {
		t.Errorf("expected 3 diagnostics for 3 missing required props, got %d:", len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
	for _, req := range []string{"name", "age", "email"} {
		found := false
		for _, d := range result.Diagnostics {
			if strings.Contains(d.Message, req) {
				found = true
			}
		}
		if !found {
			t.Errorf("expected diagnostic about missing '%s'", req)
		}
	}
}

func TestValidate_PatternPropertyMismatch(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"patternProperties": {
			"^x-": {"type": "string"}
		},
		"additionalProperties": false
	}`))

	// "x-custom" matches pattern → allowed, "regular" does not match → error
	tree := parseTree(t, `{"x-custom": "ok", "regular": "bad"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "regular") && strings.Contains(d.Message, "not allowed") {
			found = true
		}
	}
	if !found {
		t.Error("expected diagnostic for 'regular' key not matching any pattern property")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}

	// "x-custom" should NOT produce a diagnostic
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "x-custom") {
			t.Errorf("x-custom should be allowed by patternProperties, got: %s", d.Message)
		}
	}
}

func TestValidate_IfThenElseEdge(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"format": {"type": "string"},
			"data": {}
		},
		"if": {
			"properties": {"format": {"const": "json"}}
		},
		"then": {
			"properties": {"data": {"type": "object"}}
		},
		"else": {
			"properties": {"data": {"type": "string"}}
		}
	}`))

	// format=json → matches if → then branch → data should be object
	// But data is a string → error from then branch
	tree := parseTree(t, `{"format": "json", "data": "not-an-object"}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())
	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "object") {
			found = true
		}
	}
	if !found {
		t.Error("expected type mismatch from 'then' branch (data should be object)")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestValidate_OneOfAmbiguity(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"value": {
				"oneOf": [
					{"type": "string"},
					{"type": "boolean"}
				]
			}
		}
	}`))

	// 42 is a number — matches neither string nor boolean branch
	tree := parseTree(t, `{"value": 42}`, jsonLang())
	result := Validate(tree, schema, defaultOpts())

	found := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "oneOf") {
			found = true
		}
	}
	if !found {
		t.Error("expected oneOf failure when numeric value matches neither string nor boolean")
		for _, d := range result.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}

func TestRequiredPropertyRange_TargetsKey(t *testing.T) {
	schema := MustLoad([]byte(`{
		"type": "object",
		"properties": {
			"info": {
				"type": "object",
				"properties": {
					"title": {"type": "string"},
					"version": {"type": "string"}
				},
				"required": ["title", "version"]
			}
		},
		"required": ["info"]
	}`))

	yamlSrc := "info:\n  title: My API"
	tree := parseTree(t, yamlSrc, yamlLang())
	result := Validate(tree, schema, defaultOpts())

	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "Required property 'version' is missing") {
			// The range should NOT span the whole object; it should be narrowed
			// to the key "info" or the first child
			lines := d.Range.End.Line - d.Range.Start.Line
			if lines > 1 {
				t.Errorf("expected required-property diagnostic to be narrow (<=1 line), spans %d lines", lines+1)
			}
			return
		}
	}
	t.Error("expected diagnostic about missing 'version'")
}
