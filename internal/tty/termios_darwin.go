//go:build darwin

package tty

import "golang.org/x/sys/unix"

// On Darwin, the termios ioctls are TIOCGETA / TIOCSETA. The TCGETS
// constants in the Linux file are not portable.
const (
	getTermiosReq = unix.TIOCGETA
	setTermiosReq = unix.TIOCSETA
)
