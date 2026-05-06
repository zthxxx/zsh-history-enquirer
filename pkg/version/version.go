// Package version exposes build-time identification injected via -ldflags.
package version

import "fmt"

// These variables are populated by the linker (`-X`) at build time.
//
//nolint:gochecknoglobals // injected by build tooling, not configuration.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Version returns the semver tag the binary was built from, or "dev"
// for un-tagged builds.
func Version() string { return version }

// Commit returns the short commit SHA the binary was built from.
func Commit() string { return commit }

// Date returns the RFC 3339 build timestamp (UTC).
func Date() string { return date }

// Full returns a one-line "version (commit, date)" string suitable for
// `--version` output.
func Full() string {
	return fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
}
