package jsonschema

// Suggestion represents a suggested correction for an invalid key.
type Suggestion struct {
	Invalid   string
	Suggested string
	Distance  int
}

// SuggestKey finds the closest valid property name for an invalid key using
// Levenshtein distance. Returns nil if no suggestion is close enough.
// The threshold is max(2, len(invalid)/3).
func SuggestKey(invalid string, validKeys []string) *Suggestion {
	if len(validKeys) == 0 || invalid == "" {
		return nil
	}

	threshold := len(invalid) / 3
	if threshold < 2 {
		threshold = 2
	}

	var best *Suggestion
	for _, key := range validKeys {
		d := levenshtein(invalid, key)
		if d <= threshold && (best == nil || d < best.Distance) {
			best = &Suggestion{
				Invalid:   invalid,
				Suggested: key,
				Distance:  d,
			}
		}
	}
	return best
}

// InvalidKeyData is the structured data attached to Diagnostic.Data for invalid
// key diagnostics, enabling code actions (e.g., "Rename to 'summary'").
type InvalidKeyData struct {
	Kind      string `json:"kind"`
	SuggestTo string `json:"suggestTo"`
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost

			m := ins
			if del < m {
				m = del
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
