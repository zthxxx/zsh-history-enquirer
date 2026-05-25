package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// Opts captures everything the harness needs to spin up a fresh zsh
// session inside the container.
//
// All fields are optional; sensible defaults match the canonical
// seed sources under e2e/testdata/ (zshrc.template + history/seed.history)
// and the e2e/docker/Dockerfile.{debian,alpine} images.
type Opts struct {
	// Image is the libc label for the run target — "debian" or
	// "alpine". Informational only; set by the runner. The harness
	// itself does not branch on it.
	Image string

	// Rows / Cols set the pty geometry the picker sees on startup.
	// Defaults match xterm's 24x80.
	Rows, Cols int

	// HistoryFixture names a fixture file under testdata/history/.
	// The harness resolves "foo" → "/seed/history/foo.history"
	// inside the container. Empty defaults to "seed" (the default
	// fixture shared by most scenarios). Use logical names rather
	// than full container paths so scenarios stay decoupled from
	// the mount layout (DESIGN.md decision 7).
	HistoryFixture string

	// ZshrcTemplate points at a path *inside the container* that
	// contains the .zshrc template, with `{{PLUGIN}}` substituted
	// at session-start time. Empty defaults to
	// "/seed/zshrc.template".
	ZshrcTemplate string

	// PickerBinary is the path to the picker binary inside the
	// container. Empty defaults to "/usr/local/bin/zsh-history-enquirer".
	PickerBinary string

	// PluginPath is the path to the plugin file inside the container.
	// Empty defaults to "/opt/zsh-history-enquirer/plugin.zsh".
	PluginPath string

	// PreFilter, when non-empty, is typed verbatim BEFORE the test
	// can press ^R — populating zsh's $LBUFFER so the picker opens
	// with that text as the initial filter. Mirrors the
	// `08-prefilter-from-lbuffer.exp` flow where the user has typed
	// some characters before invoking the widget. Empty means open
	// the picker with an empty filter.
	PreFilter string
}

// Session is one running zsh -il under a pty, with a vt10x parser
// fed from the pty master and a structured event log.
type Session struct {
	t    *testing.T
	opts Opts

	cmd  *exec.Cmd
	ptmx *os.File
	term vt10x.Terminal

	// lastActivity is updated by the reader goroutine every time it
	// reads a non-empty chunk from the pty master. The wait-quiescent
	// primitive polls it to detect "render storm settled".
	lastActivity atomic.Int64 // unix nanos

	readWG  sync.WaitGroup
	readErr atomic.Value // error
	readCtx context.Context
	cancel  context.CancelFunc

	artifacts    *artifactSink
	artifactRoot string
	finalScreen  atomic.Value // string, populated at exit

	// exitCh is closed when the child zsh process has been reaped.
	// exitErr stores the cmd.Wait() result (nil on normal exit).
	// Both are populated by a single waiter goroutine started in
	// NewSession — never call cmd.Wait yourself. WaitExit reads
	// these for the bounded-deadline polling; cleanup ignores them
	// (the waiter goroutine is the sole reaper).
	exitCh  chan struct{}
	exitErr atomic.Value // error wrapper (nil-safe via wrap)

	// closeOnce gates the cleanup; t.Cleanup may fire it but so can
	// an explicit Close() call from a future test author.
	closeOnce sync.Once
}

// errBox wraps an error so atomic.Value.Store can accept nil values
// (atomic.Value rejects nil interfaces). A non-nil errBox with .err
// nil means "process exited cleanly"; a nil errBox means "waiter
// hasn't finished".
type errBox struct{ err error }

const (
	defaultRows = 24
	defaultCols = 80

	// seedHistoryRoot is where the container exposes fixture files.
	// The harness resolves Opts.HistoryFixture (a logical name) to
	// seedHistoryRoot + "/" + name + ".history".
	seedHistoryRoot      = "/seed/history"
	defaultHistoryName   = "seed"
	defaultZshrcTemplate = "/seed/zshrc.template"
	defaultPickerBinary  = "/usr/local/bin/zsh-history-enquirer"
	defaultPluginPath    = "/opt/zsh-history-enquirer/plugin.zsh"

	// WaitQuiescent tuning. The picker's RenderInterval is 72 ms
	// (internal/app/run.go:RenderInterval). 60 ms minimum window
	// guarantees one whole throttle cycle plus a slack margin; 2 s
	// ceiling absorbs the 250 ms DSR probe timeout plus first-frame
	// history load.
	defaultQuiescentMin = 60 * time.Millisecond
	defaultQuiescentMax = 2 * time.Second
)

