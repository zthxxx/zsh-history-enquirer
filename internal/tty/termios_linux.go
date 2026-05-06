//go:build linux

package tty

import "golang.org/x/sys/unix"

// On Linux, the canonical termios ioctls are TCGETS / TCSETS.
const (
	getTermiosReq = unix.TCGETS
	setTermiosReq = unix.TCSETS
)
