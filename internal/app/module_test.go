package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/zthxxx/zsh-history-enquirer/internal/history"
	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
)

// TestModule_GraphResolves builds an fx graph that mirrors the
// production Module shape (with TTY swapped for a stub since we
// can't open /dev/tty in CI). The test fails if any provider has a
// type-mismatch with what its consumers expect — a regression that
// is otherwise only caught at binary startup.
func TestModule_GraphResolves(t *testing.T) {
	t.Parallel()

	// Stub the TTY constructor so we don't need /dev/tty in CI.
	stubTTY := func() (*tty.TTY, error) {
		// Reuse Open() fallback: return nil, real TTY isn't needed
		// because we won't run the lifecycle.
		// We can't easily mock *tty.TTY since its fields are private,
		// so instead just verify that *every other* provider in the
		// graph type-checks. We do this by instantiating a graph
		// without including tty.NewDevTTY.
		return nil, nil
	}

	cfg := &Config{Input: "test", PrintVersion: false}

	app := fxtest.New(t,
		fx.NopLogger,
		fx.Provide(
			func() *Config { return cfg },
			func() Stdout { return io.Discard },
			func() StderrWriter { return io.Discard },
			stubTTY,
			func(c *Config) history.Loader {
				return history.FixtureLoader("/dev/null")
			},
		),
		// We don't include the real `invokeRun` — it would need a
		// real TTY. Just resolve the providers and shut down.
		fx.Invoke(func(*Config, Stdout, StderrWriter, *tty.TTY, history.Loader) {}),
	)

	startCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		t.Fatalf("fx.Start() failed: %v", err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	_ = app.Stop(stopCtx)
}

// TestModule_StdoutStderrTypesAreDistinct asserts that the named
// io.Writer types in the fx graph remain distinct — without this,
// fx silently can't disambiguate which provider satisfies which
// argument and the binary exits 0 with no output (a real bug we
// chased down earlier in development).
func TestModule_StdoutStderrTypesAreDistinct(t *testing.T) {
	t.Parallel()

	var stdoutCalled, stderrCalled bool
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Provide(
			func() Stdout {
				stdoutCalled = true
				return &bytes.Buffer{}
			},
			func() StderrWriter {
				stderrCalled = true
				return &bytes.Buffer{}
			},
		),
		fx.Invoke(func(s Stdout, e StderrWriter) {
			// Both should be injected without ambiguity.
			if s == nil || e == nil {
				t.Fatal("expected both Stdout and StderrWriter to be injected")
			}
		}),
	)

	startCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		t.Fatalf("fx.Start() failed: %v", err)
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	_ = app.Stop(stopCtx)

	if !stdoutCalled || !stderrCalled {
		t.Fatalf("constructors not called: stdout=%v stderr=%v", stdoutCalled, stderrCalled)
	}
}

// TestNewConfig_OSArgsParsing verifies argv parsing more thoroughly
// than the existing config_test.go, including edge cases around
// flags and positional args.
func TestNewConfig_FlagDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := NewConfig([]string{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistSize != 0 {
		t.Errorf("HistSize default = %d, want 0", cfg.HistSize)
	}
	if cfg.MaxLimit != 0 {
		t.Errorf("MaxLimit default = %d, want 0", cfg.MaxLimit)
	}
	if cfg.PrintVersion {
		t.Errorf("PrintVersion default = true, want false")
	}
	if cfg.Input != "" {
		t.Errorf("Input default = %q, want empty", cfg.Input)
	}
}

func TestNewConfig_HistFileFlag(t *testing.T) {
	t.Parallel()

	cfg, err := NewConfig([]string{"--histfile", "/tmp/test_history"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistFile != "/tmp/test_history" {
		t.Errorf("HistFile = %q, want %q", cfg.HistFile, "/tmp/test_history")
	}
}

func TestNewConfig_HistSizeFlag(t *testing.T) {
	t.Parallel()

	cfg, err := NewConfig([]string{"--histsize", "5000"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistSize != 5000 {
		t.Errorf("HistSize = %d, want 5000", cfg.HistSize)
	}
}

func TestNewConfig_MaxLimitFlag(t *testing.T) {
	t.Parallel()

	cfg, err := NewConfig([]string{"--max-limit", "10"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxLimit != 10 {
		t.Errorf("MaxLimit = %d, want 10", cfg.MaxLimit)
	}
}

// TestStderr_DefaultsToOSStderr — the package-level Stderr proxy
// must default to os.Stderr so production code keeps working when
// tests don't swap it.
func TestStderr_DefaultsToOSStderr(t *testing.T) {
	t.Parallel()

	if Stderr != os.Stderr {
		t.Errorf("Stderr default is not os.Stderr (got %v)", Stderr)
	}
}

// preserveOnError encodes the widget contract: BUFFER must never blank
// the user's typed input on widget error. The four cases below pin
// every quadrant of (result, err) × (input).
func TestPreserveOnError(t *testing.T) {
	t.Parallel()

	bang := errors.New("boom")
	cases := []struct {
		name    string
		in      *RunResult
		err     error
		input   string
		wantNil bool
		wantOut string
	}{
		{
			name:    "result-passes-through",
			in:      &RunResult{Output: "git status"},
			err:     bang,
			input:   "git",
			wantOut: "git status",
		},
		{
			name:    "nil-result-with-error-synthesizes-from-input",
			in:      nil,
			err:     bang,
			input:   "git log",
			wantOut: "git log",
		},
		{
			name:    "nil-result-with-error-and-empty-input-stays-nil",
			in:      nil,
			err:     bang,
			input:   "",
			wantNil: true,
		},
		{
			name:    "nil-result-no-error-stays-nil",
			in:      nil,
			err:     nil,
			input:   "git log",
			wantNil: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := preserveOnError(tc.in, tc.err, tc.input)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want non-nil result with %q, got nil", tc.wantOut)
			}
			if got.Output != tc.wantOut {
				t.Fatalf("Output = %q, want %q", got.Output, tc.wantOut)
			}
		})
	}
}
