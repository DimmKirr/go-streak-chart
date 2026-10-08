package streak

import (
	"fmt"
	"github.com/charmbracelet/x/term"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Option configures a Loader.
type Option func(*Loader)

// WithWriter sets the output. Default os.Stderr. Writers that are not a
// character-device *os.File render in plain mode.
func WithWriter(w io.Writer) Option { return func(l *Loader) { l.w = w } }

// WithTheme overrides DefaultTheme.
func WithTheme(t Theme) Option { return func(l *Loader) { l.theme = t } }

// WithTick sets the redraw and pulse interval. Default 120ms.
func WithTick(d time.Duration) Option { return func(l *Loader) { l.tick = d } }

// WithWidth caps frame lines at w columns instead of asking the TTY for
// its size on every draw. 0 restores auto-detection.
func WithWidth(w int) Option { return func(l *Loader) { l.width = w } }

// WithPlain forces plain mode on or off instead of detecting a TTY.
func WithPlain(plain bool) Option { return func(l *Loader) { l.forcePlain = &plain } }

// WithIssues selects where Warning and Error notes are shown (see IssueMode).
func WithIssues(m IssueMode) Option { return func(l *Loader) { l.theme.Issues = m } }

func withClock(c clock) Option { return func(l *Loader) { l.clock = c } }

// Loader renders a Grid inline in cooked mode: no raw mode, no alternate
// screen. Frames are redrawn in place on each tick by moving the cursor up
// and erasing below. When the writer is not a TTY it prints one plain line
// per state change instead. All methods are safe for concurrent use.
type Loader struct {
	w          io.Writer
	theme      Theme
	tick       time.Duration
	forcePlain *bool
	width      int // explicit column cap; 0 = detect from the writer when it is a TTY
	clock      clock

	mu       sync.Mutex
	grid     *Grid
	msg      Message
	dir      int
	plain    bool
	started  bool
	finished bool
	drawn    int // lines of the frame currently on screen
	stop     chan struct{}
	stopped  chan struct{}
}

// NewLoader wraps g. The loader owns g after this call.
func NewLoader(g *Grid, opts ...Option) *Loader {
	l := &Loader{
		w:     os.Stderr,
		theme: DefaultTheme(),
		tick:  120 * time.Millisecond,
		clock: realClock{},
		grid:  g,
		dir:   1,
	}
	for _, o := range opts {
		o(l)
	}
	l.plain = l.decidePlain()
	return l
}

func (l *Loader) decidePlain() bool {
	if l.forcePlain != nil {
		return *l.forcePlain
	}
	f, ok := l.w.(*os.File)
	if !ok {
		return true
	}
	fi, err := f.Stat()
	return err != nil || fi.Mode()&os.ModeCharDevice == 0
}

// Start begins redrawing. It is a no-op in plain mode or when already started.
func (l *Loader) Start() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.plain || l.started || l.finished {
		return
	}
	l.started = true
	l.stop = make(chan struct{})
	l.stopped = make(chan struct{})
	go l.run()
}

func (l *Loader) run() {
	defer close(l.stopped)
	ticks, stop := l.clock.Ticker(l.tick)
	defer stop()
	for {
		select {
		case <-l.stop:
			return
		case <-ticks:
			l.mu.Lock()
			if !l.finished {
				if l.msg.Status == Running {
					l.msg.Level, l.dir = NextPulse(l.msg.Level, l.dir)
				}
				l.draw()
			}
			l.mu.Unlock()
			if a, ok := l.clock.(tickAcker); ok {
				a.tickDone()
			}
		}
	}
}

// Set updates one cell.
func (l *Loader) Set(row, col int, s Status) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.grid.Set(row, col, s); err != nil {
		return err
	}
	if l.plain {
		l.printf("[%s] %s #%d\n", s, l.grid.Label(row), col+1)
	}
	return nil
}

// Message replaces the status line. Running messages pulse on each tick.
func (l *Loader) Message(text string, s Status) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msg = Message{Text: text, Status: s, Level: 1}
	l.dir = 1
	if l.plain {
		l.printf("[%s] %s\n", s, text)
	}
}

// Warn marks a cell Warning and records why. The note stays listed under
// the rule after the row completes.
func (l *Loader) Warn(row, col int, text string) error { return l.note(row, col, Warning, text) }

// Fail marks a cell Error and records why. The note stays listed under the
// rule after the row completes.
func (l *Loader) Fail(row, col int, text string) error { return l.note(row, col, Error, text) }

func (l *Loader) note(row, col int, s Status, text string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.grid.Set(row, col, s); err != nil {
		return err
	}
	_ = l.grid.SetNote(row, col, text)
	if l.plain {
		l.printf("[%s] %s #%d: %s\n", s, l.grid.Label(row), col+1, text)
	}
	return nil
}

// Snapshot returns a copy of the grid as of now. Use it to summarize the
// outcome (for example Grid.Worst) after concurrent producers have joined.
func (l *Loader) Snapshot() *Grid {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.grid.Clone()
}

// RowMessage sets a row's status text, shown after its cells in RowLayout
// until the row is done.
func (l *Loader) RowMessage(row int, text string, s Status) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.grid.SetRowMessage(row, Message{Text: text, Status: s, Level: MaxLevel}); err != nil {
		return err
	}
	if l.plain {
		l.printf("[%s] %s: %s\n", s, l.grid.Label(row), text)
	}
	return nil
}

// Finish stops redrawing, draws the final frame at full intensity and
// leaves it on screen followed by a newline. Subsequent calls are no-ops.
func (l *Loader) Finish(text string, s Status) {
	l.mu.Lock()
	if l.finished {
		l.mu.Unlock()
		return
	}
	l.finished = true
	l.msg = Message{Text: text, Status: s, Level: MaxLevel}
	if l.plain {
		if text != "" {
			l.printf("[%s] %s\n", s, text)
		}
		l.mu.Unlock()
		return
	}
	started := l.started
	l.mu.Unlock()

	if started {
		close(l.stop)
		<-l.stopped
	}

	l.mu.Lock()
	l.draw()
	l.printf("\r\n")
	l.drawn = 0
	l.mu.Unlock()
}

// draw writes the current frame. The caller must hold mu.
func (l *Loader) draw() {
	th := l.theme
	if th.Width == 0 {
		th.Width = l.termWidth()
	}
	lines := Lines(l.grid, l.msg, th)
	var b strings.Builder
	if l.drawn > 0 {
		b.WriteString("\r")
		if l.drawn > 1 {
			fmt.Fprintf(&b, "\x1b[%dA", l.drawn-1)
		}
		b.WriteString("\x1b[J")
	}
	b.WriteString(strings.Join(lines, "\r\n"))
	l.printf("%s", b.String())
	l.drawn = len(lines)
}

// level is a test hook returning the current message level.
func (l *Loader) level() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.msg.Level
}

// printf writes to the terminal. Write errors are deliberately ignored: a
// closed or broken terminal must not fail the host's loading sequence.
func (l *Loader) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.w, format, args...)
}

// termWidth returns the explicit width, else the writer's terminal width
// when it is a TTY, else 0 (unlimited).
func (l *Loader) termWidth() int {
	if l.width > 0 {
		return l.width
	}
	if f, ok := l.w.(*os.File); ok && term.IsTerminal(f.Fd()) {
		if w, _, err := term.GetSize(f.Fd()); err == nil && w > 0 {
			return w
		}
	}
	return 0
}