// NewSession spawns `zsh -il` on a fresh pty + vt10x parser. It
// registers a t.Cleanup that:
//
//  1. cancels the reader goroutine,
//  2. waits for it to exit (with a small grace period),
//  3. if the test has failed, writes screen.txt + raw.bin +
//     events.jsonl to e2e/_artifacts/<TestName>/.
//
// The pty is wired to a vt10x.Terminal of the requested geometry; the
// child zsh process is also told the same size via pty.Setsize so the
// picker reads the geometry correctly at startup via TIOCGWINSZ.
func NewSession(t *testing.T, opts Opts) *Session {
	t.Helper()
	o := withDefaults(opts)

	scratch := t.TempDir()
	histPath, err := writeScratchHome(scratch, o)
	if err != nil {
		t.Fatalf("harness: seed scratch HOME: %v", err)
	}

	cmd := exec.Command("zsh", "-il")
	cmd.Env = append(os.Environ(),
		"HOME="+scratch,
		"HISTFILE="+histPath,
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8",
	)
	cmd.Dir = scratch

	ws := &pty.Winsize{Rows: uint16(o.Rows), Cols: uint16(o.Cols)}
	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		t.Fatalf("harness: pty.StartWithSize: %v", err)
	}

	term := vt10x.New(vt10x.WithSize(o.Cols, o.Rows))
	sink := newArtifactSink()

	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		t:            t,
		opts:         o,
		cmd:          cmd,
		ptmx:         ptmx,
		term:         term,
		artifacts:    sink,
		artifactRoot: artifactDirForTest(t),
		readCtx:      ctx,
		cancel:       cancel,
		exitCh:       make(chan struct{}),
	}
	s.lastActivity.Store(time.Now().UnixNano())
	s.finalScreen.Store("")

	s.readWG.Add(1)
	go s.readLoop()

	// Single owner of cmd.Wait: gathers the exit status into exitErr
	// + signals exitCh. WaitExit and cleanup both read this channel
	// rather than calling Wait themselves — see F5 in REVIEW-FINDINGS.
	go func() {
		err := cmd.Wait()
		s.exitErr.Store(errBox{err: err})
		close(s.exitCh)
	}()

	sink.logEvent("session.open", fmt.Sprintf("rows=%d cols=%d image=%s", o.Rows, o.Cols, o.Image))

	t.Cleanup(s.cleanup)

	// Optional pre-filter: type the requested text before any ^R so
	// the picker opens with $LBUFFER populated (scenario 08).
	if o.PreFilter != "" {
		s.Settle()
		if _, err := s.ptmx.Write([]byte(o.PreFilter)); err != nil {
			t.Fatalf("harness: write PreFilter: %v", err)
		}
		s.artifacts.logEvent("prefilter", o.PreFilter)
	}

	return s
}

// withDefaults populates zero-valued fields with the harness's
// canonical defaults. Centralised so every NewSession path agrees.
func withDefaults(o Opts) Opts {
	if o.Rows <= 0 {
		o.Rows = defaultRows
	}
	if o.Cols <= 0 {
		o.Cols = defaultCols
	}
	if o.HistoryFixture == "" {
		o.HistoryFixture = defaultHistoryName
	}
	if o.ZshrcTemplate == "" {
		o.ZshrcTemplate = defaultZshrcTemplate
	}
	if o.PickerBinary == "" {
		o.PickerBinary = defaultPickerBinary
	}
	if o.PluginPath == "" {
		o.PluginPath = defaultPluginPath
	}
	return o
}

// historyFixturePath maps the logical fixture name to the on-disk
// file inside the container. Callers should always go through Opts
// rather than constructing paths directly (DESIGN.md decision 7,
// REVIEW-FINDINGS F3).
func historyFixturePath(name string) string {
	return filepath.Join(seedHistoryRoot, name+".history")
}

