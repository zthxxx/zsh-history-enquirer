package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestThrottle_LeadingEdge(t *testing.T) {
	t.Parallel()
	th := NewThrottle(72 * time.Millisecond)
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	// First fire: allowed.
	require.True(t, th.Fire(now))
	// Inside the window: blocked.
	require.False(t, th.Fire(now.Add(10*time.Millisecond)))
	require.False(t, th.Fire(now.Add(71*time.Millisecond)))
	// At/after the window boundary: allowed.
	require.True(t, th.Fire(now.Add(72*time.Millisecond)))
	require.False(t, th.Fire(now.Add(73*time.Millisecond)))
}

func TestThrottle_ZeroInterval(t *testing.T) {
	t.Parallel()
	th := NewThrottle(0)
	now := time.Now()
	require.True(t, th.Fire(now))
	require.True(t, th.Fire(now))
}
