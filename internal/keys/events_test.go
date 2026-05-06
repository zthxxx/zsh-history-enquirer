package keys

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEventInterface exercises the empty-method type discriminators.
// They have no behavior, but the type-switch boundary is the only
// thing that keeps Reader.Events typed correctly.
func TestEventInterface(t *testing.T) {
	t.Parallel()

	events := []Event{
		RuneEvent{R: 'a'},
		KeyEvent{Key: KeyEnter},
		PasteEvent{Payload: "ls"},
		ResizeEvent{Rows: 24, Cols: 80},
	}

	// Round-trip each event via the interface; if any of these fail,
	// the marker method failed to compile.
	for _, ev := range events {
		ev.event()
	}

	// Type-switch each one — the production update loop relies on this
	// shape, and it's worth pinning the surface.
	for _, ev := range events {
		switch e := ev.(type) {
		case RuneEvent:
			require.Equal(t, 'a', e.R)
		case KeyEvent:
			require.Equal(t, KeyEnter, e.Key)
		case PasteEvent:
			require.Equal(t, "ls", e.Payload)
		case ResizeEvent:
			require.Equal(t, 24, e.Rows)
			require.Equal(t, 80, e.Cols)
		default:
			t.Fatalf("unexpected event type %T", e)
		}
	}
}

// TestKeyConstantsStable pins the iota ordering. Reordering breaks
// downstream switches that compare by value (Update tables, etc.).
func TestKeyConstantsStable(t *testing.T) {
	t.Parallel()

	require.Equal(t, Key(0), KeyUnknown)
	require.Equal(t, Key(1), KeyEnter)
	require.Equal(t, Key(2), KeyEsc)
	require.Equal(t, Key(3), KeyBackspace)
	// Last value — locks the count too.
	require.Equal(t, Key(28), KeyCtrlY)
}
