// Package main smoke test — compiles the binary and runs the
// version fast-path against it. This is the only Go-level test we
// have on cmd/zsh-history-enquirer because the rest of main()
// requires a controlling terminal; the e2e docker scenarios cover
// the interactive paths.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
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
		// Widget-mode invocations always pass `--` before $LBUFFER,
		// so even when LBUFFER literally is "--version" the args are
		// ["bin", "--", "--version"] and isVersionFlag returns false.
		// The picker opens normally and the user's typed text is
		// preserved. See plugin/zsh-history-enquirer.plugin.zsh.
		{"widget-mode --version", []string{"bin", "--", "--version"}, false},
		{"widget-mode empty", []string{"bin", "--", ""}, false},
	}
	for _, tc := range cases {
		got := isVersionFlag(tc.args)
		require.Equalf(t, tc.want, got,
			"isVersionFlag(%v) = %v, want %v", tc.args, got, tc.want)
	}
}

// runHelpSmoke runs `bin <flag>` and asserts the output looks like
// the auto-generated Usage with no startup-failed noise on stderr.
// Used by TestSmoke_HelpFlagLong / TestSmoke_HelpFlagShort.
func runHelpSmoke(t *testing.T, flag string) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skipf("smoke test only runs on linux/darwin; got %s", runtime.GOOS)
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "zsh-history-enquirer")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	build := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	build.Dir = "."
	out, err := build.CombinedOutput()
	require.NoErrorf(t, err, "go build failed: %s", out)

	cmd := exec.CommandContext(ctx, bin, flag)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(),
		"binary %s failed: stderr=%q", flag, stderr.String())

	require.Contains(t, stdout.String(), "Usage:",
		"%s output must include Usage line", flag)
	require.Contains(t, stdout.String(), "histfile",
		"%s output must list flag names", flag)
	require.Empty(t, stderr.String(),
		"%s must not write to stderr (no startup-failed nonsense)", flag)
}

// TestSmoke_HelpFlagLong verifies `bin --help` prints usage to
// stdout and exits 0 cleanly. Catches a regression where help
// output gets stacked under a "startup failed:" message because
// flag.ErrHelp propagated through the fx graph.
func TestSmoke_HelpFlagLong(t *testing.T) {
	t.Parallel()
	runHelpSmoke(t, "--help")
}

// TestSmoke_HelpFlagShort mirrors the long-form test for `-h`.
func TestSmoke_HelpFlagShort(t *testing.T) {
	t.Parallel()
	runHelpSmoke(t, "-h")
}

// TestIsHelpFlag mirrors TestIsVersionFlag — the help fast-path uses
// the same len==2 narrow-check pattern, so the same widget-mode
// protection holds: a user typing `--help` or `-h` at the prompt
// and pressing ^R does NOT trigger the help fast-path because the
// widget always passes `--` before $LBUFFER.
func TestIsHelpFlag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"bare --help", []string{"bin", "--help"}, true},
		{"bare -help", []string{"bin", "-help"}, true},
		{"bare -h", []string{"bin", "-h"}, true},
		{"--help with extra arg", []string{"bin", "--help", "foo"}, false},
		{"positional then --help", []string{"bin", "foo", "--help"}, false},
		{"single positional", []string{"bin", "foo"}, false},
		{"no args", []string{"bin"}, false},
		{"widget-mode --help", []string{"bin", "--", "--help"}, false},
		{"widget-mode -h", []string{"bin", "--", "-h"}, false},
	}
	for _, tc := range cases {
		got := isHelpFlag(tc.args)
		require.Equalf(t, tc.want, got,
			"isHelpFlag(%v) = %v, want %v", tc.args, got, tc.want)
	}
}

// TestRecoverStartFailure_PreservesInputOnArgs checks that when fx
// startup fails (provider-time error, e.g. /dev/tty unopenable in a
// headless container) and the user invoked the binary with positional
// args, those args are echoed back to stdout. Without this safety
// net, `BUFFER=$(zsh-history-enquirer "$LBUFFER")` would resolve to
// "" and the user's typed input would be silently destroyed.
func TestRecoverStartFailure_PreservesInputOnArgs(t *testing.T) {
	t.Parallel()
	stdout, stderr := captureStdoutStderr(t)
	recoverStartFailure(stdout, stderr, []string{"git", "log"}, errors.New("dev/tty: not a terminal"))
	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())

	require.Equal(t, "git log\n", readFile(t, stdout.Name()),
		"recoverStartFailure must echo joined argv to stdout")

	stderrStr := readFile(t, stderr.Name())
	require.Contains(t, stderrStr, "startup failed")
	require.Contains(t, stderrStr, "dev/tty: not a terminal",
		"original error must surface on stderr")
}

// TestRecoverStartFailure_NoArgsLeavesStdoutEmpty pins the
// no-positional-args case: argv with only `--version` (or with no
// args at all) means there is no $LBUFFER to preserve, so stdout
// stays empty. (Only stderr should record the failure.)
func TestRecoverStartFailure_NoArgsLeavesStdoutEmpty(t *testing.T) {
	t.Parallel()
	stdout, stderr := captureStdoutStderr(t)
	recoverStartFailure(stdout, stderr, []string{}, errors.New("boom"))
	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())

	require.Empty(t, readFile(t, stdout.Name()),
		"empty argv → no input to preserve → stdout stays empty")
	require.Contains(t, readFile(t, stderr.Name()), "startup failed")
}

// TestRecoverStartFailure_TolerantOfBadArgs makes sure malformed
// argv (e.g. an unknown flag) doesn't compound the disaster — the
// recovery path is best-effort. We log the original startup error to
// stderr and then leave stdout empty rather than panicking.
func TestRecoverStartFailure_TolerantOfBadArgs(t *testing.T) {
	t.Parallel()
	stdout, stderr := captureStdoutStderr(t)
	// `--unknown` is rejected by NewConfig; the recovery code must
	// still write the original error and exit cleanly.
	recoverStartFailure(stdout, stderr, []string{"--unknown"}, errors.New("boom"))
	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())

	require.Empty(t, readFile(t, stdout.Name()),
		"NewConfig rejected --unknown → no input recovered → stdout stays empty")
	require.Contains(t, readFile(t, stderr.Name()), "startup failed",
		"original startup error must still surface")
}

// captureStdoutStderr returns two *os.File handles backed by tempfiles
// the test can read after closing them. Used because
// recoverStartFailure takes *os.File specifically (matching the
// real os.Stdout / os.Stderr signature in main()).
func captureStdoutStderr(t *testing.T) (stdout, stderr *os.File) {
	t.Helper()
	dir := t.TempDir()
	stdout, err := os.Create(filepath.Join(dir, "stdout"))
	require.NoError(t, err)
	stderr, err = os.Create(filepath.Join(dir, "stderr"))
	require.NoError(t, err)
	return stdout, stderr
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	return string(b)
}
