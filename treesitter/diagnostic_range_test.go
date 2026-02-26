package treesitter_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/LukasParke/gossip/gossiptest"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

var updateExpectations = flag.Bool("update", false, "overwrite diagnostic_expectations.json with actual ranges")

type expectationFile struct {
	Scenarios []scenario `json:"scenarios"`
}

type scenario struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Source      string          `json:"source"`
	Language    string          `json:"language"`
	Checks      []checkSpec     `json:"checks"`
	Analyzers   []analyzerSpec  `json:"analyzers,omitempty"`
	Expected    []expectedDiag  `json:"expected"`
}

type analyzerSpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type checkSpec struct {
	Name              string                      `json:"name"`
	Pattern           string                      `json:"pattern"`
	Severity          protocol.DiagnosticSeverity `json:"severity"`
	Message           string                      `json:"message"`
	DeduplicateNested bool                        `json:"deduplicate_nested,omitempty"`
}

type expectedDiag struct {
	Message  string                      `json:"message"`
	Severity protocol.DiagnosticSeverity `json:"severity"`
	Source   string                      `json:"source"`
	Range    jsonRange                   `json:"range"`
}

type jsonRange struct {
	Start jsonPosition `json:"start"`
	End   jsonPosition `json:"end"`
}

type jsonPosition struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

func (r jsonRange) toProtocol() protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: r.Start.Line, Character: r.Start.Character},
		End:   protocol.Position{Line: r.End.Line, Character: r.End.Character},
	}
}

func rangeFromProtocol(r protocol.Range) jsonRange {
	return jsonRange{
		Start: jsonPosition{Line: r.Start.Line, Character: r.Start.Character},
		End:   jsonPosition{Line: r.End.Line, Character: r.End.Character},
	}
}

func expectationsPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "diagnostic_expectations.json")
}

func loadExpectations(t *testing.T) expectationFile {
	t.Helper()
	data, err := os.ReadFile(expectationsPath())
	if err != nil {
		t.Fatalf("failed to read expectations: %v", err)
	}
	var ef expectationFile
	if err := json.Unmarshal(data, &ef); err != nil {
		t.Fatalf("failed to parse expectations: %v", err)
	}
	return ef
}

func saveExpectations(t *testing.T, ef expectationFile) {
	t.Helper()
	data, err := json.MarshalIndent(ef, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal expectations: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(expectationsPath(), data, 0644); err != nil {
		t.Fatalf("failed to write expectations: %v", err)
	}
}

func TestDiagnosticRanges(t *testing.T) {
	ef := loadExpectations(t)
	updated := false

	for i := range ef.Scenarios {
		sc := &ef.Scenarios[i]
		t.Run(sc.Name, func(t *testing.T) {
			store, _, engine, pc := setup(t)

			for _, cs := range sc.Checks {
				msg := cs.Message
				engine.RegisterCheck(cs.Name, treesitter.Check{
					Pattern:           cs.Pattern,
					Severity:          cs.Severity,
					DeduplicateNested: cs.DeduplicateNested,
					Message:           func(c treesitter.Capture) string { return msg },
				})
			}

			for _, as := range sc.Analyzers {
				registerBuiltinAnalyzer(engine, as)
			}

			uri := protocol.DocumentURI("file:///test_" + sc.Name + ".json")
			store.Open(&protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:        uri,
					LanguageID: "json",
					Version:    1,
					Text:       sc.Source,
				},
			})

			p := pc.waitFor(t, string(uri), 2*time.Second)

			if *updateExpectations {
				sc.Expected = buildExpected(p.Diagnostics)
				updated = true
				return
			}

			expectations := make([]gossiptest.DiagnosticExpectation, len(sc.Expected))
			for j, e := range sc.Expected {
				expectations[j] = gossiptest.DiagnosticExpectation{
					Message:  e.Message,
					Severity: e.Severity,
					Source:   e.Source,
					Range:    e.Range.toProtocol(),
				}
			}

			gossiptest.AssertDiagnosticRanges(t, p.Diagnostics, expectations)
		})
	}

	if *updateExpectations && updated {
		saveExpectations(t, ef)
		t.Log("updated diagnostic_expectations.json with actual ranges")
	}
}

// registerBuiltinAnalyzer registers a predefined analyzer by type name.
// This allows JSON scenarios to reference analyzer behaviors without Go code.
func registerBuiltinAnalyzer(engine *treesitter.DiagnosticEngine, as analyzerSpec) {
	switch as.Type {
	case "missing-nodes":
		engine.RegisterAnalyzer(as.Name, treesitter.Analyzer{
			Scope: treesitter.ScopeFile,
			Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
				missing := ctx.Tree.MissingNodes()
				var diags []protocol.Diagnostic
				for _, node := range missing {
					diags = append(diags, protocol.Diagnostic{
						Range:    ctx.Tree.NodeRange(node),
						Severity: protocol.SeverityError,
						Source:   as.Name,
						Message:  "expected " + node.Kind(),
					})
				}
				return diags
			},
		})
	}
}

func buildExpected(diags []protocol.Diagnostic) []expectedDiag {
	sorted := make([]protocol.Diagnostic, len(diags))
	copy(sorted, diags)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Range.Start.Line != sorted[j].Range.Start.Line {
			return sorted[i].Range.Start.Line < sorted[j].Range.Start.Line
		}
		if sorted[i].Range.Start.Character != sorted[j].Range.Start.Character {
			return sorted[i].Range.Start.Character < sorted[j].Range.Start.Character
		}
		return sorted[i].Message < sorted[j].Message
	})

	result := make([]expectedDiag, len(sorted))
	for i, d := range sorted {
		result[i] = expectedDiag{
			Message:  d.Message,
			Severity: d.Severity,
			Source:   d.Source,
			Range:    rangeFromProtocol(d.Range),
		}
	}
	return result
}
