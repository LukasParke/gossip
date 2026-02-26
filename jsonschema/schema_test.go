package jsonschema

import (
	"testing"
)

func TestLoadBasicSchema(t *testing.T) {
	data := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name"]
	}`)

	schema, err := Load(data)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if schema.Root == nil {
		t.Fatal("Root is nil")
	}
	if schema.Root.Type != "object" {
		t.Errorf("expected type 'object', got %q", schema.Root.Type)
	}
	if len(schema.Root.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(schema.Root.Properties))
	}
	if len(schema.Root.Required) != 1 || schema.Root.Required[0] != "name" {
		t.Errorf("expected required ['name'], got %v", schema.Root.Required)
	}

	nameProp := schema.Root.Properties["name"]
	if nameProp == nil || nameProp.Type != "string" {
		t.Error("expected name property with type 'string'")
	}

	ageProp := schema.Root.Properties["age"]
	if ageProp == nil || ageProp.Type != "integer" {
		t.Error("expected age property with type 'integer'")
	}
	if ageProp.Minimum == nil || *ageProp.Minimum != 0 {
		t.Error("expected age minimum=0")
	}
}

func TestLoadWithDefs(t *testing.T) {
	data := []byte(`{
		"type": "object",
		"properties": {
			"server": {"$ref": "#/$defs/ServerObj"}
		},
		"$defs": {
			"ServerObj": {
				"type": "object",
				"properties": {
					"url": {"type": "string"}
				},
				"required": ["url"]
			}
		}
	}`)

	schema, err := Load(data)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	serverProp := schema.Root.Properties["server"]
	if serverProp == nil {
		t.Fatal("server property is nil")
	}
	if serverProp.Type != "object" {
		t.Errorf("expected server to be resolved to type 'object', got %q", serverProp.Type)
	}
	if serverProp.Properties["url"] == nil {
		t.Error("expected server to have 'url' property after $ref resolution")
	}
}

func TestLoadWithEnum(t *testing.T) {
	data := []byte(`{
		"type": "string",
		"enum": ["get", "post", "put", "delete"]
	}`)

	schema, err := Load(data)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(schema.Root.Enum) != 4 {
		t.Errorf("expected 4 enum values, got %d", len(schema.Root.Enum))
	}
}

func TestLoadComposition(t *testing.T) {
	data := []byte(`{
		"oneOf": [
			{"type": "string"},
			{"type": "number"}
		]
	}`)

	schema, err := Load(data)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(schema.Root.OneOf) != 2 {
		t.Errorf("expected 2 oneOf schemas, got %d", len(schema.Root.OneOf))
	}
}

func TestMustLoadPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustLoad to panic on invalid input")
		}
	}()
	MustLoad([]byte(`{invalid`))
}

func TestRefToDefName(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"#/$defs/__schema0", "__schema0"},
		{"#/$defs/ServerObj", "ServerObj"},
		{"#/definitions/Pet", "Pet"},
		{"SomeOtherRef", "SomeOtherRef"},
	}
	for _, tt := range tests {
		got := refToDefName(tt.ref)
		if got != tt.want {
			t.Errorf("refToDefName(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}
