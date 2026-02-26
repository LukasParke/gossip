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
