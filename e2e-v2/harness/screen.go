package harness

import (
	"strings"
	"unicode"

	"github.com/hinshun/vt10x"
)

// Screen exposes a read-only view of the parsed terminal state.
//
// All methods take a defensive snapshot under vt10x's mutex so they
// are safe to call from a test goroutine while the harness reader
// goroutine continues to feed bytes into the parser.
type Screen interface {
	// Contains reports whether the printable rune contents of any
	// row contain substr (as one continuous span). Trailing spaces
	// are trimmed before the search so row-padding does not split
	// substrings that span the last printed column.
	Contains(substr string) bool

	// RowContains reports whether row `row` contains substr after
	// printable-rune extraction. Row is 0-indexed from the top.
	RowContains(row int, substr string) bool

	// Dump returns the full grid as printable runes, with one row
	// per line and a single '\n' separator. Unrendered cells emit
	// a space. The output has no trailing newline.
	Dump() string

	// Cursor returns the parser's current cursor position as
	// (row, col) measured from the top-left, both 0-indexed. The
	// values are advisory — the picker hides the cursor most of
	// the time, but they help when debugging.
	Cursor() (row, col int)

	// Size returns (rows, cols).
	Size() (rows, cols int)
}

// vtScreen is a snapshot of the vt10x state at a single point in time.
// All ops on it are pure string/array operations — the parser may keep
// running on a separate goroutine without invalidating this snapshot.
type vtScreen struct {
	rows int
	cols int
	grid []string
	curR int
	curC int
}

// snapshotVT pulls the current state out of the parser, with its mutex
// held, into a plain-data struct that is safe to inspect concurrently
// with continued reads on the pty master.
func snapshotVT(t vt10x.Terminal) *vtScreen {
	t.Lock()
	defer t.Unlock()

	cols, rows := t.Size()
	cur := t.Cursor()

	grid := make([]string, rows)
	var row strings.Builder
	for y := 0; y < rows; y++ {
		row.Reset()
		for x := 0; x < cols; x++ {
			g := t.Cell(x, y)
			r := g.Char
			if r == 0 || !unicode.IsPrint(r) {
				r = ' '
			}
			row.WriteRune(r)
		}
		grid[y] = row.String()
	}
	return &vtScreen{
		rows: rows,
		cols: cols,
		grid: grid,
		curR: cur.Y,
		curC: cur.X,
	}
}

func (s *vtScreen) Contains(substr string) bool {
	for _, row := range s.grid {
		if strings.Contains(row, substr) {
			return true
		}
		// Also try a trimmed view so a substring that ends at the
		// last printed column is not artificially split by trailing
		// spaces. (vt10x reports the full column width regardless of
		// whether the cell was written.)
		if strings.Contains(strings.TrimRight(row, " "), substr) {
			return true
		}
	}
	return false
}

func (s *vtScreen) RowContains(row int, substr string) bool {
	if row < 0 || row >= len(s.grid) {
		return false
	}
	return strings.Contains(s.grid[row], substr) ||
		strings.Contains(strings.TrimRight(s.grid[row], " "), substr)
}

func (s *vtScreen) Dump() string {
	return strings.Join(s.grid, "\n")
}

func (s *vtScreen) Cursor() (row, col int) {
	return s.curR, s.curC
}

func (s *vtScreen) Size() (rows, cols int) {
	return s.rows, s.cols
}
