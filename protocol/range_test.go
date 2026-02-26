package protocol

import "testing"

func TestPositionBefore(t *testing.T) {
	tests := []struct {
		name string
		a, b Position
		want bool
	}{
		{"same", Position{1, 5}, Position{1, 5}, false},
		{"same line, before", Position{1, 3}, Position{1, 5}, true},
		{"same line, after", Position{1, 7}, Position{1, 5}, false},
		{"earlier line", Position{0, 10}, Position{1, 0}, true},
		{"later line", Position{2, 0}, Position{1, 99}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Before(tt.b); got != tt.want {
				t.Errorf("(%v).Before(%v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestPositionBeforeOrEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b Position
		want bool
	}{
		{"same", Position{1, 5}, Position{1, 5}, true},
		{"before", Position{1, 3}, Position{1, 5}, true},
		{"after", Position{1, 7}, Position{1, 5}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.BeforeOrEqual(tt.b); got != tt.want {
				t.Errorf("(%v).BeforeOrEqual(%v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestPositionAfter(t *testing.T) {
	p := Position{2, 0}
	q := Position{1, 5}
	if !p.After(q) {
		t.Error("expected p.After(q)")
	}
	if p.After(p) {
		t.Error("same position should not be After itself")
	}
}

func TestPositionAfterOrEqual(t *testing.T) {
	p := Position{2, 0}
	q := Position{1, 5}
	if !p.AfterOrEqual(q) {
		t.Error("expected p.AfterOrEqual(q)")
	}
	if !p.AfterOrEqual(p) {
		t.Error("same position should be AfterOrEqual itself")
	}
}

func TestNewRange(t *testing.T) {
	r := NewRange(1, 2, 3, 4)
	if r.Start.Line != 1 || r.Start.Character != 2 || r.End.Line != 3 || r.End.Character != 4 {
		t.Errorf("NewRange produced unexpected range: %+v", r)
	}
}

func TestPointRange(t *testing.T) {
	pos := Position{5, 10}
	r := PointRange(pos)
	if r.Start != pos || r.End != pos {
		t.Errorf("PointRange(%v) = %+v, want start==end==pos", pos, r)
	}
	if !r.IsEmpty() {
		t.Error("PointRange should be empty")
	}
}

func TestRangeIsZero(t *testing.T) {
	if !(Range{}).IsZero() {
		t.Error("zero Range should report IsZero")
	}
	if (Range{Start: Position{0, 1}}).IsZero() {
		t.Error("non-zero start should not be IsZero")
	}
}

func TestRangeIsEmpty(t *testing.T) {
	if !(Range{Start: Position{5, 3}, End: Position{5, 3}}).IsEmpty() {
		t.Error("same start/end should be empty")
	}
	if (Range{Start: Position{5, 3}, End: Position{5, 4}}).IsEmpty() {
		t.Error("different start/end should not be empty")
	}
}

func TestRangeContains(t *testing.T) {
	outer := NewRange(1, 0, 5, 10)
	tests := []struct {
		name  string
		inner Range
		want  bool
	}{
		{"fully inside", NewRange(2, 0, 4, 5), true},
		{"equal", outer, true},
		{"start before", NewRange(0, 5, 3, 0), false},
		{"end after", NewRange(2, 0, 6, 0), false},
		{"both outside", NewRange(0, 0, 10, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := outer.Contains(tt.inner); got != tt.want {
				t.Errorf("outer.Contains(%v) = %v, want %v", tt.inner, got, tt.want)
			}
		})
	}
}

func TestRangeContainsPosition(t *testing.T) {
	r := NewRange(1, 5, 3, 10)
	tests := []struct {
		name string
		pos  Position
		want bool
	}{
		{"inside", Position{2, 0}, true},
		{"at start", Position{1, 5}, true},
		{"at end", Position{3, 10}, true},
		{"before", Position{0, 0}, false},
		{"after", Position{4, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.ContainsPosition(tt.pos); got != tt.want {
				t.Errorf("r.ContainsPosition(%v) = %v, want %v", tt.pos, got, tt.want)
			}
		})
	}
}

func TestRangeOverlaps(t *testing.T) {
	a := NewRange(1, 0, 3, 0)
	tests := []struct {
		name string
		b    Range
		want bool
	}{
		{"overlap", NewRange(2, 0, 4, 0), true},
		{"contained", NewRange(1, 5, 2, 5), true},
		{"adjacent not overlapping", NewRange(3, 0, 5, 0), false},
		{"disjoint", NewRange(4, 0, 5, 0), false},
		{"before", NewRange(0, 0, 1, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.Overlaps(tt.b); got != tt.want {
				t.Errorf("a.Overlaps(%v) = %v, want %v", tt.b, got, tt.want)
			}
		})
	}
}

func TestRangeOverlapsAny(t *testing.T) {
	r := NewRange(2, 0, 4, 0)
	others := []Range{
		NewRange(0, 0, 1, 0),
		NewRange(5, 0, 6, 0),
	}
	if r.OverlapsAny(others) {
		t.Error("should not overlap disjoint ranges")
	}
	others = append(others, NewRange(3, 0, 5, 0))
	if !r.OverlapsAny(others) {
		t.Error("should overlap with third range")
	}
}

func TestRangeClamp(t *testing.T) {
	outer := NewRange(1, 0, 5, 10)

	inner := NewRange(2, 0, 3, 5)
	if got := outer.Clamp(inner); got != inner {
		t.Errorf("Clamp of contained range should return inner, got %+v", got)
	}

	escaped := NewRange(0, 0, 10, 0)
	if got := outer.Clamp(escaped); got != outer {
		t.Errorf("Clamp of non-contained range should return outer, got %+v", got)
	}

	zero := Range{}
	if got := outer.Clamp(zero); got != outer {
		t.Errorf("Clamp of zero range on non-zero outer should return outer, got %+v", got)
	}
}

func TestRangeMerge(t *testing.T) {
	a := NewRange(1, 5, 3, 0)
	b := NewRange(2, 0, 5, 10)
	got := a.Merge(b)
	want := NewRange(1, 5, 5, 10)
	if got != want {
		t.Errorf("Merge = %+v, want %+v", got, want)
	}
}

func TestMergeRanges(t *testing.T) {
	if got := MergeRanges(); !got.IsZero() {
		t.Errorf("MergeRanges() = %+v, want zero", got)
	}

	ranges := []Range{
		NewRange(5, 0, 6, 0),
		NewRange(1, 0, 2, 0),
		NewRange(3, 0, 8, 5),
	}
	got := MergeRanges(ranges...)
	want := NewRange(1, 0, 8, 5)
	if got != want {
		t.Errorf("MergeRanges = %+v, want %+v", got, want)
	}
}
