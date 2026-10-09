package streak

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock lets tests drive Loader ticks deterministically. Tick blocks
// until the loader has processed the tick.
type fakeClock struct {
	ch    chan time.Time
	acked chan struct{}
}

func newFakeClock() *fakeClock {
	return &fakeClock{ch: make(chan time.Time), acked: make(chan struct{})}
}

func (f *fakeClock) Ticker(time.Duration) (<-chan time.Time, func()) { return f.ch, func() {} }
func (f *fakeClock) tickDone()                                       { f.acked <- struct{}{} }
func (f *fakeClock) Tick() {
	f.ch <- time.Now()
	<-f.acked
}

func TestLoader_Plain(t *testing.T) {
	var buf bytes.Buffer
	g := NewGrid([]string{"Init", "Services"}, 2)
	l := NewLoader(g, WithWriter(&buf), WithPlain(true))
	l.Start()
	l.Message("Booting", Running)
	if err := l.Set(0, 0, Done); err != nil {
		t.Fatal(err)
	}
	l.Finish("Ready", Done)
	got := buf.String()
	for _, want := range []string{"[running] Booting\n", "[done] Init #1\n", "[done] Ready\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatal("plain mode must not emit escape codes")
	}
}

func TestLoader_SetOutOfRange(t *testing.T) {
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(io.Discard), WithPlain(true))
	if err := l.Set(5, 5, Done); !errors.Is(err, ErrOutOfRange) {
		t.Fatal(err)
	}
}

func TestLoader_NonFileWriterIsPlain(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf))
	l.Start()
	l.Message("x", Running)
	l.Finish("y", Done)
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("bytes.Buffer is not a TTY; expected plain output")
	}
}

func TestLoader_FinishIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(true))
	l.Start()
	l.Finish("y", Done)
	l.Finish("again", Error)
	if strings.Contains(buf.String(), "again") {
		t.Fatal("second Finish must be a no-op")
	}
}

func TestLoader_TTY_RedrawSequence(t *testing.T) {
	var buf bytes.Buffer
	fc := newFakeClock()
	g := NewGrid([]string{"A"}, 2) // frame = row + rule + message = 3 lines
	l := NewLoader(g, WithWriter(&buf), WithPlain(false), withClock(fc))
	l.Start()
	l.Message("work", Running)
	fc.Tick()
	first := buf.String()
	if first == "" {
		t.Fatal("first tick must draw a frame")
	}
	if strings.Contains(first, "\x1b[2A") {
		t.Fatal("first frame must not move the cursor up")
	}
	fc.Tick()
	second := buf.String()[len(first):]
	if !strings.HasPrefix(second, "\r\x1b[2A\x1b[J") {
		t.Fatalf("redraw must start with CR, cursor up 2, erase below; got %q", second)
	}
	l.Finish("ok", Done)
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatal("Finish must end with a newline")
	}
	if last := lastFrame(out); !strings.Contains(last, "ok") || strings.Contains(last, "work") {
		t.Fatalf("final frame must show finish text only, got %q", last)
	}
}

func TestLoader_Pulse_BouncesBetween1And4(t *testing.T) {
	fc := newFakeClock()
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(io.Discard), WithPlain(false), withClock(fc))
	l.Start()
	l.Message("w", Running)
	var seen []Level
	for i := 0; i < 8; i++ {
		fc.Tick()
		seen = append(seen, l.level())
	}
	want := []Level{2, 3, 4, 3, 2, 1, 2, 3}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("got %v want %v", seen, want)
	}
}

func TestLoader_Pulse_StopsWhenNotRunning(t *testing.T) {
	fc := newFakeClock()
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(io.Discard), WithPlain(false), withClock(fc))
	l.Start()
	l.Message("w", Warning)
	fc.Tick()
	fc.Tick()
	if l.level() != 1 {
		t.Fatalf("non-running message must not pulse, got %d", l.level())
	}
}

func TestLoader_FinishWithoutTick_DrawsOnce(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(false), withClock(newFakeClock()))
	l.Start()
	l.Finish("done", Done)
	out := buf.String()
	if strings.Contains(out, "\x1b[J") {
		t.Fatal("no prior frame means no erase sequence")
	}
	if !strings.Contains(StripANSI(out), "done") {
		t.Fatal("final frame missing")
	}
}

func TestLoader_TTY_SetDoesNotDrawImmediately(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(false), withClock(newFakeClock()))
	l.Start()
	_ = l.Set(0, 0, Done)
	l.Message("m", Running)
	if buf.Len() != 0 {
		t.Fatal("TTY mode draws only on ticks and Finish")
	}
}

func TestLoader_RowMessage_Plain(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"Init", "Services"}, 2), WithWriter(&buf), WithPlain(true))
	l.Start()
	if err := l.RowMessage(1, "Processing Services: Auth", Running); err != nil {
		t.Fatal(err)
	}
	if err := l.RowMessage(7, "x", Running); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v", err)
	}
	if want := "[running] Services: Processing Services: Auth\n"; !strings.Contains(buf.String(), want) {
		t.Fatalf("missing %q in %q", want, buf.String())
	}
}

