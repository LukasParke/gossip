package treesitter_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

func jsonLang() *tree_sitter.Language {
	return tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_json.Language()))
}

// publishCollector collects all PublishDiagnosticsParams sent by the engine.
type publishCollector struct {
	mu     sync.Mutex
	params []protocol.PublishDiagnosticsParams
}

func (pc *publishCollector) publish(_ context.Context, p *protocol.PublishDiagnosticsParams) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.params = append(pc.params, *p)
	return nil
}

func (pc *publishCollector) latest(uri string) *protocol.PublishDiagnosticsParams {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	for i := len(pc.params) - 1; i >= 0; i-- {
		if string(pc.params[i].URI) == uri {
			return &pc.params[i]
		}
	}
	return nil
}

func (pc *publishCollector) waitFor(t testing.TB, uri string, timeout time.Duration) *protocol.PublishDiagnosticsParams {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p := pc.latest(uri); p != nil {
			return p
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for diagnostics on %s", uri)
	return nil
}

func setup(t testing.TB) (*document.Store, *treesitter.Manager, *treesitter.DiagnosticEngine, *publishCollector) {
	t.Helper()
	store := document.NewStore()
	cfg := treesitter.Config{
		Languages: map[string]*tree_sitter.Language{
			".json": jsonLang(),
		},
	}
	mgr := treesitter.NewManager(cfg, store)
	t.Cleanup(mgr.Close)

	logger := testLogger()
	engine := treesitter.NewDiagnosticEngine(mgr, store, logger)

	pc := &publishCollector{}
	engine.SetPublish(pc.publish)

	return store, mgr, engine, pc
}

func testLogger() *slog.Logger {
	return slog.Default()
}

// --- Tests ---

func TestTreeDiff_IsFullReparseOnOpen(t *testing.T) {
	store, mgr, _, _ := setup(t)

	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.json",
			LanguageID: "json",
			Version:    1,
			Text:       `{"key": "value"}`,
		},
	})

	tree := mgr.GetTree("file:///test.json")
	if tree == nil {
		t.Fatal("expected tree to exist after open")
	}
	if tree.Diff == nil {
		t.Fatal("expected Diff to be set on open")
	}
	if !tree.Diff.IsFullReparse {
		t.Error("expected IsFullReparse to be true on initial open")
	}
	if len(tree.Diff.AffectedKinds) == 0 {
		t.Error("expected AffectedKinds to be populated on open")
	}
	if !tree.Diff.AffectsKind("document") {
		t.Error("expected root 'document' kind to be in AffectedKinds")
	}
}

func TestTreeDiff_IncrementalEdit(t *testing.T) {
	store, mgr, _, _ := setup(t)

	uri := protocol.DocumentURI("file:///test.json")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: `{"a": 1}`,
		},
	})

	// Structural change: add a new key (this changes the tree structure)
	store.Change(&protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Range: &protocol.Range{
					Start: protocol.Position{Line: 0, Character: 7},
					End:   protocol.Position{Line: 0, Character: 7},
				},
				Text: `, "b": 2`,
			},
		},
	})

	tree := mgr.GetTree(uri)
	if tree == nil {
		t.Fatal("expected tree after edit")
	}
	if tree.Diff == nil {
		t.Fatal("expected Diff to be set after edit")
	}
	if tree.Diff.IsFullReparse {
		t.Error("expected IsFullReparse to be false after incremental edit")
	}
	if len(tree.Diff.ChangedRanges) == 0 {
		t.Error("expected ChangedRanges to be non-empty after structural edit")
	}
	if len(tree.Diff.AffectedNodes) == 0 {
		t.Error("expected AffectedNodes to be non-empty")
	}
	if len(tree.Diff.AffectedKinds) == 0 {
		t.Error("expected AffectedKinds to be non-empty")
	}
}

