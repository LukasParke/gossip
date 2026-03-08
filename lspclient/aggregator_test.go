package lspclient

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/LukasParke/gossip/protocol"
)

func TestAggregator_SingleSource(t *testing.T) {
	var mu sync.Mutex
	var published []protocol.Diagnostic
	var publishedURI protocol.DocumentURI

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		publishedURI = params.URI
		published = append(published[:0], params.Diagnostics...)
		return nil
	}, 10*time.Millisecond)

	diags := []protocol.Diagnostic{
		{Message: "error 1", Severity: protocol.SeverityError},
		{Message: "warning 1", Severity: protocol.SeverityWarning},
	}
	agg.Set("file:///test.yaml", "telescope", diags)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if publishedURI != "file:///test.yaml" {
		t.Errorf("expected URI file:///test.yaml, got %s", publishedURI)
	}
	if len(published) != 2 {
		t.Errorf("expected 2 diagnostics, got %d", len(published))
	}
}

func TestAggregator_MultipleSources(t *testing.T) {
	var mu sync.Mutex
	var published []protocol.Diagnostic

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		published = append(published[:0], params.Diagnostics...)
		return nil
	}, 10*time.Millisecond)

	uri := protocol.DocumentURI("file:///test.yaml")
	agg.Set(uri, "telescope", []protocol.Diagnostic{
		{Message: "openapi error", Source: "telescope"},
	})
	agg.Set(uri, "yaml-ls", []protocol.Diagnostic{
		{Message: "yaml syntax error", Source: "yaml-language-server"},
		{Message: "schema violation", Source: "yaml-language-server"},
	})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(published) != 3 {
		t.Errorf("expected 3 merged diagnostics, got %d", len(published))
	}
}

func TestAggregator_FlushNow(t *testing.T) {
	var mu sync.Mutex
	var published []protocol.Diagnostic

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		published = append(published[:0], params.Diagnostics...)
		return nil
	}, 5*time.Second) // very long debounce

	uri := protocol.DocumentURI("file:///test.yaml")
	agg.Set(uri, "telescope", []protocol.Diagnostic{
		{Message: "error"},
	})

	agg.FlushNow(uri)

	mu.Lock()
	defer mu.Unlock()
	if len(published) != 1 {
		t.Errorf("expected 1 diagnostic after FlushNow, got %d", len(published))
	}
}

func TestAggregator_Clear(t *testing.T) {
	var mu sync.Mutex
	var publishes [][]protocol.Diagnostic

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]protocol.Diagnostic, len(params.Diagnostics))
		copy(cp, params.Diagnostics)
		publishes = append(publishes, cp)
		return nil
	}, 10*time.Millisecond)

	uri := protocol.DocumentURI("file:///test.yaml")
	agg.Set(uri, "telescope", []protocol.Diagnostic{{Message: "error"}})
	agg.Clear(uri)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	// Clear publishes one empty diagnostic set to clean the client, then the
	// debounce timer should NOT fire (it was cancelled).
	if len(publishes) != 1 {
		t.Errorf("expected 1 publish (empty) after Clear, got %d", len(publishes))
	}
	if len(publishes) > 0 && len(publishes[0]) != 0 {
		t.Errorf("expected empty diagnostics from Clear, got %d", len(publishes[0]))
	}
}

func TestAggregator_SetPublishFunc(t *testing.T) {
	var mu sync.Mutex
	var publishedMsg string

	agg := NewDiagnosticAggregator(nil, 10*time.Millisecond)

	uri := protocol.DocumentURI("file:///test.yaml")
	agg.Set(uri, "telescope", []protocol.Diagnostic{{Message: "before"}})

	time.Sleep(50 * time.Millisecond)

	agg.SetPublishFunc(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		if len(params.Diagnostics) > 0 {
			publishedMsg = params.Diagnostics[0].Message
		}
		return nil
	})

	agg.Set(uri, "telescope", []protocol.Diagnostic{{Message: "after"}})
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if publishedMsg != "after" {
		t.Errorf("expected 'after', got %q", publishedMsg)
	}
}

// --- Realistic multi-source merge tests ---

