package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	cfg, err := NewConfig([]string{"--version"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !cfg.PrintVersion {
		t.Fatalf("PrintVersion = false, want true")
	}
	if !strings.Contains(VersionLine(), "zsh-history-enquirer") {
		t.Fatalf("VersionLine = %q", VersionLine())
	}
}

func TestInputJoin(t *testing.T) {
	cfg, err := NewConfig([]string{"git", "log"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Input != "git log" {
		t.Fatalf("Input = %q", cfg.Input)
	}
}

func TestNewConfig_UnknownFlagReturnsError(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	_, err := NewConfig([]string{"--no-such-flag"}, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	// Stderr must include "Usage:" so the user sees something
	// actionable rather than just exit-1 silence.
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr should print Usage; got %q", stderr.String())
	}
}

func TestNewConfig_FlagsParseValues(t *testing.T) {
	t.Parallel()
	cfg, err := NewConfig(
		[]string{
			"-histfile", "/tmp/x",
			"-histsize", "42",
			"-max-limit", "7",
			"git", "status",
		},
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistFile != "/tmp/x" {
		t.Fatalf("HistFile = %q, want /tmp/x", cfg.HistFile)
	}
	if cfg.HistSize != 42 {
		t.Fatalf("HistSize = %d, want 42", cfg.HistSize)
	}
	if cfg.MaxLimit != 7 {
		t.Fatalf("MaxLimit = %d, want 7", cfg.MaxLimit)
	}
	if cfg.Input != "git status" {
		t.Fatalf("Input = %q, want \"git status\"", cfg.Input)
	}
}

func TestNewConfig_EmptyArgs(t *testing.T) {
	t.Parallel()
	cfg, err := NewConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Input != "" {
		t.Fatalf("Input = %q, want empty", cfg.Input)
	}
	if cfg.PrintVersion {
		t.Fatal("PrintVersion = true, want false")
	}
}

// TestPrintHelp_MatchesNewConfigFlags pins drift between the
// `--help` fast-path output and the runtime parser. Both must
// declare the same flag names; if a contributor adds a flag to
// NewConfig but forgets PrintHelp (or vice versa), this test
// fails. The list is the visible-name set, not the full PrintDefaults
// formatting, so cosmetic changes (default-value rewording, doc
// string tweaks) don't fail the test gratuitously.
func TestPrintHelp_MatchesNewConfigFlags(t *testing.T) {
	t.Parallel()

	var helpBuf bytes.Buffer
	PrintHelp(&helpBuf)
	helpOut := helpBuf.String()

	var ncBuf bytes.Buffer
	_, _ = NewConfig([]string{"-h"}, &ncBuf) // intentionally triggers fs.Usage
	ncOut := ncBuf.String()

	flags := []string{"-histfile", "-histsize", "-max-limit", "-version"}
	for _, name := range flags {
		if !strings.Contains(helpOut, name) {
			t.Errorf("PrintHelp output missing flag %q:\n%s", name, helpOut)
		}
		if !strings.Contains(ncOut, name) {
			t.Errorf("NewConfig.fs.Usage output missing flag %q:\n%s", name, ncOut)
		}
	}

	// Both outputs must share the same Usage prefix. Any drift in
	// the program-name / arg-list line is a visible regression.
	const wantPrefix = "Usage: zsh-history-enquirer [flags] [initial input...]"
	if !strings.Contains(helpOut, wantPrefix) {
		t.Errorf("PrintHelp prefix drift; want %q in:\n%s", wantPrefix, helpOut)
	}
	if !strings.Contains(ncOut, wantPrefix) {
		t.Errorf("NewConfig.fs.Usage prefix drift; want %q in:\n%s", wantPrefix, ncOut)
	}
}

// TestPrintHelp_MentionsEnvVars locks the "Environment:" section
// of the help output. Help lists three env vars users may want
// to set; if any is removed without updating PrintHelp, this test
// fails. The list mirrors the README's Power-user notes.
func TestPrintHelp_MentionsEnvVars(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	PrintHelp(&buf)
	out := buf.String()

	for _, want := range []string{
		"Environment:",
		"HISTFILE",
		"NO_COLOR",
		"ZHE_DEBUG",
		"no-color.org",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintHelp output missing %q:\n%s", want, out)
		}
	}
}

// TestPrintHelp_GoesToWriter verifies PrintHelp respects its writer
// argument — this matters because main.go routes it to os.Stdout
// for the --help fast-path. A regression that hardcodes os.Stderr
// would break the `--help | grep` ergonomic.
func TestPrintHelp_GoesToWriter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	PrintHelp(&buf)
	if buf.Len() == 0 {
		t.Fatal("PrintHelp wrote nothing to its writer arg")
	}
	if !strings.Contains(buf.String(), "histfile") {
		t.Fatalf("PrintHelp body missing flag list:\n%s", buf.String())
	}
}
