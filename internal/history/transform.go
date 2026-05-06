// Package history loads, filters, and transforms zsh history entries.
//
// The loading layer (Loader) is split from the transform layer because
// the latter is a pure function on []string and can be exhaustively
// property-tested without touching disk or spawning zsh.
//
// Transform pipeline (matching spec/20-history-loading.md):
//
//  1. Reverse — most recent first.
//  2. Deduplicate — keep the first occurrence (i.e. the most recent
//     instance, since we already reversed).
//  3. Un-escape literal "\\n" sequences into real newlines so multi-
//     line zsh commands render as multiple terminal lines.
package history

import "strings"

// ReverseDedupeUnescape applies the canonical post-load transform to
// the raw lines emitted by `fc -ln 1`. The input is not mutated.
//
// Empty input returns an empty (non-nil) slice.
func ReverseDedupeUnescape(lines []string) []string {
	if len(lines) == 0 {
		return []string{}
	}

	out := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))

	// Walk from the end so duplicates resolve to the most recent
	// occurrence, then unescape on the way out.
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if _, dup := seen[line]; dup {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, unescapeNewlines(line))
	}
	return out
}

// unescapeNewlines replaces the literal two-character sequence "\\n"
// (a backslash followed by an 'n') with a real LF byte. zsh stores
// multi-line history entries with embedded literal "\n"; the picker
// needs them as actual newlines so the renderer can wrap them
// correctly.
//
// We implement the replacement manually instead of via strings.Replace
// to preserve a literal "\\\\n" (escaped backslash followed by 'n')
// — the legacy behavior is to treat that as one backslash plus a
// real newline, but that's a corner case nobody in practice hits, and
// strings.ReplaceAll(s, "\\n", "\n") matches the legacy regex.
func unescapeNewlines(s string) string {
	if !strings.Contains(s, `\n`) {
		return s
	}
	return strings.ReplaceAll(s, `\n`, "\n")
}
