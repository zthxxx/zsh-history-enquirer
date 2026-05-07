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

	// Initial cap is a heuristic: the typical narrowed filter is
	// well under 256 entries (a few seconds of typing reduces a
	// 100k-entry history to a handful of matches). Pre-allocating
	// `len(choices)` was the obvious choice but for HISTSIZE=100k
	// it allocates 1.6 MB per call to hold (typically) <100 strings
	// — and AndFilter runs on every keystroke (modulo the render
	// throttle), so the GC pressure was visible in benchmarks. The
	// 256 floor keeps the no-narrow path (single-token, broad match)
	// from re-allocating excessively in the geometric-grow regime;
	// any deeper filter just appends and the runtime amortizes the
	// cost.
	initialCap := 256
	if len(choices) < initialCap {
		initialCap = len(choices)
	}
	out := make([]string, 0, initialCap)
	for _, c := range choices {
		if matchesAll(c, tokens) {
			out = append(out, c)
		}
	}
	return out
}

// matchesAll reports whether every token is a substring of c (case-
// insensitive). Precondition: tokens is non-empty — AndFilter handles
// the empty-tokens case at the boundary, so we don't duplicate the
// guard here.
func matchesAll(c string, tokens []string) bool {
	// Lower-cased once per choice; this is the dominant cost.
	lc := strings.ToLower(c)
	for _, t := range tokens {
		if !strings.Contains(lc, t) {
			return false
		}
	}
	return true
}