func TestQueryCapturesInRanges(t *testing.T) {
	store, mgr, _, _ := setup(t)

	uri := protocol.DocumentURI("file:///test.json")
	// Multi-line JSON with an error only on line 2 ("b": bad)
	src := "{\n  \"a\": 1,\n  \"b\": bad,\n  \"c\": 3\n}"
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: src,
		},
	})

	tree := mgr.GetTree(uri)
	if tree == nil {
		t.Fatal("expected tree")
	}

	lang := jsonLang()

	// Full-tree query should find at least one ERROR
	allCaptures, err := tree.QueryCaptures(lang, "(ERROR) @error")
	if err != nil {
		t.Fatalf("QueryCaptures error: %v", err)
	}
	if len(allCaptures) == 0 {
		t.Fatal("expected at least one ERROR capture in full tree")
	}

	// Scoped query: restrict to line 2 where the error is
	scoped, err := tree.QueryCapturesInRanges(lang, "(ERROR) @error", []protocol.Range{
		{Start: protocol.Position{Line: 2, Character: 0}, End: protocol.Position{Line: 3, Character: 0}},
	})
	if err != nil {
		t.Fatalf("QueryCapturesInRanges error: %v", err)
	}
	if len(scoped) == 0 {
		t.Error("expected ERROR captures in scoped range covering line 2")
	}

	// Scoped query on line 1 (clean line with "a": 1) should find no errors
	clean, err := tree.QueryCapturesInRanges(lang, "(ERROR) @error", []protocol.Range{
		{Start: protocol.Position{Line: 3, Character: 0}, End: protocol.Position{Line: 4, Character: 0}},
	})
	if err != nil {
		t.Fatalf("QueryCapturesInRanges error: %v", err)
	}
	if len(clean) != 0 {
		t.Errorf("expected no ERROR captures in clean range (line 3), got %d", len(clean))
	}
}

func TestCheck_AutoPublishes(t *testing.T) {
	store, _, engine, pc := setup(t)

	engine.RegisterCheck("syntax-errors", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "syntax error" },
	})

	uri := "file:///test.json"
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: protocol.DocumentURI(uri), LanguageID: "json", Version: 1,
			Text: `{"valid": true}`,
		},
	})

	p := pc.waitFor(t, uri, 2*time.Second)
	if len(p.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for valid JSON, got %d", len(p.Diagnostics))
	}
}

func TestCheck_DetectsErrors(t *testing.T) {
	store, _, engine, pc := setup(t)

	engine.RegisterCheck("syntax-errors", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "syntax error" },
	})

	uri := "file:///test.json"
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: protocol.DocumentURI(uri), LanguageID: "json", Version: 1,
			Text: `{"broken": bad}`,
		},
	})

	p := pc.waitFor(t, uri, 2*time.Second)
	if len(p.Diagnostics) == 0 {
		t.Error("expected diagnostics for broken JSON")
	}
	for _, d := range p.Diagnostics {
		if d.Severity != protocol.SeverityError {
			t.Errorf("expected severity Error, got %d", d.Severity)
		}
		if d.Source != "syntax-errors" {
			t.Errorf("expected source 'syntax-errors', got %q", d.Source)
		}
	}
}

func TestCheck_IncrementalUpdate(t *testing.T) {
	store, _, engine, pc := setup(t)

	engine.RegisterCheck("syntax-errors", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "syntax error" },
	})

	uri := protocol.DocumentURI("file:///test.json")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: `{"a": 1}`,
		},
	})

	pc.waitFor(t, string(uri), 2*time.Second)

	// Introduce an error by replacing "1" with "bad"
	store.Change(&protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Range: &protocol.Range{
					Start: protocol.Position{Line: 0, Character: 6},
					End:   protocol.Position{Line: 0, Character: 7},
				},
				Text: "bad",
			},
		},
	})

	// Wait for the update
	time.Sleep(50 * time.Millisecond)
	p := pc.latest(string(uri))
	if p == nil {
		t.Fatal("expected diagnostics after edit")
	}
	if len(p.Diagnostics) == 0 {
		t.Error("expected diagnostics after introducing error")
	}
}

func TestAnalyzer_RunsWithDiff(t *testing.T) {
	store, _, engine, pc := setup(t)

	var runCount int
	var lastDiff *treesitter.TreeDiff

	engine.RegisterAnalyzer("test-analyzer", treesitter.Analyzer{
		Scope: treesitter.ScopeFile,
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			runCount++
			lastDiff = ctx.Diff
			return nil
		},
	})

	uri := protocol.DocumentURI("file:///test.json")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: `{"a": 1}`,
		},
	})

	pc.waitFor(t, string(uri), 2*time.Second)

	if runCount != 1 {
		t.Errorf("expected analyzer to run once on open, ran %d times", runCount)
	}
	if lastDiff == nil || !lastDiff.IsFullReparse {
		t.Error("expected IsFullReparse on first run")
	}
}

func TestAnalyzer_SkipsWhenInterestKindsUnaffected(t *testing.T) {
	store, _, engine, pc := setup(t)

	var runCount int

	engine.RegisterAnalyzer("narrow-analyzer", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"nonexistent_node_kind"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			runCount++
			return nil
		},
	})

	uri := protocol.DocumentURI("file:///test.json")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: `{"a": 1}`,
		},
	})

	pc.waitFor(t, string(uri), 2*time.Second)

	if runCount != 1 {
		t.Fatalf("expected exactly 1 run on open (full reparse), got %d", runCount)
	}

	// Edit that won't produce the nonexistent node kind
	store.Change(&protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Range: &protocol.Range{
					Start: protocol.Position{Line: 0, Character: 6},
					End:   protocol.Position{Line: 0, Character: 7},
				},
				Text: "2",
			},
		},
	})

	time.Sleep(50 * time.Millisecond)
	if runCount != 1 {
		t.Errorf("expected analyzer to be skipped on edit (interest kinds not affected), ran %d times", runCount)
	}
}

