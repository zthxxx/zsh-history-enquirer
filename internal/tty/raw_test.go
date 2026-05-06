package tty

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// withPty opens a pty pair, replaces the test's /dev/tty look-alike
// with the slave fd, and returns the master so the test can drive
// the responder.
func withPty(t *testing.T) (master *os.File, slave *os.File) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}

	master, slave, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = master.Close()
		_ = slave.Close()
	})
	return master, slave
}

// fromFile fabricates a TTY from an already-open *os.File and puts
// it into raw mode. Used in unit tests where the fd points at a pty
// slave rather than /dev/tty. Without raw mode, the kernel's line
// discipline buffers writes by line and echoes them back, both of
// which break the DSR round-trip we are testing.
func fromFile(f *os.File) (*TTY, error) {
	saved, err := unix.IoctlGetTermios(int(f.Fd()), getTermiosReq)
	if err != nil {
		return nil, err
	}
	t := &TTY{file: f, savedTerm: saved}
	if err := t.EnterRaw(); err != nil {
		return nil, err
	}
	return t, nil
}

func TestProbeCursor_RoundTrip(t *testing.T) {
	t.Parallel()

	master, slave := withPty(t)
	tt, err := fromFile(slave)
	require.NoError(t, err)

	// Goroutine simulating a terminal: read the DSR query and reply
	// with a canned response.
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 64)
		_ = master.SetReadDeadline(time.Now().Add(time.Second))
		n, rerr := master.Read(buf)
		if rerr != nil {
			done <- rerr
			return
		}
		if !bytes.Contains(buf[:n], []byte("\x1b[6n")) {
			done <- errors.New("did not receive DSR query")
			return
		}
		_, werr := io.WriteString(master, "\x1b[7;42R")
		done <- werr
	}()

	probe := NewProbe(tt)
	row, col, err := probe.Cursor(context.Background(), 500*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, 7, row)
	require.Equal(t, 42, col)

	require.NoError(t, <-done)
}
