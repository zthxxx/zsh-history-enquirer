package search

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkAndFilter exercises the realistic upper bound: 100k entries,
// 5 tokens, AND-matched. Provides a number to watch for regressions.
//
// Run with: go test -bench=. ./internal/search/...
func BenchmarkAndFilter(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		choices := generateChoices(n)
		tokens := []string{"git", "log", "fuller", "iso", "n"}

		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = AndFilter(choices, tokens)
			}
		})
	}
}

// BenchmarkTokenize on realistic input sizes.
func BenchmarkTokenize(b *testing.B) {
	cases := []string{
		"",
		"git",
		"git log",
		"git log fuller iso n 1",
	}
	for _, in := range cases {
		b.Run(fmt.Sprintf("len=%d", len(in)), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = Tokenize(in)
			}
		})
	}
}

// generateChoices produces a deterministic synthetic history list
// matching the typical shape: ~half are short single-token commands,
// ~half are longer multi-token commands. Cycles through a small
// vocabulary so the benchmark stays representative.
func generateChoices(n int) []string {
	vocab := []string{
		"git", "echo", "cd", "ls", "cat", "vim", "ssh", "make",
		"npm", "go", "log", "status", "diff", "push", "pull",
		"fuller", "iso", "ok",
	}
	out := make([]string, n)
	for i := range n {
		var b strings.Builder
		for j := range 1 + (i % 5) {
			if j > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(vocab[(i+j)%len(vocab)])
		}
		out[i] = b.String()
	}
	return out
}
