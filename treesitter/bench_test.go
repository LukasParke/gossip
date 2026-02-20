package treesitter_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	ts_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	ts_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	ts_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	ts_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

// ---------------------------------------------------------------------------
// Language setup helpers
// ---------------------------------------------------------------------------

func goLang() *tree_sitter.Language { return tree_sitter.NewLanguage(unsafe.Pointer(ts_go.Language())) }
func pyLang() *tree_sitter.Language { return tree_sitter.NewLanguage(unsafe.Pointer(ts_python.Language())) }
func jLang() *tree_sitter.Language  { return tree_sitter.NewLanguage(unsafe.Pointer(ts_json.Language())) }
func yLang() *tree_sitter.Language  { return tree_sitter.NewLanguage(unsafe.Pointer(ts_yaml.Language())) }

type langSpec struct {
	name string
	ext  string
	lang *tree_sitter.Language
	gen  func(lines int) string // generates a file with ~N lines
	// A structural edit in the middle of the file
	editGen func(lines int) (rng protocol.Range, text string)
}

// ---------------------------------------------------------------------------
// Realistic source generators
// ---------------------------------------------------------------------------

func genGoFile(lines int) string {
	var b strings.Builder
	b.WriteString("package bench\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
	funcs := (lines - 8) / 6
	if funcs < 1 {
		funcs = 1
	}
	for i := 0; i < funcs; i++ {
		fmt.Fprintf(&b, "// Process%d handles item %d.\nfunc Process%d(input string) string {\n", i, i, i)
		fmt.Fprintf(&b, "\tresult := strings.TrimSpace(input)\n")
		fmt.Fprintf(&b, "\tfmt.Println(result)\n")
		fmt.Fprintf(&b, "\treturn result\n}\n\n")
	}
	return b.String()
}

func genGoEdit(lines int) (protocol.Range, string) {
	midFunc := (lines - 8) / 6 / 2
	line := uint32(8 + midFunc*6 + 2) // the TrimSpace line inside the middle function
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: 1},
		End:   protocol.Position{Line: line + 1, Character: 0},
	}, "\tresult := strings.ToUpper(strings.TrimSpace(input))\n\t_ = len(result)\n"
}

func genPythonFile(lines int) string {
	var b strings.Builder
	b.WriteString("import os\nimport sys\nimport json\n\n")
	funcs := (lines - 4) / 6
	if funcs < 1 {
		funcs = 1
	}
	for i := 0; i < funcs; i++ {
		fmt.Fprintf(&b, "# Process item %d\ndef process_%d(data: str) -> str:\n", i, i)
		fmt.Fprintf(&b, "    result = data.strip()\n")
		fmt.Fprintf(&b, "    print(result)\n")
		fmt.Fprintf(&b, "    return result\n\n")
	}
	return b.String()
}

func genPythonEdit(lines int) (protocol.Range, string) {
	midFunc := (lines - 4) / 6 / 2
	line := uint32(4 + midFunc*6 + 2) // the strip() line
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: 0},
		End:   protocol.Position{Line: line + 1, Character: 0},
	}, "    result = data.strip().upper()\n    _ = len(result)\n"
}

func genJSONFile(lines int) string {
	var b strings.Builder
	b.WriteString("{\n")
	entries := lines - 2
	if entries < 1 {
		entries = 1
	}
	for i := 0; i < entries; i++ {
		comma := ","
		if i == entries-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "  \"key_%d\": \"value_%d\"%s\n", i, i, comma)
	}
	b.WriteString("}\n")
	return b.String()
}

func genJSONEdit(lines int) (protocol.Range, string) {
	mid := uint32((lines - 2) / 2)
	return protocol.Range{
		Start: protocol.Position{Line: mid + 1, Character: 0},
		End:   protocol.Position{Line: mid + 2, Character: 0},
	}, fmt.Sprintf("  \"key_%d\": [1, 2, 3],\n", mid)
}

func genYAMLFile(lines int) string {
	var b strings.Builder
	b.WriteString("---\n")
	entries := (lines - 1) / 6
	if entries < 1 {
		entries = 1
	}
	for i := 0; i < entries; i++ {
		fmt.Fprintf(&b, "# Service %d configuration\n", i)
		fmt.Fprintf(&b, "service_%d:\n", i)
		fmt.Fprintf(&b, "  name: service-%d\n", i)
		fmt.Fprintf(&b, "  version: \"%d.0.0\"\n", i)
		fmt.Fprintf(&b, "  enabled: true\n")
		fmt.Fprintf(&b, "  tags:\n")
	}
	return b.String()
}