// artifactDirForTest derives the per-test artifact directory under
// e2e/_artifacts/. The path is keyed by the testing.T's full name
// (so subtests resolve their own subdir) and rooted at the v2 module
// root, regardless of where the test process happens to be cwd'd.
func artifactDirForTest(t *testing.T) string {
	t.Helper()
	root := moduleRoot()
	// Replace slashes (subtest separators) with underscores so a
	// single test never spills across multiple directories.
	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	return filepath.Join(root, "_artifacts", safeName)
}

// moduleRoot finds the e2e module root by walking upward from cwd
// looking for go.mod whose declared module path is the e2e
// module. Falls back to cwd if we cannot find one. Best-effort —
// only used for the artifact path, never for production wiring.
func moduleRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		modPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modPath); err == nil {
			if strings.Contains(string(data), "/e2e") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

// writeScratchHome materialises ~/.zshrc and ~/.zsh_history inside
// the scratch HOME by substituting `{{PLUGIN}}` in the zshrc template
// and copying the named history fixture. Returns the path to the
// .zsh_history (the value of HISTFILE).
func writeScratchHome(scratch string, o Opts) (string, error) {
	tpl, err := os.ReadFile(o.ZshrcTemplate)
	if err != nil {
		return "", fmt.Errorf("read zshrc template %q: %w", o.ZshrcTemplate, err)
	}
	rendered := strings.ReplaceAll(string(tpl), "{{PLUGIN}}", o.PluginPath)

	if err := os.WriteFile(filepath.Join(scratch, ".zshrc"), []byte(rendered), 0o644); err != nil {
		return "", fmt.Errorf("write .zshrc: %w", err)
	}

	fixturePath := historyFixturePath(o.HistoryFixture)
	hist, err := os.ReadFile(fixturePath)
	if err != nil {
		return "", fmt.Errorf("read history fixture %q: %w", fixturePath, err)
	}
	histPath := filepath.Join(scratch, ".zsh_history")
	if err := os.WriteFile(histPath, hist, 0o600); err != nil {
		return "", fmt.Errorf("write .zsh_history: %w", err)
	}
	return histPath, nil
}

// readLoop pumps pty master bytes through the vt10x parser. It also
// teeds into the artifact sink so a failed test gets a verbatim byte
// trace, and bumps lastActivity so WaitQuiescent can detect "settled".
//
// Exits when:
//   - the context is cancelled (cleanup path), OR
//   - the pty master returns EOF/EIO (child exited).
func (s *Session) readLoop() {
	defer s.readWG.Done()

	buf := make([]byte, 4096)
	for {
		// Bail early if cleanup has fired so we don't race with the
		// fd close.
		select {
		case <-s.readCtx.Done():
			return
		default:
		}

		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			// Feed parser. vt10x.Terminal implements io.Writer.
			if _, werr := s.term.Write(chunk); werr != nil {
				s.readErr.Store(werr)
			}
			s.artifacts.appendRaw(chunk)
			s.lastActivity.Store(time.Now().UnixNano())
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				s.readErr.Store(err)
			}
			return
		}
	}
}

// Send writes raw bytes to the pty master. No implicit wait, no
// transformation. This is the primitive every typed helper composes
// onto.
func (s *Session) Send(b []byte) error {
	s.artifacts.logEvent("send", fmt.Sprintf("%d bytes: %q", len(b), shortBytes(b)))
	_, err := s.ptmx.Write(b)
	return err
}

// Type writes the given text as raw UTF-8. Identical to
// Send([]byte(text)) but the helper makes scenario code read naturally.
func (s *Session) Type(text string) error {
	return s.Send([]byte(text))
}

// SendCtrlR writes \x12, the byte zsh binds ^R to in every keymap.
// The plugin's history_enquire widget catches this and shells out to
// the picker.
func (s *Session) SendCtrlR() error {
	return s.SendKey(KeyCtrlR)
}

// SendEsc writes a single 0x1b byte. The picker's parser FSM debounces
// a bare Esc by ~50 ms (see internal/keys), so callers should pair
// this with a Settle() before checking that the picker has dismissed.
func (s *Session) SendEsc() error {
	return s.SendKey(KeyEsc)
}