func TestAnalyzer_MergePrevious(t *testing.T) {
	store, _, engine, pc := setup(t)

	engine.RegisterAnalyzer("merge-test", treesitter.Analyzer{
		Scope: treesitter.ScopeFile,
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if ctx.Diff.IsFullReparse {
				return []protocol.Diagnostic{
					{Range: protocol.Range{Start: protocol.Position{Line: 1, Character: 2}, End: protocol.Position{Line: 1, Character: 6}}, Message: "diag-line-1"},
					// Place this diagnostic at the value position so it overlaps with the edit
					{Range: protocol.Range{Start: protocol.Position{Line: 3, Character: 7}, End: protocol.Position{Line: 3, Character: 8}}, Message: "diag-at-value"},
				}
			}
			fresh := []protocol.Diagnostic{
				{Range: protocol.Range{Start: protocol.Position{Line: 3, Character: 7}, End: protocol.Position{Line: 3, Character: 10}}, Message: "updated-diag-at-value"},
			}
			return ctx.MergePrevious(fresh)
		},
	})

	uri := protocol.DocumentURI("file:///test.json")
	// Multi-line JSON so changed ranges don't span the whole file
	src := "{\n  \"a\": 1,\n  \"b\": 2,\n  \"c\": 3\n}"
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: src,
		},
	})

	pc.waitFor(t, string(uri), 2*time.Second)
	p := pc.latest(string(uri))
	if len(p.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics on open, got %d", len(p.Diagnostics))
	}

	// Structural change: replace number 3 with array [1] on line 3
	store.Change(&protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Range: &protocol.Range{
					Start: protocol.Position{Line: 3, Character: 7},
					End:   protocol.Position{Line: 3, Character: 8},
				},
				Text: "[1]",
			},
		},
	})

	time.Sleep(50 * time.Millisecond)
	p = pc.latest(string(uri))
	if p == nil {
		t.Fatal("expected diagnostics after edit")
	}

	foundLine1 := false
	foundUpdated := false
	for _, d := range p.Diagnostics {
		if d.Message == "diag-line-1" {
			foundLine1 = true
		}
		if d.Message == "updated-diag-at-value" {
			foundUpdated = true
		}
		if d.Message == "diag-at-value" {
			t.Error("stale 'diag-at-value' should have been replaced by merge (overlaps changed range)")
		}
	}
	if !foundLine1 {
		t.Error("expected 'diag-line-1' to survive merge (not in changed range)")
	}
	if !foundUpdated {
		t.Error("expected 'updated-diag-at-value' from fresh diagnostics")
	}
}

func TestCheck_WithFilter(t *testing.T) {
	store, _, engine, pc := setup(t)

	engine.RegisterCheck("strings-only", treesitter.Check{
		Pattern:  "(string) @str",
		Severity: protocol.SeverityInformation,
		Filter: func(c treesitter.Capture) bool {
			return c.Text == `"special"`
		},
		Message: func(c treesitter.Capture) string {
			return "found special string"
		},
	})

	uri := "file:///test.json"
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: protocol.DocumentURI(uri), LanguageID: "json", Version: 1,
			Text: `{"a": "normal", "b": "special"}`,
		},
	})

	p := pc.waitFor(t, uri, 2*time.Second)
	if len(p.Diagnostics) != 1 {
		t.Fatalf("expected exactly 1 diagnostic (filter should remove non-special), got %d", len(p.Diagnostics))
	}
	if p.Diagnostics[0].Message != "found special string" {
		t.Errorf("unexpected message: %q", p.Diagnostics[0].Message)
	}
}

func TestClearCache_OnClose(t *testing.T) {
	store, _, engine, pc := setup(t)

	engine.RegisterCheck("syntax-errors", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "err" },
	})

	uri := protocol.DocumentURI("file:///test.json")
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: `{"broken": bad}`,
		},
	})

	pc.waitFor(t, string(uri), 2*time.Second)

	store.Close(&protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	engine.ClearCache(uri)

	// Re-open with valid JSON
	store.Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: uri, LanguageID: "json", Version: 1,
			Text: `{"valid": true}`,
		},
	})

	time.Sleep(50 * time.Millisecond)
	p := pc.latest(string(uri))
	if p == nil {
		t.Fatal("expected diagnostics after re-open")
	}
	if len(p.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics for valid JSON after re-open, got %d", len(p.Diagnostics))
	}
}