func TestLoader_RowMessage_TTY_RowLayoutFrame(t *testing.T) {
	var buf bytes.Buffer
	fc := newFakeClock()
	th := DefaultTheme()
	th.Layout = RowLayout
	l := NewLoader(NewGrid([]string{"Init"}, 2), WithWriter(&buf), WithPlain(false), WithTheme(th), withClock(fc))
	l.Start()
	_ = l.Set(0, 0, Running)
	_ = l.RowMessage(0, "Processing Init: Config", Running)
	fc.Tick()
	frame := StripANSI(buf.String())
	if !strings.Contains(frame, "Init  ■ ■  Processing Init: Config") {
		t.Fatalf("row layout frame missing row text: %q", frame)
	}
	if strings.Count(frame, "\n") != 1 {
		t.Fatalf("row layout frame must be rows + rule, got %q", frame)
	}
	_ = l.Set(0, 0, Done)
	_ = l.Set(0, 1, Done)
	l.Finish("", Done)
	frames := strings.Split(StripANSI(buf.String()), "\r")
	if last := frames[len(frames)-1]; strings.Contains(last, "Processing") {
		t.Fatalf("done row must drop its text in the final frame: %q", last)
	}
}

func TestLoader_Plain_FinishWithEmptyTextPrintsNothing(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(true))
	l.Start()
	l.Finish("", Done)
	if buf.Len() != 0 {
		t.Fatalf("empty finish text must not log a bare status line, got %q", buf.String())
	}
}

func TestLoader_ConcurrentRowsAreSerialized(t *testing.T) {
	const rows, cols = 3, 5
	labels := []string{"Init", "Services", "Activation"}
	fc := newFakeClock()
	th := DefaultTheme()
	th.Layout = RowLayout
	var buf bytes.Buffer
	l := NewLoader(NewGrid(labels, cols), WithWriter(&buf), WithPlain(false), WithTheme(th), withClock(fc))
	l.Start()

	var wg sync.WaitGroup
	for r := 0; r < rows; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for c := 0; c < cols; c++ {
				if err := l.Set(r, c, Running); err != nil {
					t.Error(err)
				}
				if err := l.RowMessage(r, labels[r]+" step", Running); err != nil {
					t.Error(err)
				}
				if err := l.Set(r, c, Done); err != nil {
					t.Error(err)
				}
			}
		}(r)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	for {
		select {
		case <-done:
			goto finished
		default:
			fc.Tick() // keep redrawing while rows mutate the grid
		}
	}
finished:
	l.Finish("", Done)

	g := l.Snapshot()
	for r := 0; r < rows; r++ {
		if !g.RowDone(r) {
			t.Errorf("row %d not done after all goroutines joined", r)
		}
		for c := 0; c < cols; c++ {
			if s, _ := g.Get(r, c); s != Done {
				t.Errorf("cell %d,%d = %v, want Done", r, c, s)
			}
		}
	}
	if last := lastFrame(buf.String()); strings.Contains(last, "step") {
		t.Fatalf("final frame must drop all row text: %q", last)
	}
}

func TestLoader_Snapshot_IsIndependentCopy(t *testing.T) {
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(io.Discard), WithPlain(true))
	snap := l.Snapshot()
	_ = l.Set(0, 0, Error)
	if s, _ := snap.Get(0, 0); s != Pending {
		t.Fatal("snapshot must not observe later writes")
	}
	if s, _ := l.Snapshot().Get(0, 0); s != Error {
		t.Fatal("fresh snapshot must see the write")
	}
}

func lastFrame(out string) string {
	stripped := StripANSI(out)
	// Frames are separated by \r (the redraw prefix). Lines within a frame
	// use \r\n, so split on lone \r (not followed by \n) to isolate frames,
	// then take the last non-empty one.
	stripped = strings.ReplaceAll(stripped, "\r\n", "\n")
	frames := strings.Split(stripped, "\r")
	for i := len(frames) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(frames[i]); s != "" {
			return frames[i]
		}
	}
	return stripped
}

func TestLoader_WarnFail_Plain(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"Init", "Activation"}, 2), WithWriter(&buf), WithPlain(true))
	l.Start()
	if err := l.Warn(0, 1, "Telemetry endpoint slow"); err != nil {
		t.Fatal(err)
	}
	if err := l.Fail(1, 0, "Health checks failed"); err != nil {
		t.Fatal(err)
	}
	if err := l.Fail(9, 0, "x"); !errors.Is(err, ErrOutOfRange) {
		t.Fatalf("got %v", err)
	}
	for _, want := range []string{"[warning] Init #2: Telemetry endpoint slow\n", "[error] Activation #1: Health checks failed\n"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q in %q", want, buf.String())
		}
	}
	g := l.Snapshot()
	if s, _ := g.Get(1, 0); s != Error {
		t.Fatal("Fail must set the cell to Error")
	}
	if len(g.Issues()) != 2 {
		t.Fatalf("want 2 issues, got %+v", g.Issues())
	}
}

