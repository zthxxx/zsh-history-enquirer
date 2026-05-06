package ui

import (
	"fmt"
	"testing"
)

// BenchmarkRender measures one full Render() cycle for a typical
// picker state: 100k history entries, filtered down to ~15 visible
// matches, with a 5-token input. This is the hot path that runs on
// every keystroke (modulo the trailing-edge throttle).
func BenchmarkRender(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		choices := generateBenchChoices(n)
		m := NewModel("git log iso", choices, 24, 80, 1, 5, DefaultMaxLimit)
		// Prime: run once so any one-time init is amortised.
		_ = m.Render(RenderOptions{})

		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = m.Render(RenderOptions{PrevSize: m.Limit})
			}
		})
	}
}

// BenchmarkHighlight measures the SGR-wrapping cost on a typical
// rendered line. The token list mirrors the AND-filter benchmark.
func BenchmarkHighlight(b *testing.B) {
	tokens := []string{"git", "log", "fuller", "iso", "n"}
	line := "git log --pretty=fuller --date=iso -n 1"

	b.ReportAllocs()
	for range b.N {
		_ = highlight(line, tokens)
	}
}

func generateBenchChoices(n int) []string {
	templates := []string{
		"git log --pretty=fuller --date=iso -n 1",
		"git status",
		"echo ok",
		"cd Documents",
		"echo zsh-history-enquirer",
		"command-15",
		"git push",
		"npm install",
		"go test -race ./...",
		"task check",
	}
	out := make([]string, n)
	for i := range n {
		out[i] = fmt.Sprintf("%s #%d", templates[i%len(templates)], i)
	}
	return out
}
