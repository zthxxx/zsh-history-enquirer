package version

import (
	"strings"
	"testing"
)

func TestFull_DefaultValues(t *testing.T) {
	t.Parallel()
	got := Full()
	// Default un-injected build identifies as "dev".
	if !strings.Contains(got, "dev") {
		t.Fatalf("Full() = %q, want it to contain %q", got, "dev")
	}
	if !strings.Contains(got, "commit") {
		t.Fatalf("Full() = %q, want it to contain %q", got, "commit")
	}
	if !strings.Contains(got, "built") {
		t.Fatalf("Full() = %q, want it to contain %q", got, "built")
	}
}

func TestVersion_DefaultValue(t *testing.T) {
	t.Parallel()
	if Version() != "dev" {
		t.Fatalf("Version() = %q, want %q", Version(), "dev")
	}
}

func TestCommit_DefaultValue(t *testing.T) {
	t.Parallel()
	if Commit() != "unknown" {
		t.Fatalf("Commit() = %q, want %q", Commit(), "unknown")
	}
}

func TestDate_DefaultValue(t *testing.T) {
	t.Parallel()
	if Date() != "unknown" {
		t.Fatalf("Date() = %q, want %q", Date(), "unknown")
	}
}
