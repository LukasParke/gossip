package treesitter

import (
	"testing"

	"github.com/LukasParke/gossip/protocol"
)

func TestTransformPositionThroughEdits(t *testing.T) {
	t.Run("shifts position after multiline insertion", func(t *testing.T) {
		pos, ok := transformPositionThroughEdits(
			protocol.Position{Line: 5, Character: 3},
			[]DiffEdit{{
				Start:  protocol.Position{Line: 1, Character: 0},
				OldEnd: protocol.Position{Line: 1, Character: 0},
				NewEnd: protocol.Position{Line: 3, Character: 0},
			}},
		)
		if !ok {
			t.Fatal("expected position to remain valid")
		}
		if pos.Line != 7 || pos.Character != 3 {
			t.Fatalf("expected line 7 char 3, got line %d char %d", pos.Line, pos.Character)
		}
	})

	t.Run("drops position inside replaced span", func(t *testing.T) {
		_, ok := transformPositionThroughEdits(
			protocol.Position{Line: 2, Character: 4},
			[]DiffEdit{{
				Start:  protocol.Position{Line: 2, Character: 2},
				OldEnd: protocol.Position{Line: 2, Character: 8},
				NewEnd: protocol.Position{Line: 2, Character: 5},
			}},
		)
		if ok {
			t.Fatal("expected position inside replaced span to be invalid")
		}
	})
}

func TestMergePreviousDiagnostics_TranslatesAndFilters(t *testing.T) {
	prev := []protocol.Diagnostic{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 5, Character: 4},
			},
			Message: "preserve-me",
		},
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 2},
				End:   protocol.Position{Line: 2, Character: 8},
			},
			Message: "drop-me",
		},
	}

	fresh := []protocol.Diagnostic{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 2},
				End:   protocol.Position{Line: 2, Character: 5},
			},
			Message: "fresh",
		},
	}

	diff := &TreeDiff{
		ChangedRanges: []protocol.Range{{
			Start: protocol.Position{Line: 2, Character: 2},
			End:   protocol.Position{Line: 2, Character: 5},
		}},
		Edits: []DiffEdit{{
			Start:  protocol.Position{Line: 2, Character: 2},
			OldEnd: protocol.Position{Line: 2, Character: 8},
			NewEnd: protocol.Position{Line: 2, Character: 5},
		}},
	}

	merged := mergePreviousDiagnostics(prev, fresh, diff)
	if len(merged) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(merged))
	}
	if merged[0].Message != "preserve-me" || merged[1].Message != "fresh" {
		t.Fatalf("unexpected merge order/messages: %#v", merged)
	}
}
