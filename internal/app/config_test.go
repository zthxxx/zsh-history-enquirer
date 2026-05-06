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
