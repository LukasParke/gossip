package gossiptest

import (
	"strings"
	"testing"

	"github.com/gossip-lsp/gossip/protocol"
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
