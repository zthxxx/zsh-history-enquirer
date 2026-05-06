package app

import (
	"fmt"
	"io"
	"os"
)

// openDebugLog returns an io.Writer for the `ZHE_DEBUG=<path>`
// diagnostic log, or io.Discard when the env var is unset (or the
// open fails — diagnostics are best-effort, never load-bearing).
//
// Callers should defer-close the returned writer when it is an
// *os.File. The discarder is idempotent on Close.
func openDebugLog() io.Writer {
	path := os.Getenv("ZHE_DEBUG")
	if path == "" {
		return io.Discard
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return io.Discard
	}
	return f
}

// debugProbe emits the captured cursor / geometry pair to the debug
// log. Used at startup once, after the probe + size queries land.
func debugProbe(w io.Writer, cur cursorResult, leftover string, rows, cols int) {
	if w == nil || w == io.Discard {
		return
	}
	fmt.Fprintf(w, "[zhe] probe: row=%d col=%d err=%v leftover=%q\n",
		cur.row, cur.col, cur.err, leftover)
	fmt.Fprintf(w, "[zhe] geom: rows=%d cols=%d\n", rows, cols)
}
