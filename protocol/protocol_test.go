package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func roundTrip[T any](t *testing.T, original T) T {
	t.Helper()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded T
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return decoded
}

func TestDiagnosticRoundTrip(t *testing.T) {
	original := Diagnostic{
		Range: Range{
			Start: Position{Line: 5, Character: 10},
			End:   Position{Line: 5, Character: 20},
		},
		Severity: SeverityWarning,
		Code:     "E001",
		CodeDescription: &CodeDescription{
			Href: URI("https://example.com/docs/E001"),
		},
		Source:  "my-linter",
		Message: "undefined variable",
		Tags:    []DiagnosticTag{DiagnosticTagUnnecessary, DiagnosticTagDeprecated},
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestPositionRoundTrip(t *testing.T) {
	original := Position{Line: 42, Character: 17}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestRangeRoundTrip(t *testing.T) {
	original := Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 10, Character: 50},
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestLocationRoundTrip(t *testing.T) {
	original := Location{
		URI: DocumentURI("file:///path/to/file.go"),
		Range: Range{
			Start: Position{Line: 1, Character: 5},
			End:   Position{Line: 1, Character: 15},
		},
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestCompletionItemRoundTrip(t *testing.T) {
	original := CompletionItem{
		Label:         "myFunc",
		Kind:          CompletionKindFunction,
		Detail:        "func myFunc(x int)",
		Documentation: "Does something",
		InsertText:    "myFunc($0)",
		TextEdit: &TextEdit{
			Range:   Range{Start: Position{0, 0}, End: Position{0, 0}},
			NewText: "myFunc()",
		},
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestHoverRoundTrip(t *testing.T) {
	r := Range{
		Start: Position{Line: 2, Character: 4},
		End:   Position{Line: 2, Character: 8},
	}
	original := Hover{
		Contents: MarkupContent{Kind: Markdown, Value: "**Bold** hover content"},
		Range:    &r,
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestServerCapabilitiesRoundTrip(t *testing.T) {
	original := ServerCapabilities{
		TextDocumentSync: &TextDocumentSyncOptions{
			OpenClose: true,
			Change:    SyncFull,
			Save:      &SaveOptions{IncludeText: true},
		},
		HoverProvider: true,
		CompletionProvider: &CompletionOptions{
			TriggerCharacters: []string{".", ":"},
			ResolveProvider:   true,
		},
		DefinitionProvider:      true,
		ReferencesProvider:      true,
		DocumentSymbolProvider:  true,
		WorkspaceSymbolProvider: true,
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestSemanticTokensOptionsRoundTrip(t *testing.T) {
	original := SemanticTokensOptions{
		Legend: SemanticTokensLegend{
			TokenTypes:     []string{"keyword", "function", "variable"},
			TokenModifiers: []string{"declaration", "definition"},
		},
		Range: true,
		Full:  true,
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

func TestDocumentDiagnosticReportRoundTrip(t *testing.T) {
	original := DocumentDiagnosticReport{
		Kind:     DiagnosticReportKindFull,
		ResultID: "abc-123",
		Items: []Diagnostic{
			{
				Range:   Range{Start: Position{0, 0}, End: Position{0, 5}},
				Message: "syntax error",
				Severity: SeverityError,
			},
		},
	}
	decoded := roundTrip(t, original)
	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}
