package tty

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/zthxxx/zsh-history-enquirer/internal/ansi"
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
// matching the protocol's own indexing. The query is bounded by the
// passed deadline; callers typically supply a 250 ms timeout.
//
// Returns an error if the terminal does not respond within the
// deadline (some emulators silently ignore DSR). On timeout, any
// non-DSR bytes that were read are returned via the Leftover field
// of the error so the caller can replay them through the regular
// keystream parser.
//
// Implementation: instead of relying on os.File.SetReadDeadline (which
// is unreliable on /dev/tty in some kernels — notably docker's pty
// emulation) we drive the read with unix.Poll directly. Poll honours
// the absolute timeout we pass it byte-for-byte regardless of the
// underlying file's blocking mode.
func (p *Probe) Cursor(ctx context.Context, timeout time.Duration) (row, col int, err error) {
	if _, err = io.WriteString(p.tty.Writer(), ansi.DSRCursor); err != nil {
		return 0, 0, fmt.Errorf("write DSR: %w", err)
	}

	deadline := time.Now().Add(timeout)
	fd := int(p.tty.file.Fd())

	var buf [64]byte
	var resp strings.Builder
	for {
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, 0, &TimeoutError{
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
			return 0, 0, fmt.Errorf("poll /dev/tty: %w", perr)
		}
		if nready == 0 || pfd[0].Revents&unix.POLLIN == 0 {
			return 0, 0, &TimeoutError{
				Cause:    fmt.Errorf("read /dev/tty: i/o timeout"),
				Leftover: resp.String(),
			}
		}

		n, rerr := unix.Read(fd, buf[:])
		if n > 0 {
			resp.Write(buf[:n])
			if strings.IndexByte(resp.String(), 'R') >= 0 {
				break
			}
		} else if n == 0 {
			// EOF or empty read — break to fallback.
			return 0, 0, &TimeoutError{
				Cause:    fmt.Errorf("read /dev/tty: empty read"),
				Leftover: resp.String(),
			}
		}
		if rerr != nil {
			return 0, 0, &TimeoutError{Cause: rerr, Leftover: resp.String()}
		}
	}

	row, col, err = parseDSRResponse(resp.String())
	return row, col, err
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

// parseDSRResponse extracts row/col from a CSI <row>;<col>R reply.
// Bytes outside the bracketed payload are tolerated (terminals
// occasionally emit one or two stray bytes ahead of the response).
func parseDSRResponse(s string) (int, int, error) {
	start := strings.Index(s, "[")
	end := strings.IndexByte(s, 'R')
	if start < 0 || end < 0 || end <= start {
		return 0, 0, fmt.Errorf("malformed DSR response %q", s)
	}
	body := s[start+1 : end]
	semi := strings.IndexByte(body, ';')
	if semi < 0 {
		return 0, 0, fmt.Errorf("malformed DSR response %q (no semicolon)", s)
	}
	rowStr, colStr := body[:semi], body[semi+1:]
	row, rerr := strconv.Atoi(rowStr)
	col, cerr := strconv.Atoi(colStr)
	if rerr != nil || cerr != nil {
		return 0, 0, fmt.Errorf("malformed DSR response %q (non-numeric)", s)
	}
	return row, col, nil
}
