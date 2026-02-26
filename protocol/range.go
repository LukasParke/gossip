package protocol

// Before reports whether position p is strictly before other.
func (p Position) Before(other Position) bool {
	return p.Line < other.Line || (p.Line == other.Line && p.Character < other.Character)
}

// BeforeOrEqual reports whether position p is before or equal to other.
func (p Position) BeforeOrEqual(other Position) bool {
	return p.Line < other.Line || (p.Line == other.Line && p.Character <= other.Character)
}

// After reports whether position p is strictly after other.
func (p Position) After(other Position) bool {
	return other.Before(p)
}

// AfterOrEqual reports whether position p is after or equal to other.
func (p Position) AfterOrEqual(other Position) bool {
	return other.BeforeOrEqual(p)
}

// NewRange constructs a Range from raw coordinates.
func NewRange(startLine, startChar, endLine, endChar uint32) Range {
	return Range{
		Start: Position{Line: startLine, Character: startChar},
		End:   Position{Line: endLine, Character: endChar},
	}
}

// PointRange returns a zero-width range at the given position.
// Useful for cursor positions, insertion points, and selection range fallbacks.
func PointRange(pos Position) Range {
	return Range{Start: pos, End: pos}
}

// IsZero reports whether the range is the zero value (both positions at 0:0).
func (r Range) IsZero() bool {
	return r.Start.Line == 0 && r.Start.Character == 0 &&
		r.End.Line == 0 && r.End.Character == 0
}

// IsEmpty reports whether the range has zero length (start == end),
// including non-zero positions like 5:3..5:3.
func (r Range) IsEmpty() bool {
	return r.Start.Line == r.End.Line && r.Start.Character == r.End.Character
}

// Contains reports whether inner is fully contained within r.
// An equal range is considered contained. A zero-valued inner is contained
// only if r itself is zero-valued or starts at 0:0.
func (r Range) Contains(inner Range) bool {
	return r.Start.BeforeOrEqual(inner.Start) && inner.End.BeforeOrEqual(r.End)
}

// ContainsPosition reports whether pos lies within r (inclusive of boundaries).
func (r Range) ContainsPosition(pos Position) bool {
	return r.Start.BeforeOrEqual(pos) && pos.BeforeOrEqual(r.End)
}

// Overlaps reports whether r and other share any positions.
// Adjacent ranges (one ends where the other starts) are not considered overlapping.
func (r Range) Overlaps(other Range) bool {
	if r.End.BeforeOrEqual(other.Start) || other.End.BeforeOrEqual(r.Start) {
		return false
	}
	return true
}

// OverlapsAny reports whether r overlaps with any range in others.
func (r Range) OverlapsAny(others []Range) bool {
	for _, o := range others {
		if r.Overlaps(o) {
			return true
		}
	}
	return false
}

// Clamp returns inner if it is fully contained within r; otherwise returns r.
// This is the standard pattern for ensuring selectionRange ⊆ fullRange as
// required by DocumentSymbol, CallHierarchyItem, and TypeHierarchyItem.
func (r Range) Clamp(inner Range) Range {
	if r.Contains(inner) {
		return inner
	}
	return r
}

// Merge returns the smallest range that contains both r and other.
func (r Range) Merge(other Range) Range {
	start := r.Start
	if other.Start.Before(start) {
		start = other.Start
	}
	end := r.End
	if other.End.After(end) {
		end = other.End
	}
	return Range{Start: start, End: end}
}

// MergeRanges returns the smallest range that contains all provided ranges.
// Returns a zero Range if no ranges are given.
func MergeRanges(ranges ...Range) Range {
	if len(ranges) == 0 {
		return Range{}
	}
	result := ranges[0]
	for _, r := range ranges[1:] {
		result = result.Merge(r)
	}
	return result
}
