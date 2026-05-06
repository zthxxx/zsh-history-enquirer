// Package tty owns the controlling-terminal handle.
//
// The widget invokes us inside `BUFFER=$(zsh-history-enquirer …)`, so
// stdout is a pipe — useless for drawing. We open /dev/tty directly
// and route every UI byte (reads and writes) through that fd. See
// docs/design/30-tty.md for the full rationale.
package tty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"go.uber.org/fx"
	"golang.org/x/sys/unix"
)

// TTY is a thin wrapper around an open /dev/tty fd. It implements
// io.Reader and io.Writer so the rest of the code can treat it like
// a stdio stream.
type TTY struct {
	file       *os.File
	savedTerm  *unix.Termios
	rawEntered bool
}

// Reader returns an io.Reader bound to the controlling terminal. When
// the process is run interactively this is /dev/tty; when invoked
// from a subprocess inside `$(...)` this is still /dev/tty, since
// command substitution only redirects stdout.
func (t *TTY) Reader() io.Reader { return t.file }

// Writer returns an io.Writer bound to the controlling terminal.
func (t *TTY) Writer() io.Writer { return t.file }

// File exposes the underlying *os.File for syscalls that need a real
// fd (e.g. ioctls). Callers must not close this file directly — use
// (*TTY).Close.
func (t *TTY) File() *os.File { return t.file }

// Open opens /dev/tty for reading and writing. Used outside of the
// fx graph (e.g. in tests).
func Open() (*TTY, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/tty: %w", err)
	}
	saved, err := unix.IoctlGetTermios(int(f.Fd()), getTermiosReq)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("ioctl GET termios: %w", err)
	}
	return &TTY{file: f, savedTerm: saved}, nil
}

// NewDevTTY is the fx-injected constructor. It registers an OnStop
// hook that:
//   - leaves bracketed paste mode (if entered)
//   - restores the original termios
//   - closes the fd
//
// This guarantees that even a panic upstream still leaves the user's
// terminal in a usable state.
func NewDevTTY(lc fx.Lifecycle) (*TTY, error) {
	t, err := Open()
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return t.Close()
		},
	})
	return t, nil
}

// Close restores termios, exits raw mode if entered, and closes the
// underlying fd. Safe to call multiple times.
func (t *TTY) Close() error {
	if t.file == nil {
		return nil
	}
	var err error
	if t.rawEntered {
		err = t.LeaveRaw()
	}
	if cerr := t.file.Close(); cerr != nil && err == nil && !errors.Is(cerr, os.ErrClosed) {
		err = cerr
	}
	t.file = nil
	return err
}

// Size returns the current terminal dimensions.
func (t *TTY) Size() (rows, cols int, err error) {
	ws, err := unix.IoctlGetWinsize(int(t.file.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, fmt.Errorf("ioctl GET winsize: %w", err)
	}
	return int(ws.Row), int(ws.Col), nil
}
