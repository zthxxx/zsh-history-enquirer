// Package search implements the multi-word AND-filter described in
// docs/spec/30-search-and-filter.md.
//
// The matcher does *not* rank — it only filters. Order of the matches
// equals the order of the inputs, which is reverse-chronological after
// the history loader has run. That deliberate stability is what
// distinguishes this picker from fuzzy finders like fzf.
package search

import "strings"

// Tokenize splits the input on ASCII whitespace, drops empties, and
// lowercases each token. Used by both the filter and the highlighter.
func Tokenize(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Fields(strings.ToLower(input))
	if len(parts) == 0 {
		return nil
	}
	// Deduplicate while preserving first-seen order. Identical tokens
	// would only do redundant Contains() checks otherwise.
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
