package gossiptest

import (
	"sort"
	"strings"
	"testing"

	"github.com/LukasParke/gossip/protocol"
)

// AssertHoverContains asserts that the hover result contains the expected substring.
func AssertHoverContains(t testing.TB, hover *protocol.Hover, substr string) {
	t.Helper()
	if hover == nil {
		t.Fatal("hover result is nil")
	}
	if !strings.Contains(hover.Contents.Value, substr) {
		t.Errorf("hover contents %q does not contain %q", hover.Contents.Value, substr)
	}
}

// AssertCompletionContains asserts that the completion list contains an item with the given label.
func AssertCompletionContains(t testing.TB, list *protocol.CompletionList, label string) {
	t.Helper()
	if list == nil {
		t.Fatal("completion list is nil")
	}
	for _, item := range list.Items {
		if item.Label == label {
			return
		}
	}
	labels := make([]string, len(list.Items))
	for i, item := range list.Items {
		labels[i] = item.Label
	}
	t.Errorf("completion list does not contain %q, got: %v", label, labels)
}

// AssertDiagnosticCount asserts the number of diagnostics for a URI.
func AssertDiagnosticCount(t testing.TB, diags []protocol.PublishDiagnosticsParams, uri string, count int) {
	t.Helper()
	for _, d := range diags {
		if string(d.URI) == uri {
			if len(d.Diagnostics) != count {
				t.Errorf("expected %d diagnostics for %s, got %d", count, uri, len(d.Diagnostics))
			}
			return
		}
	}
	if count != 0 {
		t.Errorf("no diagnostics found for %s, expected %d", uri, count)
	}
}

// AssertLocationCount asserts the number of locations returned.
func AssertLocationCount(t testing.TB, locations []protocol.Location, count int) {
	t.Helper()
	if len(locations) != count {
		t.Errorf("expected %d locations, got %d", count, len(locations))
	}
}

// DiagnosticExpectation describes a single expected diagnostic with its range.
type DiagnosticExpectation struct {
	Message  string
	Severity protocol.DiagnosticSeverity
	Source   string
	Range    protocol.Range
}

// AssertDiagnosticRanges compares actual diagnostics against expected ones.
// Both slices are sorted by range position then message for deterministic
// comparison. Reports count mismatches and per-element field diffs.
func AssertDiagnosticRanges(t testing.TB, actual []protocol.Diagnostic, expected []DiagnosticExpectation) {
	t.Helper()

	if len(expected) == 0 && len(actual) == 0 {
		return
	}

	sortedActual := make([]protocol.Diagnostic, len(actual))
	copy(sortedActual, actual)
	sort.Slice(sortedActual, func(i, j int) bool {
		return diagLess(sortedActual[i], sortedActual[j])
	})

	sortedExpected := make([]DiagnosticExpectation, len(expected))
	copy(sortedExpected, expected)
	sort.Slice(sortedExpected, func(i, j int) bool {
		return expectLess(sortedExpected[i], sortedExpected[j])
	})

	if len(sortedActual) != len(sortedExpected) {
		t.Errorf("diagnostic count mismatch: got %d, want %d", len(sortedActual), len(sortedExpected))
		for i, d := range sortedActual {
			t.Errorf("  actual[%d]: message=%q source=%q severity=%d range=%d:%d-%d:%d",
				i, d.Message, d.Source, d.Severity,
				d.Range.Start.Line, d.Range.Start.Character,
				d.Range.End.Line, d.Range.End.Character)
		}
		for i, e := range sortedExpected {
			t.Errorf("  expected[%d]: message=%q source=%q severity=%d range=%d:%d-%d:%d",
				i, e.Message, e.Source, e.Severity,
				e.Range.Start.Line, e.Range.Start.Character,
				e.Range.End.Line, e.Range.End.Character)
		}
		return
	}

	for i := range sortedExpected {
		act := sortedActual[i]
		exp := sortedExpected[i]
		if act.Message != exp.Message {
			t.Errorf("[%d] message: got %q, want %q", i, act.Message, exp.Message)
		}
		if act.Severity != exp.Severity {
			t.Errorf("[%d] %q: severity got %d, want %d", i, exp.Message, act.Severity, exp.Severity)
		}
		if act.Source != exp.Source {
			t.Errorf("[%d] %q: source got %q, want %q", i, exp.Message, act.Source, exp.Source)
		}
		if act.Range != exp.Range {
			t.Errorf("[%d] %q: range got %d:%d-%d:%d, want %d:%d-%d:%d",
				i, exp.Message,
				act.Range.Start.Line, act.Range.Start.Character,
				act.Range.End.Line, act.Range.End.Character,
				exp.Range.Start.Line, exp.Range.Start.Character,
				exp.Range.End.Line, exp.Range.End.Character)
		}
	}
}

func diagLess(a, b protocol.Diagnostic) bool {
	if a.Range.Start.Line != b.Range.Start.Line {
		return a.Range.Start.Line < b.Range.Start.Line
	}
	if a.Range.Start.Character != b.Range.Start.Character {
		return a.Range.Start.Character < b.Range.Start.Character
	}
	if a.Range.End.Line != b.Range.End.Line {
		return a.Range.End.Line < b.Range.End.Line
	}
	if a.Range.End.Character != b.Range.End.Character {
		return a.Range.End.Character < b.Range.End.Character
	}
	return a.Message < b.Message
}

func expectLess(a, b DiagnosticExpectation) bool {
	if a.Range.Start.Line != b.Range.Start.Line {
		return a.Range.Start.Line < b.Range.Start.Line
	}
	if a.Range.Start.Character != b.Range.Start.Character {
		return a.Range.Start.Character < b.Range.Start.Character
	}
	if a.Range.End.Line != b.Range.End.Line {
		return a.Range.End.Line < b.Range.End.Line
	}
	if a.Range.End.Character != b.Range.End.Character {
		return a.Range.End.Character < b.Range.End.Character
	}
	return a.Message < b.Message
}
