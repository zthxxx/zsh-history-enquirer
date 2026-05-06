package ui

import "time"

// Throttle is a leading-edge time-throttler. The first Fire in any
// `interval` window returns true; subsequent calls inside that window
// return false. After the window elapses, the next Fire is allowed
// again.
//
// Used by the program loop to throttle frame writes to ~14 fps,
// matching the legacy 72 ms throttle.
type Throttle struct {
	interval time.Duration
	lastFire time.Time
}

// NewThrottle constructs a throttle with the given window. Setting
// interval to zero disables throttling (every Fire returns true).
func NewThrottle(interval time.Duration) *Throttle {
	return &Throttle{interval: interval}
}

// Fire reports whether enough time has passed since the last allowed
// Fire that we should fire again. Pass `now` (rather than reading the
// clock internally) so tests can drive it deterministically.
func (t *Throttle) Fire(now time.Time) bool {
	if t.interval <= 0 {
		t.lastFire = now
		return true
	}
	if t.lastFire.IsZero() || now.Sub(t.lastFire) >= t.interval {
		t.lastFire = now
		return true
	}
	return false
}
