package history

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkReverseDedupeUnescape is the hot path that runs every
// time the picker opens — must stay fast at HISTSIZE=100000.
func BenchmarkReverseDedupeUnescape(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		lines := generateLines(n)
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = ReverseDedupeUnescape(lines)
			}
		})
	}
}

// generateLines produces n synthetic history lines with ~30%
// duplicates and ~10% containing literal `\n` escape sequences,
// so the benchmark exercises the dedup map AND the unescape pass.
func generateLines(n int) []string {
	cmds := []string{
		"git status",
		"git log --pretty=fuller --date=iso -n 1",
		"echo ok",
		"cd Documents",
		`echo "alpha\nbeta\ngamma"`, // multi-line entry
		"pwgen --help",
		"cat <<< 123",
	}
	out := make([]string, n)
	for i := range n {
		out[i] = cmds[i%len(cmds)]
		if i%3 == 0 {
			// Reuse — produces dedup hits.
			out[i] = cmds[(i/3)%len(cmds)]
		}
		if strings.Contains(out[i], `\n`) && i%7 == 0 {
			// Mutate slightly to keep the dedup map non-trivial.
			out[i] = out[i] + " #" + fmt.Sprint(i)
		}
	}
	return out
}