func genYAMLEdit(lines int) (protocol.Range, string) {
	midEntry := (lines - 1) / 6 / 2
	line := uint32(1 + midEntry*6 + 4) // the "enabled: true" line
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: 0},
		End:   protocol.Position{Line: line + 1, Character: 0},
	}, "  enabled:\n    - yes\n    - no\n"
}

var languages = []langSpec{
	{name: "Go", ext: ".go", lang: goLang(), gen: genGoFile, editGen: genGoEdit},
	{name: "Python", ext: ".py", lang: pyLang(), gen: genPythonFile, editGen: genPythonEdit},
	{name: "JSON", ext: ".json", lang: jLang(), gen: genJSONFile, editGen: genJSONEdit},
	{name: "YAML", ext: ".yaml", lang: yLang(), gen: genYAMLFile, editGen: genYAMLEdit},
}

var sizes = []struct {
	name  string
	lines int
}{
	{"Small_50", 50},
	{"Medium_500", 500},
	{"Large_5000", 5000},
}

// ---------------------------------------------------------------------------
// LSP implementations: realistic Check/Analyzer rules per language
// ---------------------------------------------------------------------------

func registerGoRules(engine *treesitter.DiagnosticEngine) {
	engine.RegisterCheck("go-syntax", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "syntax error" },
	})
	engine.RegisterCheck("go-comments", treesitter.Check{
		Pattern:  "(comment) @comment",
		Severity: protocol.SeverityInformation,
		Filter:   func(c treesitter.Capture) bool { return strings.Contains(c.Text, "TODO") },
		Message:  func(c treesitter.Capture) string { return "TODO found" },
	})
	engine.RegisterAnalyzer("go-duplicate-funcs", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"function_declaration"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if !ctx.Diff.IsFullReparse && !ctx.Diff.AffectsKind("function_declaration") {
				return ctx.Previous
			}
			captures, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(function_declaration name: (identifier) @name)")
			seen := map[string]bool{}
			var diags []protocol.Diagnostic
			for _, c := range captures {
				if seen[c.Text] {
					diags = append(diags, protocol.Diagnostic{
						Range: treesitter.NodeRange(c.Node), Severity: protocol.SeverityError,
						Message: "duplicate function: " + c.Text,
					})
				}
				seen[c.Text] = true
			}
			return diags
		},
	})
	engine.RegisterAnalyzer("go-unused-imports", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"import_declaration", "import_spec", "selector_expression", "call_expression"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if !ctx.Diff.IsFullReparse &&
				!ctx.Diff.AffectsKind("import_declaration") && !ctx.Diff.AffectsKind("import_spec") &&
				!ctx.Diff.AffectsKind("call_expression") {
				return ctx.Previous
			}
			imports, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(import_spec path: (interpreted_string_literal) @path)")
			calls, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(call_expression function: (selector_expression operand: (identifier) @pkg))")
			usedPkgs := map[string]bool{}
			for _, c := range calls {
				usedPkgs[c.Text] = true
			}
			var diags []protocol.Diagnostic
			for _, c := range imports {
				pkg := strings.Trim(c.Text, "\"")
				parts := strings.Split(pkg, "/")
				shortName := parts[len(parts)-1]
				if !usedPkgs[shortName] {
					diags = append(diags, protocol.Diagnostic{
						Range: treesitter.NodeRange(c.Node), Severity: protocol.SeverityWarning,
						Message: "unused import: " + pkg,
					})
				}
			}
			return diags
		},
	})
}

