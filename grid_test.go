package streak

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGrid_SetGet(t *testing.T) {
	g := NewGrid([]string{"Init", "Services"}, 3)
	if g.Rows() != 2 || g.Cols() != 3 {
		t.Fatalf("dims %dx%d", g.Rows(), g.Cols())
	}
	if err := g.Set(1, 2, Done); err != nil {
		t.Fatal(err)
	}
	if got, _ := g.Get(1, 2); got != Done {
		t.Fatalf("got %v", got)
	}
	if got, _ := g.Get(0, 0); got != Pending {
		t.Fatalf("default must be Pending, got %v", got)
	}
}

func TestGrid_Bounds(t *testing.T) {
	g := NewGrid([]string{"A"}, 2)
	for _, rc := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 2}} {
		if err := g.Set(rc[0], rc[1], Done); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Set%v: got %v want ErrOutOfRange", rc, err)
		}
		if _, err := g.Get(rc[0], rc[1]); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Get%v: got %v", rc, err)
		}
	}
}

func TestGrid_Worst(t *testing.T) {
	g := NewGrid([]string{"A", "B"}, 2)
	for r := 0; r < 2; r++ {
		for c := 0; c < 2; c++ {
			_ = g.Set(r, c, Done)
		}
	}
	if g.Worst() != Done {
		t.Fatal("all done must be Done")
	}
	_ = g.Set(1, 1, Warning)
	if g.Worst() != Warning {
		t.Fatal("expected Warning")
	}
	_ = g.Set(0, 0, Error)
	if g.Worst() != Error {
		t.Fatal("expected Error")
	}
}

func TestGrid_Labels_Copy(t *testing.T) {
	in := []string{"A"}
	g := NewGrid(in, 1)
	in[0] = "Z"
	if g.Label(0) != "A" {
		t.Fatal("NewGrid must copy labels")
	}
	if g.Label(7) != "" {
		t.Fatal("out-of-range label must be empty")
	}
}

func TestGrid_Clone_IsDeep(t *testing.T) {
	g := NewGrid([]string{"A"}, 1)
	c := g.Clone()
	_ = g.Set(0, 0, Error)
	if s, _ := c.Get(0, 0); s != Pending {
		t.Fatal("clone must not share cells")
	}
}

func TestGrid_RowMessage(t *testing.T) {
	g := NewGrid([]string{"A", "B"}, 2)
	if err := g.SetRowMessage(1, Message{Text: "working", Status: Running}); err != nil {
		t.Fatal(err)
	}
	if got := g.RowMessage(1); got.Text != "working" || got.Status != Running {
		t.Fatalf("got %+v", got)
	}
	if got := g.RowMessage(0); got != (Message{}) {
		t.Fatalf("unset row must be zero, got %+v", got)
	}
	if got := g.RowMessage(9); got != (Message{}) {
		t.Fatalf("out of range must be zero, got %+v", got)
	}
	if err := g.SetRowMessage(9, Message{}); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v", err)
	}
}

func TestGrid_RowDone(t *testing.T) {
	g := NewGrid([]string{"A"}, 3)
	if g.RowDone(0) {
		t.Fatal("pending row is not done")
	}
	_ = g.Set(0, 0, Done)
	_ = g.Set(0, 1, Error)
	_ = g.Set(0, 2, Running)
	if g.RowDone(0) {
		t.Fatal("running cell means not done")
	}
	_ = g.Set(0, 2, Warning)
	if !g.RowDone(0) {
		t.Fatal("done, error and warning cells are all terminal")
	}
	if g.RowDone(5) {
		t.Fatal("out of range is never done")
	}
}

func TestGrid_Clone_CopiesRowMessages(t *testing.T) {
	g := NewGrid([]string{"A"}, 1)
	_ = g.SetRowMessage(0, Message{Text: "x"})
	c := g.Clone()
	_ = g.SetRowMessage(0, Message{Text: "y"})
	if c.RowMessage(0).Text != "x" {
		t.Fatal("clone must copy row messages")
	}
}

func TestGrid_Notes_And_Issues(t *testing.T) {
	g := NewGrid([]string{"Init", "Activation"}, 3)
	if err := g.SetNote(9, 0, "x"); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v", err)
	}
	// The error on the second row is recorded before the warning on the
	// first row: Issues must follow that chronology, not row-major order.
	_ = g.Set(1, 1, Error)
	_ = g.SetNote(1, 1, "Health checks failed")
	_ = g.Set(0, 2, Warning)
	_ = g.SetNote(0, 2, "Telemetry endpoint slow")
	_ = g.Set(0, 0, Done)
	_ = g.SetNote(0, 0, "note on a done cell is not an issue")
	_ = g.Set(1, 2, Error) // error without a note: no issue line

	if got := g.Note(1, 1); got != "Health checks failed" {
		t.Fatalf("got %q", got)
	}
	if got := g.Note(5, 5); got != "" {
		t.Fatalf("out of range note must be empty, got %q", got)
	}
	want := []Issue{
		{Row: 1, Col: 1, Status: Error, Text: "Health checks failed"},
		{Row: 0, Col: 2, Status: Warning, Text: "Telemetry endpoint slow"},
	}
	if diff := cmp.Diff(want, g.Issues()); diff != "" {
		t.Fatal(diff)
	}
}

func TestGrid_Issues_RenotedCellKeepsItsSlot(t *testing.T) {
	g := NewGrid([]string{"A"}, 3)
	_ = g.Set(0, 0, Error)
	_ = g.SetNote(0, 0, "first")
	_ = g.Set(0, 1, Warning)
	_ = g.SetNote(0, 1, "second")
	_ = g.SetNote(0, 0, "first, updated")
	got := g.Issues()
	if len(got) != 2 || got[0].Text != "first, updated" || got[1].Text != "second" {
		t.Fatalf("updating a note must not reorder the list, got %+v", got)
	}
}

func TestGrid_Clone_CopiesNotes(t *testing.T) {
	g := NewGrid([]string{"A"}, 1)
	_ = g.SetNote(0, 0, "x")
	c := g.Clone()
	_ = g.SetNote(0, 0, "y")
	if c.Note(0, 0) != "x" {
		t.Fatal("clone must copy notes")
	}
}