// SendEnter writes \r. The picker interprets this as "submit focused
// entry"; zsh interprets it as "execute current BUFFER".
func (s *Session) SendEnter() error {
	return s.SendKey(KeyEnter)
}

// SendKey writes the byte sequence registered for the typed Key. See
// keys.go for the catalog.
func (s *Session) SendKey(k Key) error {
	b := k.Bytes()
	if len(b) == 0 {
		return fmt.Errorf("harness: SendKey: unknown key %d", k)
	}
	s.artifacts.logEvent("sendkey", fmt.Sprintf("key=%d bytes=%q", k, b))
	_, err := s.ptmx.Write(b)
	return err
}

// SendBracketedPaste wraps payload in DECPM 2004 sentinels and writes
// the result. The picker's parser FSM coalesces this into a single
// EventPaste; the typed text never goes through the per-key debounce.
func (s *Session) SendBracketedPaste(payload []byte) error {
	s.artifacts.logEvent("sendpaste", fmt.Sprintf("%d bytes: %q", len(payload), shortBytes(payload)))
	if _, err := s.ptmx.Write(BracketedPasteOpen); err != nil {
		return err
	}
	if _, err := s.ptmx.Write(payload); err != nil {
		return err
	}
	if _, err := s.ptmx.Write(BracketedPasteClose); err != nil {
		return err
	}
	return nil
}

// Resize updates both the pty winsize (which the kernel will SIGWINCH
// to the child) and the vt10x parser's view, then settles. See risk
// #5 in DESIGN.md for the rationale on the post-resize wait.
func (s *Session) Resize(rows, cols int) error {
	s.artifacts.logEvent("resize", fmt.Sprintf("rows=%d cols=%d", rows, cols))
	if err := pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		return err
	}
	s.term.Resize(cols, rows)
	s.WaitQuiescent(80*time.Millisecond, 1*time.Second)
	return nil
}

// WaitQuiescent reads pty bytes until the parser has been idle for
// at least `min` continuous duration since the call started, or `max`
// has elapsed since the call started. Returns the elapsed wall-clock
// duration so a flaky test can be retuned.
//
// The "since the call started" floor matters: a Settle() that fires
// 100ms after the previous one would otherwise return immediately
// (last activity timestamp is older than `min`) and miss any
// reaction the just-sent byte is about to provoke. By taking
// max(callStart, lastActivity) as the reference, the harness always
// waits at least `min` from the call before considering the screen
// settled. This mirrors expect's behaviour (which also waits a
// minimum interval after each send) and matches what the legacy
// .exp scenarios encode as `sleep 1.2` after `^R`.
//
// Implementation: poll lastActivity every 10 ms. This avoids the
// async-channel ceremony that buys nothing here — the reader goroutine
// already publishes activity via an atomic int64, which is what we
// want anyway.
func (s *Session) WaitQuiescent(min, max time.Duration) time.Duration {
	start := time.Now()
	deadline := start.Add(max)
	poll := 10 * time.Millisecond
	for {
		now := time.Now()
		if now.After(deadline) {
			s.artifacts.logEvent("wait", fmt.Sprintf("max-reached min=%s max=%s elapsed=%s", min, max, now.Sub(start)))
			return now.Sub(start)
		}
		idleSince := time.Unix(0, s.lastActivity.Load())
		if idleSince.Before(start) {
			idleSince = start
		}
		if now.Sub(idleSince) >= min {
			s.artifacts.logEvent("wait", fmt.Sprintf("quiescent min=%s elapsed=%s", min, now.Sub(start)))
			return now.Sub(start)
		}
		time.Sleep(poll)
	}
}

// Settle is the canonical "wait for the picker to finish painting"
// helper, with the default tuning encoded in defaultQuiescentMin /
// defaultQuiescentMax.
func (s *Session) Settle() time.Duration {
	return s.WaitQuiescent(defaultQuiescentMin, defaultQuiescentMax)
}