func TestLoader_Issues_StayInFinalFrame(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(false), withClock(newFakeClock()))
	l.Start()
	_ = l.Fail(0, 0, "Build of ABC failed")
	l.Finish("Loading finished with errors", Error)
	last := lastFrame(buf.String())
	if !strings.Contains(last, "■ Build of ABC failed\n■ Loading finished with errors") {
		t.Fatalf("final frame must keep the issue above the footer: %q", last)
	}
}

func TestLoader_WithIssuesNone(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(false), WithIssues(NoIssues), withClock(newFakeClock()))
	l.Start()
	_ = l.Fail(0, 0, "Build of ABC failed")
	l.Finish("done", Error)
	if strings.Contains(lastFrame(buf.String()), "Build of ABC failed") {
		t.Fatalf("issue log disabled, got %q", lastFrame(buf.String()))
	}
}

func TestLoader_WithWidth_TruncatesFrame(t *testing.T) {
	var buf bytes.Buffer
	th := DefaultTheme()
	th.Layout = RowLayout
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(false), WithTheme(th), WithWidth(24), withClock(newFakeClock()))
	l.Start()
	_ = l.Fail(0, 0, "a very long note that certainly exceeds twenty-four columns")
	l.Finish("", Error)
	for _, line := range strings.Split(StripANSI(lastFrame(buf.String())), "\n") {
		if w := len([]rune(line)); w > 24 {
			t.Fatalf("frame line wider than WithWidth: %d %q", w, line)
		}
	}
}

func TestLoader_MutatorsAfterFinishReturnErrFinished(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&buf), WithPlain(true))
	l.Start()
	l.Finish("done", Done)
	n := buf.Len()
	if err := l.Set(0, 0, Error); !errors.Is(err, ErrFinished) {
		t.Fatalf("Set after Finish: got %v", err)
	}
	if err := l.RowMessage(0, "x", Running); !errors.Is(err, ErrFinished) {
		t.Fatalf("RowMessage after Finish: got %v", err)
	}
	if err := l.Warn(0, 0, "x"); !errors.Is(err, ErrFinished) {
		t.Fatalf("Warn after Finish: got %v", err)
	}
	if err := l.Fail(0, 0, "x"); !errors.Is(err, ErrFinished) {
		t.Fatalf("Fail after Finish: got %v", err)
	}
	l.Message("late", Running)
	if buf.Len() != n {
		t.Fatalf("nothing may be written after Finish, got %q", buf.String()[n:])
	}
	if s, _ := l.Snapshot().Get(0, 0); s != Pending {
		t.Fatal("grid must not change after Finish")
	}
}

func TestLoader_CloseFinishesWithWorstStatus(t *testing.T) {
	var buf bytes.Buffer
	fc := newFakeClock()
	l := NewLoader(NewGrid([]string{"A"}, 2), WithWriter(&buf), WithPlain(false), withClock(fc))
	l.Start()
	_ = l.Fail(0, 0, "boom")
	_ = l.Set(0, 1, Done)
	fc.Tick()
	l.Close()
	select {
	case <-l.stopped:
	case <-time.After(time.Second):
		t.Fatal("Close must stop the ticker goroutine")
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("Close must leave the cursor on a fresh line, got %q", out)
	}
	if s := l.Snapshot().Worst(); s != Error {
		t.Fatalf("worst = %v", s)
	}
	n := buf.Len()
	l.Finish("again", Done)
	l.Close()
	if buf.Len() != n {
		t.Fatal("Close and Finish after Close must be no-ops")
	}
}

func TestLoader_FinalFrameIsOneWrite(t *testing.T) {
	var w countingWriter
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&w), WithPlain(false), withClock(newFakeClock()))
	l.Start()
	l.Finish("done", Done)
	if w.writes != 1 {
		t.Fatalf("final frame and newline must be a single write, got %d", w.writes)
	}
	if !strings.HasSuffix(w.buf.String(), "\r\n") {
		t.Fatalf("final write must end with CRLF, got %q", w.buf.String())
	}
}

func TestLoader_FinishFlushesBufferedWriter(t *testing.T) {
	var w countingWriter
	l := NewLoader(NewGrid([]string{"A"}, 1), WithWriter(&w), WithPlain(false), withClock(newFakeClock()))
	l.Start()
	l.Finish("done", Done)
	if w.flushes != 1 {
		t.Fatalf("Finish must flush a writer that supports it, got %d flushes", w.flushes)
	}
}

// countingWriter records write and flush calls.
type countingWriter struct {
	buf     bytes.Buffer
	writes  int
	flushes int
}

func (c *countingWriter) Write(p []byte) (int, error) { c.writes++; return c.buf.Write(p) }
func (c *countingWriter) Flush() error                { c.flushes++; return nil }
