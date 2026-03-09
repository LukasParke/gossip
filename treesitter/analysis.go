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

// MergePrevious merges freshly computed diagnostics with previous diagnostics
// that are unaffected by the current edit.
//
// Previous diagnostic ranges are translated through ctx.Diff.Edits into the new
// document coordinate space before overlap filtering against ChangedRanges.
// Diagnostics that intersect edited spans are dropped.
func (ctx *AnalysisContext) MergePrevious(fresh []protocol.Diagnostic) []protocol.Diagnostic {
	return mergePreviousDiagnostics(ctx.Previous, fresh, ctx.Diff)
}

func mergePreviousDiagnostics(previous, fresh []protocol.Diagnostic, diff *TreeDiff) []protocol.Diagnostic {
	if diff == nil || diff.IsFullReparse {
		return fresh
	}
	if len(previous) == 0 {
		return fresh
	}
	if len(diff.ChangedRanges) == 0 {
		// If nothing structurally changed, preserve previous diagnostics and
		// append any fresh entries produced by the analyzer/check.
		merged := make([]protocol.Diagnostic, 0, len(previous)+len(fresh))
		merged = append(merged, previous...)
		merged = append(merged, fresh...)
		return merged
	}
	if len(diff.Edits) == 0 {
		// Without edit mapping we cannot safely translate previous ranges.
		return fresh
	}

	merged := make([]protocol.Diagnostic, 0, len(previous)+len(fresh))
	for _, d := range previous {
		rng, ok := transformRangeThroughEdits(d.Range, diff.Edits)
		if !ok || rng.OverlapsAny(diff.ChangedRanges) {
			continue
		}
		d.Range = rng
		merged = append(merged, d)
	}
	merged = append(merged, fresh...)
	return merged
}

func transformRangeThroughEdits(rng protocol.Range, edits []DiffEdit) (protocol.Range, bool) {
	start, ok := transformPositionThroughEdits(rng.Start, edits)
	if !ok {
		return protocol.Range{}, false
	}
	end, ok := transformPositionThroughEdits(rng.End, edits)
	if !ok {
		return protocol.Range{}, false
	}
	if end.Before(start) {
		return protocol.Range{}, false
	}
	return protocol.Range{Start: start, End: end}, true
}

func transformPositionThroughEdits(pos protocol.Position, edits []DiffEdit) (protocol.Position, bool) {
	cur := pos
	for _, edit := range edits {
		if cur.Before(edit.Start) {
			continue
		}
		if cur.Before(edit.OldEnd) {
			// Position falls inside replaced span, so the old diagnostic is stale.
			return protocol.Position{}, false
		}
		cur = shiftPositionAcrossEdit(cur, edit)
	}
	return cur, true
}

func shiftPositionAcrossEdit(pos protocol.Position, edit DiffEdit) protocol.Position {
	if edit.OldEnd.Line == edit.NewEnd.Line {
		if pos.Line == edit.OldEnd.Line {
			deltaChar := int64(edit.NewEnd.Character) - int64(edit.OldEnd.Character)
			pos.Character = uint32(int64(pos.Character) + deltaChar)
		}
		return pos
	}

	if pos.Line == edit.OldEnd.Line {
		suffixChar := int64(pos.Character) - int64(edit.OldEnd.Character)
		pos.Line = edit.NewEnd.Line
		pos.Character = uint32(int64(edit.NewEnd.Character) + suffixChar)
		return pos
	}

	lineDelta := int64(edit.NewEnd.Line) - int64(edit.OldEnd.Line)
	pos.Line = uint32(int64(pos.Line) + lineDelta)
	return pos
}
