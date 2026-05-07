package tty

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/charmbracelet/x/ansi"
)

// Probe asks the terminal for its current cursor position via the DSR
// query (`\e[6n`) and parses the `\e[<row>;<col>R` response.
//
// Must be called *after* the TTY has entered raw mode — otherwise the
// terminal echoes the response into the application input.
type Probe struct {
	tty *TTY
}

// NewProbe wraps a TTY for cursor-position queries.
func NewProbe(t *TTY) *Probe {
	return &Probe{tty: t}
}

// Cursor returns the current cursor position as 1-indexed (row, col)
// matching the protocol's own indexing, and any leftover bytes the
// probe read alongside the DSR response. The query is bounded by the
// passed deadline; callers typically supply a 250 ms timeout.
//
// Leftover semantics: when the user types something between Ctrl-R
// and the picker's first render — a fast-typing case that's
// completely normal at the human-keystroke timescale — those bytes
// arrive at the TTY input ahead of the DSR response. The probe's
// read loop sees them, mixed with the response. Without a leftover
// channel, those bytes would be silently consumed and the user's
// `^R git`-style input would lose its `g`/`i`/`t`. The returned
// leftover string contains the non-DSR bytes (pre-CSI prefix and
// post-`R` suffix concatenated) so the caller can re-feed them into
// the regular keystream parser.
//
// On timeout, leftover is also populated via the TimeoutError's
// Leftover field; the function-level return is empty in that case.
//
// Implementation: instead of relying on os.File.SetReadDeadline (which
// is unreliable on /dev/tty in some kernels — notably docker's pty
// emulation) we drive the read with unix.Poll directly. Poll honors
// the absolute timeout we pass it byte-for-byte regardless of the
// underlying file's blocking mode.
//
// The loop also requires `\x1b[<...>R`, not merely a stray `R`, so a
// user who types `R` before the response doesn't break out early.
func (p *Probe) Cursor(ctx context.Context, timeout time.Duration) (row, col int, leftover string, err error) {
	if _, err = io.WriteString(p.tty.Writer(), ansi.RequestCursorPositionReport); err != nil {
		return 0, 0, "", fmt.Errorf("write DSR: %w", err)
	}

	deadline := time.Now().Add(timeout)
	fd := int(p.tty.file.Fd())

	var buf [64]byte
	var resp strings.Builder
	for {
		select {
		case <-ctx.Done():
			return 0, 0, "", ctx.Err()
		default:
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, 0, "", &TimeoutError{
				Cause:    fmt.Errorf("read /dev/tty: i/o timeout"),
				Leftover: resp.String(),
			}
		}

		// poll() with the remaining timeout. A return of 0 means the
		// timeout expired with no readable data; >0 means the fd is
		// readable now and a Read will not block.
		pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		nready, perr := unix.Poll(pfd, int(remaining/time.Millisecond))
		if perr != nil {
			// EINTR — interrupted by a signal — is a normal poll
			// outcome; loop and recompute the remaining time.
			if perr == unix.EINTR {
				continue
			}
			return 0, 0, "", fmt.Errorf("poll /dev/tty: %w", perr)
		}
		if nready == 0 || pfd[0].Revents&unix.POLLIN == 0 {
			return 0, 0, "", &TimeoutError{
				Cause:    fmt.Errorf("read /dev/tty: i/o timeout"),
				Leftover: resp.String(),
			}
		}

		n, rerr := unix.Read(fd, buf[:])
		if n > 0 {
			resp.Write(buf[:n])
			// Only break once we have a CSI introducer followed by an
			// 'R' — guards against a user-typed bare 'R' before the
			// response prematurely exiting the loop. The CSI form is
			// `\x1b[...R`; a stray `R` is treated as ordinary input.
			if csiStart := strings.Index(resp.String(), "\x1b["); csiStart >= 0 {
				if strings.IndexByte(resp.String()[csiStart:], 'R') >= 0 {
					break
				}
			}
		} else if n == 0 && rerr == nil {
			// EOF or empty read — break to fallback. Only when rerr
			// is also nil; an EINTR with n==0 must continue (handled
			// below).
			return 0, 0, "", &TimeoutError{
				Cause:    fmt.Errorf("read /dev/tty: empty read"),
				Leftover: resp.String(),
			}
		}
		if rerr != nil {
			// EINTR is recoverable: a signal (typically SIGWINCH if
			// the user resizes the terminal during the probe window)
			// interrupted the read between poll returning POLLIN and
			// the read syscall completing. Falling back on every
			// resize would force the picker to render at col=1
			// instead of inline at the prompt — a visible regression
			// for users who happen to resize while pressing Ctrl-R.
			// Loop continues; the deadline check at the top of the
			// iteration enforces the original timeout budget.
			if rerr == unix.EINTR {
				continue
			}
			return 0, 0, "", &TimeoutError{Cause: rerr, Leftover: resp.String()}
		}
	}

	row, col, leftover, err = parseDSRResponse(resp.String())
	return row, col, leftover, err
}

// TimeoutError is returned when the DSR cursor probe does not see an
// 'R' marker within the deadline. Leftover holds any non-DSR bytes
// the probe consumed while waiting; callers should re-feed them into
// the keystream so user input typed before the picker rendered is
// not dropped.
type TimeoutError struct {
	Cause    error
	Leftover string
}

// Error implements error.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("DSR cursor probe timed out: %v", e.Cause)
}

// Unwrap allows errors.Is / errors.As to reach the underlying cause.
func (e *TimeoutError) Unwrap() error { return e.Cause }

// parseDSRResponse extracts (row, col) from a CSI <row>;<col>R reply
// embedded in s. Bytes outside the response payload are returned as
// leftover — pre-CSI bytes (anything before the `\x1b[`) and post-R
// bytes (anything after the matched 'R') concatenated in input order.
//
// Why anchor on `\x1b[` rather than the first `[` byte: a user who
// types `[` immediately after Ctrl-R would otherwise short-circuit
// the `start` search. Anchoring on the CSI introducer keeps the
// parse robust for any printable user input that happens to overlap
// the probe window.
func parseDSRResponse(s string) (row, col int, leftover string, err error) {
	csiStart := strings.Index(s, "\x1b[")
	if csiStart < 0 {
		return 0, 0, s, fmt.Errorf("malformed DSR response %q (no CSI)", s)
	}
	// Search for 'R' only after the CSI introducer so a stray user-
	// typed `R` before the response doesn't drag the end-marker left
	// of the actual reply.
	rRel := strings.IndexByte(s[csiStart:], 'R')
	if rRel < 0 {
		return 0, 0, s, fmt.Errorf("malformed DSR response %q (no R after CSI)", s)
	}
	rAbs := csiStart + rRel
	body := s[csiStart+2 : rAbs] // skip `\x1b[`
	semi := strings.IndexByte(body, ';')
	if semi < 0 {
		return 0, 0, s, fmt.Errorf("malformed DSR response %q (no semicolon)", s)
	}
	rowStr, colStr := body[:semi], body[semi+1:]
	rowVal, rerr := strconv.Atoi(rowStr)
	colVal, cerr := strconv.Atoi(colStr)
	if rerr != nil || cerr != nil {
		return 0, 0, s, fmt.Errorf("malformed DSR response %q (non-numeric)", s)
	}
	leftover = s[:csiStart] + s[rAbs+1:]
	return rowVal, colVal, leftover, nil
}
