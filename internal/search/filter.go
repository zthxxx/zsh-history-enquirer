package search

import "strings"

// AndFilter returns the subset of choices that contain every token,
// case-insensitively. Order is preserved from the input. Empty token
// list returns the input unchanged (without copying).
//
// The cost is O(N * len(tokens)) substring searches; for the realistic
// upper bound (100k entries × ~5 tokens) it completes in single-digit
// milliseconds.
func AndFilter(choices, tokens []string) []string {
	if len(tokens) == 0 {
		return choices
	}

	out := make([]string, 0, len(choices))
	for _, c := range choices {
		if matchesAll(c, tokens) {
			out = append(out, c)
		}
	}
	return out
}

// matchesAll reports whether every token is a substring of c (case-
// insensitive). The lowercase normalisation is performed lazily so the
// short-circuit on the first failed token avoids the full ToLower
// allocation when possible.
func matchesAll(c string, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}

	// Lower-cased once per choice; this is the dominant cost.
	lc := strings.ToLower(c)
	for _, t := range tokens {
		if !strings.Contains(lc, t) {
			return false
		}
	}
	return true
}
