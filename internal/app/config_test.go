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
