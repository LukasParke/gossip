package jsonschema

import "testing"

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"summary", "sumary", 1},
		{"description", "desription", 1},
	}
	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggestKey(t *testing.T) {
	validKeys := []string{"summary", "description", "operationId", "parameters", "responses"}

	tests := []struct {
		invalid string
		want    string
	}{
		{"sumary", "summary"},
		{"summry", "summary"},
		{"descrption", "description"},
		{"operationid", "operationId"},
		{"paramters", "parameters"},
		{"respon", ""},
		{"zzzzzzzzz", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := SuggestKey(tt.invalid, validKeys)
		if tt.want == "" {
			if got != nil {
				t.Errorf("SuggestKey(%q) = %q, want nil", tt.invalid, got.Suggested)
			}
		} else {
			if got == nil {
				t.Errorf("SuggestKey(%q) = nil, want %q", tt.invalid, tt.want)
			} else if got.Suggested != tt.want {
				t.Errorf("SuggestKey(%q) = %q, want %q", tt.invalid, got.Suggested, tt.want)
			}
		}
	}
}

func TestSuggestKeyEmptyValid(t *testing.T) {
	got := SuggestKey("something", nil)
	if got != nil {
		t.Error("expected nil for empty valid keys")
	}
}