func TestAggregator_RealisticYAMLMerge(t *testing.T) {
	var mu sync.Mutex
	var published []protocol.Diagnostic

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		published = append(published[:0], params.Diagnostics...)
		return nil
	}, 10*time.Millisecond)

	uri := protocol.DocumentURI("file:///api.yaml")

	// Simulate telescope finding a duplicate-keys warning
	agg.Set(uri, "telescope", []protocol.Diagnostic{
		{
			Message:  "Duplicate key 'get' (first defined at line 7)",
			Severity: protocol.SeverityError,
			Source:   "telescope",
			Code:     "duplicate-keys",
			Range:    protocol.Range{Start: protocol.Position{Line: 12, Character: 4}},
		},
		{
			Message:  "No servers defined; add a 'servers' section",
			Severity: protocol.SeverityWarning,
			Source:   "telescope",
			Code:     "oas3-api-servers",
			Range:    protocol.Range{Start: protocol.Position{Line: 0, Character: 0}},
		},
	})

	// Simulate yaml-ls finding a syntax error at a different location
	agg.Set(uri, "yaml-ls", []protocol.Diagnostic{
		{
			Message:  "Implicit map keys need to be on a single line",
			Severity: protocol.SeverityError,
			Source:   "yaml-language-server",
			Range:    protocol.Range{Start: protocol.Position{Line: 5, Character: 2}},
		},
	})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(published) != 3 {
		t.Errorf("expected 3 merged diagnostics (2 telescope + 1 yaml-ls), got %d", len(published))
		for i, d := range published {
			t.Logf("  [%d] src=%q msg=%q", i, d.Source, d.Message)
		}
	}

	sources := make(map[string]int)
	for _, d := range published {
		sources[d.Source]++
	}
	if sources["telescope"] != 2 {
		t.Errorf("expected 2 telescope diagnostics, got %d", sources["telescope"])
	}
	if sources["yaml-language-server"] != 1 {
		t.Errorf("expected 1 yaml-ls diagnostic, got %d", sources["yaml-language-server"])
	}
}

func TestAggregator_RealisticJSONMerge(t *testing.T) {
	var mu sync.Mutex
	var published []protocol.Diagnostic

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		published = append(published[:0], params.Diagnostics...)
		return nil
	}, 10*time.Millisecond)

	uri := protocol.DocumentURI("file:///api.json")

	// Simulate telescope finding an oas3-schema error
	agg.Set(uri, "telescope", []protocol.Diagnostic{
		{
			Message:  "Required property 'info' is missing",
			Severity: protocol.SeverityError,
			Source:   "oas3-schema",
			Code:     "oas3-schema",
			Range:    protocol.Range{Start: protocol.Position{Line: 0, Character: 0}},
		},
	})

	// Simulate json-ls finding a trailing comma
	agg.Set(uri, "json-ls", []protocol.Diagnostic{
		{
			Message:  "Trailing comma",
			Severity: protocol.SeverityError,
			Source:   "vscode-json-languageserver",
			Range:    protocol.Range{Start: protocol.Position{Line: 3, Character: 15}},
		},
	})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(published) != 2 {
		t.Errorf("expected 2 merged diagnostics (1 telescope + 1 json-ls), got %d", len(published))
	}

	// Both sources should be present
	foundOAS := false
	foundJSON := false
	for _, d := range published {
		if d.Source == "oas3-schema" {
			foundOAS = true
		}
		if d.Source == "vscode-json-languageserver" {
			foundJSON = true
		}
	}
	if !foundOAS {
		t.Error("expected oas3-schema diagnostic in merged output")
	}
	if !foundJSON {
		t.Error("expected json-ls diagnostic in merged output")
	}
}

func TestAggregator_SourceClearedOnClose(t *testing.T) {
	var mu sync.Mutex
	var lastPublished []protocol.Diagnostic
	publishCount := 0

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		lastPublished = append(lastPublished[:0], params.Diagnostics...)
		publishCount++
		return nil
	}, 10*time.Millisecond)

	uri := protocol.DocumentURI("file:///api.yaml")

	// Set up diagnostics from two sources
	agg.Set(uri, "telescope", []protocol.Diagnostic{
		{Message: "schema error", Source: "telescope"},
	})
	agg.Set(uri, "yaml-ls", []protocol.Diagnostic{
		{Message: "syntax error", Source: "yaml-language-server"},
	})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(lastPublished) != 2 {
		t.Errorf("expected 2 diagnostics initially, got %d", len(lastPublished))
	}
	mu.Unlock()

	// Simulate yaml-ls sending empty diagnostics (e.g., after fixing the file)
	agg.Set(uri, "yaml-ls", []protocol.Diagnostic{})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(lastPublished) != 1 {
		t.Errorf("expected 1 diagnostic after yaml-ls cleared, got %d", len(lastPublished))
	}
	if len(lastPublished) > 0 && lastPublished[0].Source != "telescope" {
		t.Errorf("expected remaining diagnostic from telescope, got source=%q", lastPublished[0].Source)
	}
}

func TestAggregator_SourceReplacement(t *testing.T) {
	var mu sync.Mutex
	var lastPublished []protocol.Diagnostic

	agg := NewDiagnosticAggregator(func(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
		mu.Lock()
		defer mu.Unlock()
		lastPublished = append(lastPublished[:0], params.Diagnostics...)
		return nil
	}, 10*time.Millisecond)

	uri := protocol.DocumentURI("file:///test.yaml")
	agg.Set(uri, "yaml-ls", []protocol.Diagnostic{
		{Message: "old error 1"},
		{Message: "old error 2"},
	})
	// Replace with fewer diagnostics from same source
	agg.Set(uri, "yaml-ls", []protocol.Diagnostic{
		{Message: "new error"},
	})

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(lastPublished) != 1 {
		t.Errorf("expected 1 diagnostic (replacement), got %d", len(lastPublished))
	}
	if lastPublished[0].Message != "new error" {
		t.Errorf("expected 'new error', got %q", lastPublished[0].Message)
	}
}
