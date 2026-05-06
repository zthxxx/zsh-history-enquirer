package app_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zthxxx/zsh-history-enquirer/internal/app"
)

func TestVersionFlag(t *testing.T) {
	cfg, err := app.NewConfig([]string{"--version"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !cfg.PrintVersion {
		t.Fatalf("PrintVersion = false, want true")
	}
	if !strings.Contains(app.VersionLine(), "zsh-history-enquirer") {
		t.Fatalf("VersionLine = %q", app.VersionLine())
	}
}

func TestInputJoin(t *testing.T) {
	cfg, err := app.NewConfig([]string{"git", "log"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Input != "git log" {
		t.Fatalf("Input = %q", cfg.Input)
	}
}
