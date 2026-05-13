package scenarios

import (
	"testing"
)

// TestEmptyHistory documents why scenario 12 has no v2 counterpart.
//
// e2e/scenarios/12-empty-history.exp invokes the picker BINARY
// DIRECTLY with `--histfile /tmp/no-such-history-file`, bypassing
// zsh entirely. The user's v2 constraint #3 forbids that path
// ("e2e 测试必须在 docker 中使用 linux 发布版中的 zsh 测试，
// 不能脱离 zsh 与真实触发方式 Ctrl+R").
//
// The "(no matches)" rendering is already exercised end-to-end
// through the real zsh + widget path by TestCancelPreservesInput
// (port of scenario 03), which pre-types `qwerty-no-match`,
// triggers ^R, and asserts on `(no matches)`. Adding a separate
// empty-history-via-zsh test would only verify zsh's `fc -R` on a
// zero-byte file, not picker behaviour — and that path is already
// covered by zsh's own test suite.
//
// Leaving this as a SKIP rather than deleting the file so the
// coverage rationale stays adjacent to its scenario number for
// future maintenance.
func TestEmptyHistory(t *testing.T) {
	t.Skip("Coverage merged into TestCancelPreservesInput (scenario 03 port). See file comment for rationale.")
}
