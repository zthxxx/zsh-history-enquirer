package tty

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// EnterRaw switches the terminal into a raw mode tuned for our use
// case (no canonical input, no echo, no signal generation, no
// post-processing). The termios state captured at Open() is preserved
// in TTY.savedTerm and restored by LeaveRaw or Close.
//
// We deliberately disable ISIG so that ^C arrives as the byte 0x03
// instead of as a SIGINT — this lets the picker translate it into a
// controlled "cancel preserves input" exit. (See spec/50.)
func (t *TTY) EnterRaw() error {
	if t.rawEntered {
		return nil
	}
	if t.savedTerm == nil {
		return fmt.Errorf("savedTerm is nil; cannot enter raw mode")
	}

	raw := *t.savedTerm

	// Input flags.
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK |
		unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON

	// Output flags — keep OPOST off so newline is exactly LF and we
	// can emit our own \r\n where needed.
	raw.Oflag &^= unix.OPOST

	// Local flags — no echo, no canonical mode, no signals, no extended.
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN

	// Character flags — enforce 8-bit, no parity.
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8

	// Read returns as soon as one byte is available.
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(int(t.file.Fd()), setTermiosReq, &raw); err != nil {
		return fmt.Errorf("ioctl SET termios (raw): %w", err)
	}
	t.rawEntered = true
	return nil
}

// LeaveRaw restores the termios state captured at Open(). Safe to call
// multiple times.
func (t *TTY) LeaveRaw() error {
	if !t.rawEntered || t.savedTerm == nil {
		return nil
	}
	if err := unix.IoctlSetTermios(int(t.file.Fd()), setTermiosReq, t.savedTerm); err != nil {
		return fmt.Errorf("ioctl SET termios (restore): %w", err)
	}
	t.rawEntered = false
	return nil
}
