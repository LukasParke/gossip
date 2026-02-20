package treesitter

import (
	"context"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/gossip-lsp/gossip/document"
	"github.com/gossip-lsp/gossip/protocol"
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
	// for this file. On first run this is nil.
	Previous []protocol.Diagnostic
}

// MergePrevious takes freshly computed diagnostics (covering only the changed
// regions) and merges them with the cached diagnostics from unchanged regions.
// It removes any previously cached diagnostic whose range overlaps with any of
// the diff's ChangedRanges, then appends the fresh diagnostics. This is the
// standard way for ScopeFile Analyzers that only re-check the affected portion
// of the file to produce a complete diagnostic set.
func (ctx *AnalysisContext) MergePrevious(fresh []protocol.Diagnostic) []protocol.Diagnostic {
	if ctx.Diff == nil || len(ctx.Diff.ChangedRanges) == 0 {
		return fresh
	}

	var merged []protocol.Diagnostic
	for _, d := range ctx.Previous {
		if !rangesOverlapAny(d.Range, ctx.Diff.ChangedRanges) {
			merged = append(merged, d)
		}
	}
	merged = append(merged, fresh...)
	return merged
}

// rangesOverlapAny reports whether r overlaps with any range in rs.
func rangesOverlapAny(r protocol.Range, rs []protocol.Range) bool {
	for _, cr := range rs {
		if rangesOverlap(r, cr) {
			return true
		}
	}
	return false
}

// rangesOverlap reports whether two LSP ranges overlap.
func rangesOverlap(a, b protocol.Range) bool {
	if positionBefore(a.End, b.Start) || positionBefore(b.End, a.Start) {
		return false
	}
	return true
}

// positionBefore reports whether a is strictly before b.
func positionBefore(a, b protocol.Position) bool {
	if a.Line < b.Line {
		return true
	}
	if a.Line == b.Line && a.Character < b.Character {
		return true
	}
	return false
}
