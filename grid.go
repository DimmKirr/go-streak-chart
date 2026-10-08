package streak

import (
	"errors"
	"sort"
)

// ErrOutOfRange is returned for row or column indexes outside the grid.
var ErrOutOfRange = errors.New("streak: index out of range")

// Grid holds the status of every component cell. It is not safe for
// concurrent use; Loader and teastreak serialize access to it.
type Grid struct {
	labels  []string
	cols    int
	cells   [][]Status
	rowMsgs []Message
	notes   [][]string
	noteSeq [][]int // order in which each cell first received a note; 0 = never
	seq     int
}

// NewGrid creates a grid with one row per label and cols cells per row,
// all Pending.
func NewGrid(rows []string, cols int) *Grid {
	if cols < 0 {
		cols = 0
	}
	g := &Grid{
		labels:  append([]string(nil), rows...),
		cols:    cols,
		cells:   make([][]Status, len(rows)),
		rowMsgs: make([]Message, len(rows)),
		notes:   make([][]string, len(rows)),
		noteSeq: make([][]int, len(rows)),
	}
	for i := range g.cells {
		g.cells[i] = make([]Status, cols)
		g.notes[i] = make([]string, cols)
		g.noteSeq[i] = make([]int, cols)
	}
	return g
}

// Rows returns the number of rows.
func (g *Grid) Rows() int { return len(g.labels) }

// Cols returns the number of cells per row.
func (g *Grid) Cols() int { return g.cols }

// Label returns the row label, or "" when out of range.
func (g *Grid) Label(row int) string {
	if row < 0 || row >= len(g.labels) {
		return ""
	}
	return g.labels[row]
}

func (g *Grid) check(row, col int) error {
	if row < 0 || row >= len(g.labels) || col < 0 || col >= g.cols {
		return ErrOutOfRange
	}
	return nil
}

// Set updates one cell.
func (g *Grid) Set(row, col int, s Status) error {
	if err := g.check(row, col); err != nil {
		return err
	}
	g.cells[row][col] = s
	return nil
}

// Get reads one cell.
func (g *Grid) Get(row, col int) (Status, error) {
	if err := g.check(row, col); err != nil {
		return Pending, err
	}
	return g.cells[row][col], nil
}

// Worst returns the highest-severity status in the grid. An empty or fully
// completed grid reports Done.
func (g *Grid) Worst() Status {
	worst := Done
	for _, r := range g.cells {
		for _, s := range r {
			if s.Severity() > worst.Severity() {
				worst = s
			}
		}
	}
	return worst
}

// Clone returns a deep copy.
func (g *Grid) Clone() *Grid {
	c := NewGrid(g.labels, g.cols)
	for i := range g.cells {
		copy(c.cells[i], g.cells[i])
		copy(c.notes[i], g.notes[i])
		copy(c.noteSeq[i], g.noteSeq[i])
	}
	copy(c.rowMsgs, g.rowMsgs)
	c.seq = g.seq
	return c
}

// SetRowMessage attaches a status line to a row. RowLayout shows it after
// the row's cells until the row is done.
func (g *Grid) SetRowMessage(row int, m Message) error {
	if row < 0 || row >= len(g.labels) {
		return ErrOutOfRange
	}
	g.rowMsgs[row] = m
	return nil
}

// RowMessage returns the row's status line, or the zero Message.
func (g *Grid) RowMessage(row int) Message {
	if row < 0 || row >= len(g.labels) {
		return Message{}
	}
	return g.rowMsgs[row]
}

// RowDone reports whether every cell in the row has reached a terminal
// status (Done, Warning or Error).
func (g *Grid) RowDone(row int) bool {
	if row < 0 || row >= len(g.labels) {
		return false
	}
	for _, s := range g.cells[row] {
		if s == Pending || s == Running {
			return false
		}
	}
	return true
}

// Issue is a note on a cell that ended in Warning or Error.
type Issue struct {
	Row, Col int
	Status   Status
	Text     string
}

// SetNote attaches text to a cell. Notes on Warning and Error cells are
// listed under the rule as issues; notes on other cells are kept but not
// shown.
func (g *Grid) SetNote(row, col int, text string) error {
	if err := g.check(row, col); err != nil {
		return err
	}
	g.notes[row][col] = text
	if g.noteSeq[row][col] == 0 {
		g.seq++
		g.noteSeq[row][col] = g.seq
	}
	return nil
}

// Note returns the cell's note, or "" when unset or out of range.
func (g *Grid) Note(row, col int) string {
	if g.check(row, col) != nil {
		return ""
	}
	return g.notes[row][col]
}

// Issues returns the notes of Warning and Error cells in the order the notes
// were first recorded, so the list reads as a chronological log.
func (g *Grid) Issues() []Issue {
	var out []Issue
	for r, row := range g.cells {
		for c, s := range row {
			if (s == Warning || s == Error) && g.notes[r][c] != "" {
				out = append(out, Issue{Row: r, Col: c, Status: s, Text: g.notes[r][c]})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return g.noteSeq[out[i].Row][out[i].Col] < g.noteSeq[out[j].Row][out[j].Col]
	})
	return out
}