func registerPythonRules(engine *treesitter.DiagnosticEngine) {
	engine.RegisterCheck("py-syntax", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "syntax error" },
	})
	engine.RegisterCheck("py-todos", treesitter.Check{
		Pattern:  "(comment) @comment",
		Severity: protocol.SeverityInformation,
		Filter:   func(c treesitter.Capture) bool { return strings.Contains(c.Text, "TODO") },
		Message:  func(c treesitter.Capture) string { return "TODO found" },
	})
	engine.RegisterAnalyzer("py-duplicate-funcs", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"function_definition"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if !ctx.Diff.IsFullReparse && !ctx.Diff.AffectsKind("function_definition") {
				return ctx.Previous
			}
			captures, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(function_definition name: (identifier) @name)")
			seen := map[string]bool{}
			var diags []protocol.Diagnostic
			for _, c := range captures {
				if seen[c.Text] {
					diags = append(diags, protocol.Diagnostic{
						Range: treesitter.NodeRange(c.Node), Severity: protocol.SeverityError,
						Message: "duplicate function: " + c.Text,
					})
				}
				seen[c.Text] = true
			}
			return diags
		},
	})
	engine.RegisterAnalyzer("py-duplicate-imports", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"import_statement", "import_from_statement"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if !ctx.Diff.IsFullReparse && !ctx.Diff.AffectsKind("import_statement") {
				return ctx.Previous
			}
			captures, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(import_statement name: (dotted_name) @mod)")
			seen := map[string]bool{}
			var diags []protocol.Diagnostic
			for _, c := range captures {
				if seen[c.Text] {
					diags = append(diags, protocol.Diagnostic{
						Range: treesitter.NodeRange(c.Node), Severity: protocol.SeverityWarning,
						Message: "duplicate import: " + c.Text,
					})
				}
				seen[c.Text] = true
			}
			return diags
		},
	})
}

func registerJSONRules(engine *treesitter.DiagnosticEngine) {
	engine.RegisterCheck("json-syntax", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "syntax error" },
	})
	engine.RegisterAnalyzer("json-duplicate-keys", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"pair", "object"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if !ctx.Diff.IsFullReparse && !ctx.Diff.AffectsKind("pair") {
				return ctx.Previous
			}
			captures, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(pair key: (string) @key)")
			seen := map[string]bool{}
			var diags []protocol.Diagnostic
			for _, c := range captures {
				if seen[c.Text] {
					diags = append(diags, protocol.Diagnostic{
						Range: treesitter.NodeRange(c.Node), Severity: protocol.SeverityError,
						Message: "duplicate key: " + c.Text,
					})
				}
				seen[c.Text] = true
			}
			return diags
		},
	})
	engine.RegisterAnalyzer("json-deep-nesting", treesitter.Analyzer{
		Scope: treesitter.ScopeFile,
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			var diags []protocol.Diagnostic
			var walk func(node *tree_sitter.Node, depth int)
			walk = func(node *tree_sitter.Node, depth int) {
				if node == nil {
					return
				}
				if node.Kind() == "object" || node.Kind() == "array" {
					depth++
					if depth > 5 {
						diags = append(diags, protocol.Diagnostic{
							Range: treesitter.NodeRange(node), Severity: protocol.SeverityWarning,
							Message: fmt.Sprintf("deeply nested structure (depth %d)", depth),
						})
					}
				}
				for i := uint(0); i < node.ChildCount(); i++ {
					walk(node.Child(i), depth)
				}
			}
			walk(ctx.Tree.RootNode(), 0)
			return diags
		},
	})
}

func registerYAMLRules(engine *treesitter.DiagnosticEngine) {
	engine.RegisterCheck("yaml-syntax", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return "syntax error" },
	})
	engine.RegisterAnalyzer("yaml-duplicate-keys", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"block_mapping_pair"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if !ctx.Diff.IsFullReparse && !ctx.Diff.AffectsKind("block_mapping_pair") {
				return ctx.Previous
			}
			captures, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(block_mapping_pair key: (_) @key)")
			seen := map[string]bool{}
			var diags []protocol.Diagnostic
			for _, c := range captures {
				if seen[c.Text] {
					diags = append(diags, protocol.Diagnostic{
						Range: treesitter.NodeRange(c.Node), Severity: protocol.SeverityError,
						Message: "duplicate key: " + c.Text,
					})
				}
				seen[c.Text] = true
			}
			return diags
		},
	})
	engine.RegisterAnalyzer("yaml-deep-nesting", treesitter.Analyzer{
		Scope: treesitter.ScopeFile,
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			var diags []protocol.Diagnostic
			var walk func(node *tree_sitter.Node, depth int)
			walk = func(node *tree_sitter.Node, depth int) {
				if node == nil {
					return
				}
				if node.Kind() == "block_mapping" || node.Kind() == "block_sequence" {
					depth++
					if depth > 5 {
						diags = append(diags, protocol.Diagnostic{
							Range: treesitter.NodeRange(node), Severity: protocol.SeverityWarning,
							Message: fmt.Sprintf("deeply nested structure (depth %d)", depth),
						})
					}
				}
				for i := uint(0); i < node.ChildCount(); i++ {
					walk(node.Child(i), depth)
				}
			}
			walk(ctx.Tree.RootNode(), 0)
			return diags
		},
	})
}

