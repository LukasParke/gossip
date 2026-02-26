package treesitter

import (
	"context"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
)

// Check is a declarative, pattern-based diagnostic rule. Register it via
// Server.Check(). The framework runs the Pattern as a tree-sitter query,
// scoped to changed ranges, and converts matches into diagnostics
// automatically.
type Check struct {
	// Pattern is a tree-sitter query pattern (e.g., "(ERROR) @error").
	Pattern string

	// Severity is the LSP diagnostic severity for matches.
	Severity protocol.DiagnosticSeverity

	// Source is the diagnostic source string. If empty, the check name is used.
	Source string

	// Code, if non-empty, is set as the diagnostic code (typically a rule ID).
	Code string

	// CodeDescription, if non-nil, links to documentation for this diagnostic.
	CodeDescription *protocol.CodeDescription

	// Tags, if non-nil, adds diagnostic tags (e.g., unnecessary, deprecated).
	Tags []protocol.DiagnosticTag

	// DeduplicateNested, when true, skips captured nodes that have a child
	// matching the same node kind (i.e., only leaf-level nodes are kept).
	// This is essential for ERROR patterns: tree-sitter nests ERROR nodes,
	// so "(ERROR) @error" captures both parent and child ERRORs. Enabling
	// this keeps only the tightest (deepest) range per error.
	DeduplicateNested bool

	// Filter, if non-nil, is called for each capture. Return true to keep it.
	Filter func(Capture) bool

	// Message converts a capture into a diagnostic message string.
	Message func(Capture) string
}

// AnalysisScope controls when an Analyzer re-runs.
type AnalysisScope int

const (
	// ScopeChanged restricts the analyzer to only the changed ranges.
	ScopeChanged AnalysisScope = iota
	// ScopeFile re-runs the analyzer on the entire file, but only when
	// InterestKinds (if set) intersect with the diff's affected kinds.
	ScopeFile
)

// Analyzer is an imperative diagnostic rule with full control over the
// analysis logic. Register it via Server.Analyze().
type Analyzer struct {
	// Scope controls when the analyzer re-runs.
	Scope AnalysisScope

	// InterestKinds, if non-empty, causes the analyzer to be skipped when none
	// of these node kinds appear in the diff's AffectedKinds. An empty slice
	// means the analyzer runs on every edit.
	InterestKinds []string

	// Run performs the analysis and returns diagnostics. It receives a context
	// with the current tree, diff, document, language, and cached results from
	// the previous run of this analyzer.
	Run func(*AnalysisContext) []protocol.Diagnostic
}

// UserDataProvider returns user-defined data for a document URI. Register via
// DiagnosticEngine.SetUserDataProvider to attach custom state (e.g., an
// OpenAPI index) to AnalysisContext.UserData before analyzers run.
type UserDataProvider func(uri protocol.DocumentURI) interface{}

// AnalysisContext is passed to Analyzer.Run with everything the analyzer needs.
type AnalysisContext struct {
	context.Context

	// Tree is the current parse tree (after the edit).
	Tree *Tree

	// Diff describes what changed from the previous tree.
	Diff *TreeDiff

	// Document is the managed text document.
	Document *document.Document

	// Language is the tree-sitter language for this document.
	Language *tree_sitter.Language

	// Previous holds the cached diagnostics from the last run of this analyzer
	// for this file. Nil on the first run for a document.
	Previous []protocol.Diagnostic

	// UserData holds arbitrary user-defined state from UserDataProvider(uri).
	// It is computed once per tree update before analyzers run and is shared by
	// all analyzers for that document. Do not mutate shared state from Run.
	UserData interface{}
}

// MergePrevious takes freshly computed diagnostics (covering only the changed
// regions) and merges them with ctx.Previous from unchanged regions. It drops
// any previous diagnostic whose range overlaps ctx.Diff.ChangedRanges, then
// appends fresh. Use this when a ScopeFile analyzer only re-checks affected
// regions but must return a complete diagnostic set. If ctx.Diff is nil or has
// no ChangedRanges, returns fresh unchanged.
func (ctx *AnalysisContext) MergePrevious(fresh []protocol.Diagnostic) []protocol.Diagnostic {
	if ctx.Diff == nil || len(ctx.Diff.ChangedRanges) == 0 {
		return fresh
	}

	var merged []protocol.Diagnostic
	for _, d := range ctx.Previous {
		if !d.Range.OverlapsAny(ctx.Diff.ChangedRanges) {
			merged = append(merged, d)
		}
	}
	merged = append(merged, fresh...)
	return merged
}