// WaitFor polls the screen until pred returns true or `timeout`
// elapses. On timeout, the test is failed via t.Fatalf with a screen
// dump for post-mortem. `label` is included in the failure message
// and the events.jsonl log to make stalls easy to grep for.
//
// This is the canonical primitive for "wait until X is on screen";
// prefer it over WaitQuiescent + Screen.Contains() pairs in scenario
// code (REVIEW-FINDINGS F1).
func (s *Session) WaitFor(label string, timeout time.Duration, pred func(Screen) bool) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pred(s.Screen()) {
			s.artifacts.logEvent("waitfor.hit", label)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	scr := s.Screen()
	s.artifacts.logEvent("waitfor.miss", label)
	s.t.Fatalf("harness: WaitFor(%q) timed out after %s\nscreen:\n%s", label, timeout, scr.Dump())
}

// Screen snapshots the parser state and returns a Screen view. The
// returned value is a snapshot — calling Screen() twice yields
// independent values, which lets a test assert on a stable view even
// while the parser continues processing background bytes.
func (s *Session) Screen() Screen {
	return snapshotVT(s.term)
}

// DebugRaw returns a printable preview of the raw pty bytes captured
// so far. Intended for use from tests when a Screen() assertion
// fails and the caller wants to see exactly what the parser was fed.
// Not exposed via the Screen interface because it is not a snapshot
// of the parser state — it is the *input* to the parser.
func (s *Session) DebugRaw() string {
	s.artifacts.mu.Lock()
	raw := make([]byte, len(s.artifacts.raw))
	copy(raw, s.artifacts.raw)
	s.artifacts.mu.Unlock()
	out := strings.ReplaceAll(string(raw), "\x1b", "\\e")
	out = strings.ReplaceAll(out, "\r", "\\r")
	out = strings.ReplaceAll(out, "\x00", "\\0")
	return out
}

// WaitExit blocks until the child zsh process exits or `max` elapses.
// Reads the result of the single waiter goroutine started in
// NewSession — never calls cmd.Wait itself, so it cannot race with
// cleanup's reaper (REVIEW-FINDINGS F5).
func (s *Session) WaitExit(max time.Duration) error {
	select {
	case <-s.exitCh:
		if v := s.exitErr.Load(); v != nil {
			return v.(errBox).err
		}
		return nil
	case <-time.After(max):
		return fmt.Errorf("harness: WaitExit timed out after %s", max)
	}
}

// cleanup runs at the end of the test. It is registered via t.Cleanup
// in NewSession so it fires automatically; calling Close() directly is
// also fine but not required.
func (s *Session) cleanup() {
	s.closeOnce.Do(func() {
		// Cancel the reader context and close the pty master to wake
		// any blocked Read. closing the pty also sends SIGHUP to the
		// child zsh, which is how the legacy expect harness's `exit`
		// branch eventually wins anyway.
		s.cancel()
		_ = s.ptmx.Close()

		// Best-effort terminate the child if it's still running.
		// The waiter goroutine in NewSession is the sole owner of
		// cmd.Wait — cleanup never calls Wait itself (F5).
		// We wait up to 500 ms for SIGHUP-via-ptmx-close to drain;
		// failing that, SIGKILL and then trust the waiter to reap.
		if s.cmd.Process != nil {
			select {
			case <-s.exitCh:
			case <-time.After(500 * time.Millisecond):
				_ = s.cmd.Process.Kill()
				<-s.exitCh
			}
		}

		// Wait for the reader to fully drain before we attempt to
		// snapshot the final screen — otherwise the dump would race
		// with an in-flight Write into the parser.
		s.readWG.Wait()

		// Snapshot for the artifact dump.
		final := snapshotVT(s.term).Dump()
		s.finalScreen.Store(final)

		s.artifacts.logEvent("session.close", fmt.Sprintf("failed=%t", s.t.Failed()))

		// Only write artifacts on failure — the design explicitly
		// scopes them to that case (DESIGN.md decision 12).
		if s.t.Failed() {
			s.artifacts.writeDumps(s.artifactRoot, final)
		}
	})
}

// shortBytes turns a byte slice into a stable, printable preview for
// the events log. Control bytes are rendered as Go-style escapes; long
// payloads are truncated to keep events.jsonl readable.
func shortBytes(b []byte) string {
	const max = 64
	if len(b) <= max {
		return strings.ReplaceAll(strings.ReplaceAll(string(b), "\x1b", "\\e"), "\r", "\\r")
	}
	return strings.ReplaceAll(strings.ReplaceAll(string(b[:max]), "\x1b", "\\e"), "\r", "\\r") + "..."
}