// ---------------------------------------------------------------------------
// Benchmark infrastructure
// ---------------------------------------------------------------------------

type noopPublisher struct{}

func (noopPublisher) publish(_ context.Context, _ *protocol.PublishDiagnosticsParams) error {
	return nil
}

func setupBench(b *testing.B, spec langSpec) (*document.Store, *treesitter.Manager, *treesitter.DiagnosticEngine) {
	store := document.NewStore()
	cfg := treesitter.Config{
		Languages: map[string]*tree_sitter.Language{spec.ext: spec.lang},
	}
	mgr := treesitter.NewManager(cfg, store)
	b.Cleanup(mgr.Close)

	engine := treesitter.NewDiagnosticEngine(mgr, store, slog.New(slog.NewTextHandler(devNull{}, nil)))
	np := noopPublisher{}
	engine.SetPublish(np.publish)

	switch spec.name {
	case "Go":
		registerGoRules(engine)
	case "Python":
		registerPythonRules(engine)
	case "JSON":
		registerJSONRules(engine)
	case "YAML":
		registerYAMLRules(engine)
	}

	return store, mgr, engine
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkInitialParse measures full parse + TreeDiff + all checks/analyzers on first open.
func BenchmarkInitialParse(b *testing.B) {
	for _, spec := range languages {
		for _, sz := range sizes {
			name := fmt.Sprintf("%s/%s", spec.name, sz.name)
			src := spec.gen(sz.lines)
			b.Run(name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(src)))
				for i := 0; i < b.N; i++ {
					store, mgr, _ := setupBench(b, spec)
					uri := protocol.DocumentURI(fmt.Sprintf("file:///bench%d%s", i, spec.ext))
					store.Open(&protocol.DidOpenTextDocumentParams{
						TextDocument: protocol.TextDocumentItem{
							URI: uri, LanguageID: spec.name, Version: 1, Text: src,
						},
					})
					tree := mgr.GetTree(uri)
					if tree == nil {
						b.Fatal("nil tree")
					}
				}
			})
		}
	}
}

// BenchmarkIncrementalEdit measures incremental reparse + scoped checks/analyzers.
func BenchmarkIncrementalEdit(b *testing.B) {
	for _, spec := range languages {
		for _, sz := range sizes {
			name := fmt.Sprintf("%s/%s", spec.name, sz.name)
			src := spec.gen(sz.lines)
			editRange, editText := spec.editGen(sz.lines)

			b.Run(name, func(b *testing.B) {
				store, _, _ := setupBench(b, spec)
				uri := protocol.DocumentURI(fmt.Sprintf("file:///bench%s", spec.ext))
				store.Open(&protocol.DidOpenTextDocumentParams{
					TextDocument: protocol.TextDocumentItem{
						URI: uri, LanguageID: spec.name, Version: 1, Text: src,
					},
				})

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Apply edit
					store.Change(&protocol.DidChangeTextDocumentParams{
						TextDocument: protocol.VersionedTextDocumentIdentifier{
							TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
							Version:                int32(i + 2),
						},
						ContentChanges: []protocol.TextDocumentContentChangeEvent{
							{Range: &editRange, Text: editText},
						},
					})
					// Revert to keep file stable
					revertRange := protocol.Range{
						Start: editRange.Start,
						End: protocol.Position{
							Line:      editRange.Start.Line + uint32(strings.Count(editText, "\n")),
							Character: 0,
						},
					}
					origLines := strings.Split(src, "\n")
					revertText := ""
					startLine := int(editRange.Start.Line)
					endLine := int(editRange.End.Line)
					if startLine < len(origLines) && endLine <= len(origLines) {
						revertText = strings.Join(origLines[startLine:endLine], "\n") + "\n"
					}
					store.Change(&protocol.DidChangeTextDocumentParams{
						TextDocument: protocol.VersionedTextDocumentIdentifier{
							TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
							Version:                int32(i + 3),
						},
						ContentChanges: []protocol.TextDocumentContentChangeEvent{
							{Range: &revertRange, Text: revertText},
						},
					})
				}
			})
		}
	}
}

