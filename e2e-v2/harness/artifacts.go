package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// event is one entry in the per-session events.jsonl log. The harness
// writes one of these every time the test driver issues a Send /
// Wait / assertion call so a failed-test post-mortem can correlate
// timing.
//
// DESIGN.md decision 12 calls this out: when a scenario flakes, it is
// almost always a missed render window between a Send() and the next
// Wait() — that's exactly what this log makes visible.
type event struct {
	Time    string `json:"t"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail,omitempty"`
	Elapsed string `json:"elapsed,omitempty"`
}

// artifactSink is the per-session collector for failure-debugging
// dumps. Every Session has one; it accumulates the raw byte log and
// the structured event log in memory, then flushes to disk in the
// t.Cleanup hook iff the test failed.
type artifactSink struct {
	mu       sync.Mutex
	raw      []byte
	events   []event
	start    time.Time
	maxBytes int
}

func newArtifactSink() *artifactSink {
	return &artifactSink{
		start: time.Now(),
		// Cap raw-byte capture at 8 MiB. A misbehaving scenario that
		// somehow loops on a 1-KHz keypress for 30 minutes should not
		// hold 100 MiB of pty bytes in RAM, much less write them.
		maxBytes: 8 * 1024 * 1024,
	}
}

func (a *artifactSink) appendRaw(b []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.raw) >= a.maxBytes {
		return
	}
	room := a.maxBytes - len(a.raw)
	if len(b) > room {
		a.raw = append(a.raw, b[:room]...)
		return
	}
	a.raw = append(a.raw, b...)
}

func (a *artifactSink) logEvent(kind, detail string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Kind:    kind,
		Detail:  detail,
		Elapsed: time.Since(a.start).String(),
	})
}

// writeDumps writes the three artifact files into dir/. dir is
// created if missing. Any error is silently swallowed: this runs at
// test-cleanup time and we never want a failed dump to mask the real
// test failure.
func (a *artifactSink) writeDumps(dir string, finalScreen string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "harness: artifact mkdir failed: %v\n", err)
		return
	}

	if err := os.WriteFile(filepath.Join(dir, "screen.txt"), []byte(finalScreen), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "harness: artifact screen write failed: %v\n", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raw.bin"), a.raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "harness: artifact raw write failed: %v\n", err)
	}

	f, err := os.Create(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness: artifact events create failed: %v\n", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range a.events {
		if err := enc.Encode(e); err != nil {
			fmt.Fprintf(os.Stderr, "harness: events encode failed: %v\n", err)
			return
		}
	}
}
