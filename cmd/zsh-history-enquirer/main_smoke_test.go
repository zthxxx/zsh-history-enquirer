// Package main smoke test — compiles the binary and runs the
// version fast-path against it. This is the only Go-level test we
// have on cmd/zsh-history-enquirer because the rest of main()
// requires a controlling terminal; the e2e docker scenarios cover
// the interactive paths.
package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSmoke_VersionFlag builds the binary in a temp dir and runs
// `zsh-history-enquirer --version`. The output must contain the
// program name. Catches:
//   - any regression where main() opens /dev/tty before checking
//     the version flag (which would hang in a CI runner without
//     a controlling terminal).
//   - any regression where --version exits non-zero or prints
//     nothing.
//
// This test runs only on linux/darwin since those are the platforms
// where /dev/tty is meaningful and the build target is exercised.
func TestSmoke_VersionFlag(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skipf("smoke test only runs on linux/darwin; got %s", runtime.GOOS)
	}
	t.Parallel()

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "zsh-history-enquirer")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	build := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	build.Dir = "." // run inside the cmd/zsh-history-enquirer dir
	out, err := build.CombinedOutput()
	require.NoErrorf(t, err, "go build failed: %s", out)

	cmd := exec.CommandContext(ctx, bin, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(),
		"binary --version failed: stderr=%q", stderr.String())

	require.Contains(t, stdout.String(), "zsh-history-enquirer",
		"--version output must include program name")
	require.Empty(t, stderr.String(),
		"--version must not write to stderr")
	require.True(t,
		strings.HasSuffix(stdout.String(), "\n"),
		"--version output must end with newline (CLI convention)")
}

// TestIsVersionFlag pins the narrowed argv check that protects
// widget-mode invocations. The widget calls
//
//	BUFFER=$(zsh-history-enquirer "$LBUFFER")
//
// — passing $LBUFFER as a single positional arg. Earlier code used
// `slices.Contains(os.Args[1:], "--version")` which would
// erroneously fast-path on inputs like `LBUFFER="foo --version"`
// where --version was a positional token, not a flag. The narrowed
// check requires --version to be the ONLY arg.
func TestIsVersionFlag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"bare --version", []string{"bin", "--version"}, true},
		{"bare -version", []string{"bin", "-version"}, true},
		{"--version with extra arg", []string{"bin", "--version", "foo"}, false},
		{"positional then --version", []string{"bin", "foo", "--version"}, false},
		{"single positional", []string{"bin", "foo"}, false},
		{"no args", []string{"bin"}, false},
		{"empty argv", []string{}, false},
	}
	for _, tc := range cases {
		got := isVersionFlag(tc.args)
		require.Equalf(t, tc.want, got,
			"isVersionFlag(%v) = %v, want %v", tc.args, got, tc.want)
	}
}