// BenchmarkScopedQueryVsFull measures QueryCapturesInRanges vs full QueryCaptures.
func BenchmarkScopedQueryVsFull(b *testing.B) {
	for _, spec := range languages {
		for _, sz := range sizes {
			src := spec.gen(sz.lines)
			editRange, _ := spec.editGen(sz.lines)
			scopeRanges := []protocol.Range{editRange}
			pattern := "(ERROR) @error"

			store := document.NewStore()
			cfg := treesitter.Config{Languages: map[string]*tree_sitter.Language{spec.ext: spec.lang}}
			mgr := treesitter.NewManager(cfg, store)
			b.Cleanup(mgr.Close)

			uri := protocol.DocumentURI(fmt.Sprintf("file:///bench%s", spec.ext))
			store.Open(&protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{URI: uri, LanguageID: spec.name, Version: 1, Text: src},
			})
			tree := mgr.GetTree(uri)

			b.Run(fmt.Sprintf("%s/%s/Full", spec.name, sz.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					tree.QueryCaptures(spec.lang, pattern)
				}
			})

			b.Run(fmt.Sprintf("%s/%s/Scoped", spec.name, sz.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					tree.QueryCapturesInRanges(spec.lang, pattern, scopeRanges)
				}
			})
		}
	}
}

// BenchmarkAnalyzerSkip measures the cost of checking InterestKinds and skipping.
func BenchmarkAnalyzerSkip(b *testing.B) {
	for _, spec := range languages {
		src := spec.gen(500)
		store, mgr, _ := setupBench(b, spec)

		uri := protocol.DocumentURI(fmt.Sprintf("file:///bench%s", spec.ext))
		store.Open(&protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: uri, LanguageID: spec.name, Version: 1, Text: src},
		})

		tree := mgr.GetTree(uri)
		diff := &treesitter.TreeDiff{
			AffectedKinds: map[string]bool{"comment": true},
		}

		analyzer := treesitter.Analyzer{
			Scope:         treesitter.ScopeFile,
			InterestKinds: []string{"function_declaration", "function_definition", "pair", "block_mapping_pair"},
			Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
				b.Fatal("should not be called")
				return nil
			},
		}

		b.Run(fmt.Sprintf("%s/Skip", spec.name), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				shouldRun := false
				for _, kind := range analyzer.InterestKinds {
					if diff.AffectsKind(kind) {
						shouldRun = true
						break
					}
				}
				_ = shouldRun
				// Access previous (cache lookup simulation)
				_ = tree.Diff
			}
		})
	}
}

// BenchmarkMergePrevious measures the cost of merging diagnostics.
func BenchmarkMergePrevious(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		prev := make([]protocol.Diagnostic, count)
		for i := range prev {
			prev[i] = protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(i * 2), Character: 0},
					End:   protocol.Position{Line: uint32(i*2 + 1), Character: 10},
				},
				Message: fmt.Sprintf("diag-%d", i),
			}
		}
		changedRanges := []protocol.Range{
			{Start: protocol.Position{Line: uint32(count), Character: 0},
				End: protocol.Position{Line: uint32(count + 5), Character: 0}},
		}
		fresh := []protocol.Diagnostic{
			{Range: changedRanges[0], Message: "fresh"},
		}

		b.Run(fmt.Sprintf("Diags_%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ctx := &treesitter.AnalysisContext{
					Context:  context.Background(),
					Diff:     &treesitter.TreeDiff{ChangedRanges: changedRanges},
					Previous: prev,
				}
				result := ctx.MergePrevious(fresh)
				_ = result
			}
		})
	}
}

// BenchmarkEndToEnd measures the complete open-edit-publish cycle.
func BenchmarkEndToEnd(b *testing.B) {
	for _, spec := range languages {
		for _, sz := range sizes {
			name := fmt.Sprintf("%s/%s", spec.name, sz.name)
			src := spec.gen(sz.lines)
			editRange, editText := spec.editGen(sz.lines)

			b.Run(name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					store, mgr, _ := setupBench(b, spec)
					uri := protocol.DocumentURI(fmt.Sprintf("file:///bench%d%s", i, spec.ext))

					store.Open(&protocol.DidOpenTextDocumentParams{
						TextDocument: protocol.TextDocumentItem{
							URI: uri, LanguageID: spec.name, Version: 1, Text: src,
						},
					})

					store.Change(&protocol.DidChangeTextDocumentParams{
						TextDocument: protocol.VersionedTextDocumentIdentifier{
							TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
							Version:                2,
						},
						ContentChanges: []protocol.TextDocumentContentChangeEvent{
							{Range: &editRange, Text: editText},
						},
					})

					tree := mgr.GetTree(uri)
					if tree == nil || tree.Diff == nil {
						b.Fatal("expected tree with diff")
					}
				}
			})
		}
	}
}
