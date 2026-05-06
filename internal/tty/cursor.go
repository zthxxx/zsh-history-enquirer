package tty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

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
// deadline (some emulators silently ignore DSR).
func (p *Probe) Cursor(ctx context.Context, timeout time.Duration) (row, col int, err error) {
	if _, err = io.WriteString(p.tty.Writer(), ansi.DSRCursor); err != nil {
		return 0, 0, fmt.Errorf("write DSR: %w", err)
	}

	// Set a read deadline. file.SetReadDeadline works because we wrap
	// the fd in *os.File at Open() time.
	if err = p.tty.file.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, 0, fmt.Errorf("set read deadline: %w", err)
	}
	defer func() {
		_ = p.tty.file.SetReadDeadline(time.Time{})
	}()

	// Read until we see 'R' or hit the deadline. The response we
	// expect is "\e[<row>;<col>R". A poorly-behaved terminal might
	// echo extra bytes — we consume them silently after the 'R'.
	var buf [64]byte
	var resp strings.Builder
	for {
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}

		n, rerr := p.tty.file.Read(buf[:])
		if n > 0 {
			resp.Write(buf[:n])
			if strings.IndexByte(resp.String(), 'R') >= 0 {
				break
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				break
			}
			return 0, 0, fmt.Errorf("read DSR: %w", rerr)
		}
	}

	row, col, err = parseDSRResponse(resp.String())
	return row, col, err
}

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
