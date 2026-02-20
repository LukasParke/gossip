package document

import "github.com/LukasParke/gossip/protocol"

// ApplyChanges applies a set of LSP content change events to document text.
// Supports both full and incremental sync.
func ApplyChanges(text string, changes []protocol.TextDocumentContentChangeEvent) string {
	for _, change := range changes {
		if change.Range == nil {
			text = change.Text
		} else {
			start := OffsetAt(text, change.Range.Start)
			end := OffsetAt(text, change.Range.End)
			if start < 0 {
				start = 0
			}
			if end > len(text) {
				end = len(text)
			}
			if start > end {
				start = end
			}
			text = text[:start] + change.Text + text[end:]
		}
	}
	return text
}

// EditRange represents a byte range that was modified, used for tree-sitter integration.
type EditRange struct {
	StartByte  int
	OldEndByte int
	NewEndByte int
	StartPos   protocol.Position
	OldEndPos  protocol.Position
	NewEndPos  protocol.Position
}

// ApplyChangesWithEdits applies changes and returns edit ranges for incremental parsing.
func ApplyChangesWithEdits(text string, changes []protocol.TextDocumentContentChangeEvent) (string, []EditRange) {
	var edits []EditRange
	for _, change := range changes {
		if change.Range == nil {
			edits = append(edits, EditRange{
				StartByte:  0,
				OldEndByte: len(text),
				NewEndByte: len(change.Text),
				StartPos:   protocol.Position{Line: 0, Character: 0},
				OldEndPos:  PositionAt(text, len(text)),
				NewEndPos:  PositionAt(change.Text, len(change.Text)),
			})
			text = change.Text
		} else {
			start := OffsetAt(text, change.Range.Start)
			end := OffsetAt(text, change.Range.End)
			if start < 0 {
				start = 0
			}
			if end > len(text) {
				end = len(text)
			}
			if start > end {
				start = end
			}

			newEnd := start + len(change.Text)
			edits = append(edits, EditRange{
				StartByte:  start,
				OldEndByte: end,
				NewEndByte: newEnd,
				StartPos:   change.Range.Start,
				OldEndPos:  change.Range.End,
				NewEndPos:  PositionAt(text[:start]+change.Text+text[end:], newEnd),
			})
			text = text[:start] + change.Text + text[end:]
		}
	}
	return text, edits
}
