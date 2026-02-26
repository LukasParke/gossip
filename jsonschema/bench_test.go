package jsonschema

import (
	"testing"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	tree_sitter_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"

	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

func benchJSONLang() *tree_sitter.Language {
	return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_json.Language()))
}

func benchYAMLLang() *tree_sitter.Language {
	return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_yaml.Language()))
}

func benchParseTree(b *testing.B, src string, lang *tree_sitter.Language) *treesitter.Tree {
	b.Helper()
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		b.Fatalf("SetLanguage: %v", err)
	}
	raw := parser.Parse([]byte(src), nil)
	if raw == nil {
		b.Fatal("parse returned nil")
	}
	return treesitter.NewTree(raw, []byte(src))
}

var benchSchema *CompiledSchema

func init() {
	data := []byte(`{
		"type": "object",
		"properties": {
			"openapi": {"type": "string"},
			"info": {
				"type": "object",
				"properties": {
					"title": {"type": "string"},
					"version": {"type": "string"},
					"description": {"type": "string"},
					"contact": {"type": "object"}
				},
				"required": ["title", "version"]
			},
			"paths": {"type": "object"},
			"components": {"type": "object"},
			"servers": {"type": "array", "items": {"type": "object"}}
		},
		"required": ["openapi", "info"]
	}`)
	var err error
	benchSchema, err = Load(data)
	if err != nil {
		panic("failed to load bench schema: " + err.Error())
	}
}

func BenchmarkValidateJSON(b *testing.B) {
	src := `{
  "openapi": "3.1.0",
  "info": {"title": "Test", "version": "1.0.0"},
  "paths": {},
  "servers": [{"url": "https://api.example.com"}]
}`
	tree := benchParseTree(b, src, benchJSONLang())
	opts := ValidateOptions{
		Source:   "bench",
		Severity: protocol.SeverityWarning,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(tree, benchSchema, opts)
	}
}

func BenchmarkValidateYAML(b *testing.B) {
	src := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
servers:
  - url: https://api.example.com
`
	tree := benchParseTree(b, src, benchYAMLLang())
	opts := ValidateOptions{
		Source:   "bench",
		Severity: protocol.SeverityWarning,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(tree, benchSchema, opts)
	}
}

func BenchmarkSuggestKey(b *testing.B) {
	validKeys := []string{
		"openapi", "info", "paths", "components", "servers",
		"security", "tags", "externalDocs", "webhooks",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SuggestKey("opeanpi", validKeys)
		_ = SuggestKey("infos", validKeys)
		_ = SuggestKey("component", validKeys)
	}
}

func BenchmarkLoad(b *testing.B) {
	data := []byte(`{
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
			},
			"paths": {"type": "object"}
		},
		"required": ["openapi", "info"]
	}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
