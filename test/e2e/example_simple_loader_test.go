// Package e2e drives examples/simple-loader under a headless pseudo-terminal
// and asserts on what a user would see.
//
// Hierarchy: TestExample_SimpleLoader/<Mode>/<Shape>/<Check>. Mode is the
// rendering mode (Footer: sequential with a footer line; Row: row layout in
// scattered mode; Parallel: rows concurrent with in-order cells; Beats: rows
// in lockstep following stall scripts; Plain:
// non-TTY log), Shape the grid size
// (3x5, 5x10), Check one observable behaviour. Every PTY-backed check saves
// hi-DPI PNG and static SVG screenshots plus an animated recording.svg of
// the whole run under test/results/<dateTimeISO>-<testName>/.
package e2e

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	streak "github.com/dimmkirr/go-streak-chart"
	"github.com/dimmkirr/termoscope"
)

const (
	termCols, termRows = 160, 20 // wide enough for two chained notes on one row
	timeout            = 60 * time.Second

	warnText = "Search Service answered slowly, using cached index"
	doneText = "Loaded with warnings"
	errText  = "Loading finished with errors"
)

// outcome is one injected warning or error cell in the example.
type outcome struct {
	row, col int
	hue      string
	text     string
}

// outcomes are the example's injected cells. Their chronological order
// depends on the mode, so tests observe it rather than hard-code it.
var outcomes = []outcome{
	{0, 4, "red", "Secrets vault unreachable, continuing without rotation"},
	{1, 1, "red", "Messaging Service: broker handshake timed out"},
	{1, 3, "amber", warnText},
	{2, 2, "red", "Health checks failed: 2 of 3 probes timed out"},
}

var (
	theme    = streak.DefaultTheme()
	cellRune = []rune(theme.Glyph)[0]
	msgRune  = []rune(theme.MessageGlyph)[0]

	// labels follow the example's built-in row order.
	labels = []string{"Init", "Services", "Activation", "Migrations", "Warmup"}
	shapes = []shape{{"3x5", 3, 5}, {"5x10", 5, 10}}
)

// shape is one grid size the example can render.
type shape struct {
	name       string
	rows, cols int
}

func (s shape) args() []string {
	return []string{"-rows=" + strconv.Itoa(s.rows), "-cols=" + strconv.Itoa(s.cols)}
}

// cells is the text of one full row of tiles.
func (s shape) cells() string {
	return strings.TrimSuffix(strings.Repeat(theme.Glyph+theme.Gap, s.cols), theme.Gap)
}

func (s shape) first() string { return labels[0] }
func (s shape) last() string  { return labels[s.rows-1] }

// complete reports whether the screen holds a fully drawn matrix: every
// labelled row with exactly cols tiles, and the rule. Large frames can
// exceed one PTY read, so a sample may otherwise land mid-redraw.
func (s shape) complete(tm *termoscope.Terminal) bool {
	for _, l := range labels[:s.rows] {
		if n := len(rowHues(tm, l, s.cols)); n != s.cols {
			return false
		}
	}
	_, ok := findLine(tm, theme.RuleChar)
	return ok
}

// settle waits briefly for the final frame to be fully drawn after exit.
func settle(t *testing.T, tm *termoscope.Terminal, s shape) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := tm.WaitUntil(ctx, s.complete); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// harness

func buildExample(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "simple-loader")
	// -buildvcs=false: VCS stamping shells out to git, which can fail in
	// sandboxes (e.g. "dubious ownership") and must not fail the tests.
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, "../../examples/simple-loader")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// start runs the example in a PTY at e2e pace and records the whole run;
// recording.svg is written next to the screenshots when the test ends.
func start(t *testing.T, bin string, args ...string) (*termoscope.Terminal, context.Context) {
	t.Helper()
	return startAt(t, bin, termCols, args...)
}

// startAt is start with an explicit terminal width.
func startAt(t *testing.T, bin string, cols int, args ...string) (*termoscope.Terminal, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	tm, err := termoscope.Start(ctx, cols, termRows, bin, append([]string{"-fast"}, args...)...)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	termoscope.Record(t, tm) // closes tm and writes recording.svg on cleanup
	return tm, ctx
}

// snapshot saves the current screen as both a hi-DPI PNG and a static SVG.
func snapshot(t *testing.T, tm *termoscope.Terminal, name string) {
	t.Helper()
	termoscope.SavePNG(t, tm, name)
	termoscope.SaveSVG(t, tm, name)
}

func plainRun(t *testing.T, bin string, args ...string) []byte {
	t.Helper()
	out, _ := exec.Command(bin, append([]string{"-fast", "-plain"}, args...)...).CombinedOutput()
	if bytes.Contains(out, []byte("\x1b[")) {
		t.Fatalf("plain output must not contain escape codes:\n%s", out)
	}
	return out
}

// ---------------------------------------------------------------------------
// screen queries

// findLine returns the first screen line starting with prefix, if any.
// Safe to call from wait predicates while the screen is still blank.
func findLine(tm *termoscope.Terminal, prefix string) (string, bool) {
	for y := 0; y < tm.Height(); y++ {
		if l := tm.Line(y); strings.HasPrefix(l, prefix) {
			return l, true
		}
	}
	return "", false
}

// line returns the index and text of the first screen line starting with
// prefix, failing the test when absent.
func line(t *testing.T, tm *termoscope.Terminal, prefix string) (int, string) {
	t.Helper()
	for y := 0; y < tm.Height(); y++ {
		if l := tm.Line(y); strings.HasPrefix(l, prefix) {
			return y, l
		}
	}
	t.Fatalf("no line starting with %q on screen:\n%s", prefix, tm.Screen())
	return 0, ""
}

// cellAt returns the screen coordinates of the n-th (0-based) tile on the
// row labelled label.
func cellAt(t *testing.T, tm *termoscope.Terminal, label string, n int) (int, int) {
	t.Helper()
	y, l := line(t, tm, label)
	group := -1
	for x, r := range []rune(l) {
		if r != cellRune {
			continue
		}
		if group++; group == n {
			return x, y
		}
	}
	t.Fatalf("row %q has fewer than %d tiles: %q", label, n+1, l)
	return 0, 0
}

func ruleLine(t *testing.T, tm *termoscope.Terminal) int {
	t.Helper()
	y, _ := line(t, tm, theme.RuleChar)
	return y
}

// underRule returns the indexes of the consecutive lines below the rule
// that start with the status square (issue notes, then the footer if any).
func underRule(t *testing.T, tm *termoscope.Terminal) []int {
	t.Helper()
	var ys []int
	for y := ruleLine(t, tm) + 1; y < tm.Height(); y++ {
		if !strings.HasPrefix(tm.Line(y), string(msgRune)+" ") {
			break
		}
		ys = append(ys, y)
	}
	return ys
}

// hueOf classifies a cell's foreground as one of the status hues.
func hueOf(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	switch {
	case absDiff(r, g) < 0x1000 && absDiff(g, b) < 0x1000:
		return "grey"
	case g > r && g > b:
		return "green"
	case b > r && b > g:
		return "blue"
	case r > b && g > b && r >= g && g*2 > r: // amber has real green; red does not
		return "amber"
	case r > g && r > b:
		return "red"
	}
	return ""
}

// rowHues returns the hue of every tile on label's row.
func rowHues(tm *termoscope.Terminal, label string, cols int) []string {
	l, ok := findLine(tm, label)
	if !ok {
		return nil
	}
	y := 0
	for ; y < tm.Height(); y++ {
		if tm.Line(y) == l {
			break
		}
	}
	if y == tm.Height() {
		return nil // the screen changed between the two reads; sample again
	}
	hues := make([]string, 0, cols)
	for x, r := range []rune(l) {
		if r == cellRune {
			c := tm.CellAt(x, y)
			if c == nil || c.Content != theme.Glyph {
				return nil // mid-redraw: the cell under this column moved
			}
			hues = append(hues, hueOf(c.Style.Fg))
		}
	}
	return hues
}

// fg returns the foreground color of a cell, or nil when the cell is off
// screen, so assertions report a hue mismatch instead of panicking.
func fg(tm *termoscope.Terminal, x, y int) color.Color {
	if c := tm.CellAt(x, y); c != nil {
		return c.Style.Fg
	}
	return nil
}

// observeOutcomes polls the screen until the process exits and returns the
// order in which the injected outcome cells turned amber or red. Cells that
// first appear in the same sample are grouped: their relative order is
// unknown, so assertions treat each group as unordered.
func observeOutcomes(tm *termoscope.Terminal, s shape) [][]outcome {
	seen := map[int]bool{}
	var timeline [][]outcome
	sample := func() {
		var group []outcome
		for i, o := range outcomes {
			if seen[i] || o.row >= s.rows {
				continue
			}
			if hues := rowHues(tm, labels[o.row], s.cols); o.col < len(hues) && hues[o.col] == o.hue {
				seen[i] = true
				group = append(group, o)
			}
		}
		if len(group) > 0 {
			timeline = append(timeline, group)
		}
	}
	tick := time.NewTicker(15 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tm.Done():
			time.Sleep(50 * time.Millisecond) // let the final frame land
			sample()
			return timeline
		case <-tick.C:
			sample()
		}
	}
}

// rowActive reports whether label's row currently shows "Processing" text.
func rowActive(tm *termoscope.Terminal, s shape, label string) bool {
	l, ok := findLine(tm, label)
	return ok && strings.Contains(l, s.cells()+"  Processing ")
}

func activeRows(tm *termoscope.Terminal, s shape) int {
	n := 0
	for _, l := range labels[:s.rows] {
		if rowActive(tm, s, l) {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// assertions

func assertHue(t *testing.T, c color.Color, hue string) {
	t.Helper()
	if c == nil {
		t.Fatal("cell has no foreground color; is CLICOLOR_FORCE honoured?")
	}
	if got := hueOf(c); got != hue {
		r, g, b, _ := c.RGBA()
		t.Fatalf("want %s, got %s rgb(%d,%d,%d)", hue, got, r>>8, g>>8, b>>8)
	}
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// assertMatrix checks every labelled row shows exactly cols tiles.
func assertMatrix(t *testing.T, tm *termoscope.Terminal, s shape) {
	t.Helper()
	screen := tm.Screen()
	for _, l := range labels[:s.rows] {
		if !strings.Contains(screen, l) {
			t.Errorf("row %q missing:\n%s", l, screen)
		}
	}
	if n := strings.Count(screen, s.cells()); n != s.rows {
		t.Errorf("want %d rows of %d cells, got %d:\n%s", s.rows, s.cols, n, screen)
	}
	assertRuleWidth(t, tm)
}

// assertRuleWidth checks the rule spans the whole terminal: as wide as the
// widest screen line or the column count, whichever is larger. The Loader
// cuts every line to the TTY width, so that is always the column count.
func assertRuleWidth(t *testing.T, tm *termoscope.Terminal) {
	t.Helper()
	_, rule := line(t, tm, theme.RuleChar)
	if strings.Trim(rule, theme.RuleChar) != "" {
		t.Errorf("rule must be made of %q only, got %q", theme.RuleChar, rule)
	}
	want := tm.Width()
	for y := 0; y < tm.Height(); y++ {
		if n := len([]rune(tm.Line(y))); n > want {
			want = n
		}
	}
	if got := len([]rune(rule)); got != want {
		t.Errorf("rule is %d columns wide, want %d = max(widest line, %d columns):\n%s", got, want, tm.Width(), tm.Screen())
	}
}

// assertNoFooter checks that whatever follows the rule is issue notes only,
// never a live "Processing" footer (row-based layouts have no footer).
func assertNoFooter(t *testing.T, tm *termoscope.Terminal) {
	t.Helper()
	for _, y := range underRule(t, tm) {
		if l := tm.Line(y); strings.Contains(l, "Processing") {
			t.Errorf("row layout must not render a footer line, got %q", l)
		}
	}
}

// assertIssues checks that every injected outcome has a note under the
// rule, listed in the order the cells were observed turning amber or red
// (timeline), each led by a square of its status color, followed exactly by
// the given trailing lines (the footer, if any).
func assertIssues(t *testing.T, tm *termoscope.Terminal, timeline [][]outcome, trailing ...string) {
	t.Helper()
	ys := underRule(t, tm)
	if want := len(outcomes) + len(trailing); len(ys) != want {
		t.Fatalf("want %d lines under the rule, got %d:\n%s", want, len(ys), tm.Screen())
	}
	// Position of each note in the rendered list.
	pos := map[string]int{}
	for i, y := range ys[:len(outcomes)] {
		pos[strings.TrimPrefix(tm.Line(y), string(msgRune)+" ")] = i
	}
	observed := 0
	for gi, group := range timeline {
		observed += len(group)
		for _, o := range group {
			p, ok := pos[o.text]
			if !ok {
				t.Fatalf("note %q missing under the rule:\n%s", o.text, tm.Screen())
			}
			assertHue(t, fg(tm, 0, ys[p]), o.hue)
			// Every note from an earlier group must come before this one.
			for _, earlier := range timeline[:gi] {
				for _, e := range earlier {
					if pos[e.text] > p {
						t.Errorf("issue order: %q appeared before %q on screen but is listed after it", e.text, o.text)
					}
				}
			}
		}
	}
	if observed != len(outcomes) {
		t.Fatalf("observed %d of %d outcomes turning amber/red; sampling missed some", observed, len(outcomes))
	}
	for i, w := range trailing {
		if l := tm.Line(ys[len(outcomes)+i]); !strings.HasSuffix(l, w) {
			t.Errorf("trailing line %d under the rule: got %q want suffix %q", i, l, w)
		}
	}
}

// assertOutcomeCells checks the injected warning and error tiles.
func assertOutcomeCells(t *testing.T, tm *termoscope.Terminal) {
	t.Helper()
	for _, o := range outcomes {
		x, y := cellAt(t, tm, labels[o.row], o.col)
		assertHue(t, fg(tm, x, y), o.hue)
	}
}

func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	var ee *exec.ExitError
	switch {
	case want == 0 && err == nil:
	case want != 0 && errors.As(err, &ee) && ee.ExitCode() == want:
	default:
		t.Fatalf("want exit code %d, got %v", want, err)
	}
}

// ---------------------------------------------------------------------------
// tests

func TestExample_SimpleLoader(t *testing.T) {
	bin := buildExample(t)

	t.Run("Footer", func(t *testing.T) { forEachShape(t, func(t *testing.T, s shape) { footerChecks(t, bin, s) }) })
	t.Run("Row", func(t *testing.T) { forEachShape(t, func(t *testing.T, s shape) { rowChecks(t, bin, s) }) })
	t.Run("Parallel", func(t *testing.T) { forEachShape(t, func(t *testing.T, s shape) { parallelChecks(t, bin, s) }) })
	t.Run("Beats", func(t *testing.T) { forEachShape(t, func(t *testing.T, s shape) { beatsChecks(t, bin, s) }) })
	t.Run("Plain", func(t *testing.T) { plainChecks(t, bin) })
	t.Run("VariableRowCols", func(t *testing.T) { variableRowColsChecks(t, bin) })
}

func forEachShape(t *testing.T, run func(t *testing.T, s shape)) {
	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			run(t, s)
		})
	}
}

// footerChecks: default layout, one status line under the rule.
func footerChecks(t *testing.T, bin string, s shape) {
	t.Run("Matrix", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, s.args()...)
		if err := tm.WaitUntil(ctx, func(tm *termoscope.Terminal) bool {
			return s.complete(tm) && strings.Contains(tm.Screen(), "Processing Services")
		}); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "mid-run")
		assertMatrix(t, tm, s)
		x, y := cellAt(t, tm, s.first(), 0)
		assertHue(t, fg(tm, x, y), "green")
		x, y = cellAt(t, tm, s.last(), s.cols-1)
		assertHue(t, fg(tm, x, y), "grey")
		if ys := underRule(t, tm); len(ys) == 0 || !strings.HasPrefix(tm.Line(ys[len(ys)-1]), string(msgRune)+" Processing Services") {
			t.Errorf("footer line must be the last line under the rule:\n%s", tm.Screen())
		}
		_ = tm.Wait()
	})

	t.Run("CompletesWithWarning", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, append(s.args(), "-error=false")...)
		if _, err := tm.WaitFor(ctx, doneText); err != nil {
			t.Fatal(err)
		}
		assertExitCode(t, tm.Wait(), 0)
		settle(t, tm, s)
		snapshot(t, tm, "final")
		x, y := cellAt(t, tm, "Services", 3)
		assertHue(t, fg(tm, x, y), "amber")
		ys := underRule(t, tm)
		if len(ys) != 2 || !strings.HasSuffix(tm.Line(ys[0]), warnText) || !strings.HasSuffix(tm.Line(ys[1]), doneText) {
			t.Fatalf("want warning note then footer under the rule:\n%s", tm.Screen())
		}
		assertHue(t, fg(tm, 0, ys[1]), "amber")
		if n := strings.Count(tm.Screen(), s.first()); n != 1 {
			t.Fatalf("expected a single frame on screen, found %d:\n%s", n, tm.Screen())
		}
	})

	t.Run("FinishesWithErrors", func(t *testing.T) {
		t.Parallel()
		tm, _ := start(t, bin, s.args()...)
		timeline := observeOutcomes(tm, s)
		assertExitCode(t, tm.Wait(), 1)
		settle(t, tm, s)
		snapshot(t, tm, "final")
		assertOutcomeCells(t, tm)
		assertIssues(t, tm, timeline, errText) // notes in order of appearance, footer last
	})

	t.Run("IssueLogDisabled", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, append(s.args(), "-issues=none")...)
		if _, err := tm.WaitFor(ctx, errText); err != nil {
			t.Fatal(err)
		}
		_ = tm.Wait()
		settle(t, tm, s)
		snapshot(t, tm, "final")
		for _, o := range outcomes {
			if strings.Contains(tm.Screen(), o.text) {
				t.Fatalf("issue log must be hidden, found %q:\n%s", o.text, tm.Screen())
			}
		}
		if ys := underRule(t, tm); len(ys) != 1 {
			t.Fatalf("only the footer may follow the rule, got %d lines:\n%s", len(ys), tm.Screen())
		}
	})
}

// rowChecks: row layout in scattered mode, which is what the layout is for:
// rows progress concurrently, each with two workers over a strided cell
// order, so tiles light up out of sequence and every row carries its own
// live text. No footer line.
func rowChecks(t *testing.T, bin string, s shape) {
	args := append(s.args(), "-mode=scattered")

	t.Run("ActiveRowText", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, args...)
		// Init runs fastest: wait until it has finished (its tiles terminal, its
		// live text gone) while at least one slower row is still processing.
		initDoneOthersLive := func(tm *termoscope.Terminal) bool {
			return rowSettled(tm, s, s.first()) && activeRows(tm, s) >= 1
		}
		if err := tm.WaitUntil(ctx, func(tm *termoscope.Terminal) bool { return s.complete(tm) && initDoneOthersLive(tm) }); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "mid-run")
		assertMatrix(t, tm, s)
		for _, label := range labels[1:s.rows] {
			if !rowActive(tm, s, label) {
				continue
			}
			re := regexp.MustCompile(`^` + label + `\s+` + regexp.QuoteMeta(s.cells()) + `  Processing \S.*$`)
			if _, l := line(t, tm, label); !re.MatchString(l) {
				t.Errorf("active row must carry its status text after its cells, got %q", l)
			}
		}
		// The first row finished with an injected error: its live text is gone
		// and only the note remains (inline issues, the default).
		if _, l := line(t, tm, s.first()); strings.Contains(l, "Processing") || !strings.Contains(l, s.cells()+"  "+outcomes[0].text) {
			t.Errorf("done row must show its note instead of live text, got %q", l)
		}
		assertNoFooter(t, tm)
		_ = tm.Wait()
	})

	t.Run("OutOfOrder", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, args...)
		// Proof of non-sequential progress: some row has a finished tile to
		// the right of an unstarted one, some row has two tiles running, and
		// at least two rows are in flight at once.
		scattered := func(tm *termoscope.Terminal) bool {
			gapBeforeDone, twoRunning := false, false
			for _, l := range labels[:s.rows] {
				running, sawGrey := 0, false
				for _, h := range rowHues(tm, l, s.cols) {
					switch h {
					case "grey":
						sawGrey = true
					case "blue":
						running++
					case "green", "amber", "red":
						if sawGrey {
							gapBeforeDone = true
						}
					}
				}
				if running >= 2 {
					twoRunning = true
				}
			}
			return gapBeforeDone && twoRunning && activeRows(tm, s) >= 2
		}
		if err := tm.WaitUntil(ctx, func(tm *termoscope.Terminal) bool { return s.complete(tm) && scattered(tm) }); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "mid-run")
		assertMatrix(t, tm, s)
		assertNoFooter(t, tm)
		_ = tm.Wait()
	})

	t.Run("ClearsWhenDone", func(t *testing.T) {
		t.Parallel()
		tm, _ := start(t, bin, args...)
		_ = observeOutcomes(tm, s) // blocks until the program exits
		assertExitCode(t, tm.Wait(), 1)
		settle(t, tm, s)
		snapshot(t, tm, "final")
		if screen := tm.Screen(); strings.Contains(screen, "Processing") {
			t.Fatalf("finished rows must not keep status text:\n%s", screen)
		}
		assertMatrix(t, tm, s)
		assertOutcomeCells(t, tm)
		assertInlineIssues(t, tm, s) // default mode: notes stay on their rows, nothing under the rule
	})

	// FitsWidth: inline notes are cut with an ellipsis at the terminal width,
	// so no row wraps, the rule stays on line rows and nothing follows it.
	t.Run("FitsWidth", func(t *testing.T) {
		t.Parallel()
		const narrow = 60
		tm, _ := startAt(t, bin, narrow, args...)
		_ = observeOutcomes(tm, s) // blocks until the program exits
		assertExitCode(t, tm.Wait(), 1)
		settle(t, tm, s)
		snapshot(t, tm, "final")
		assertRuleWidth(t, tm)
		if y := ruleLine(t, tm); y != s.rows {
			t.Fatalf("rule on line %d, want %d: a row wrapped\n%s", y, s.rows, tm.Screen())
		}
		cut := 0
		for y := 0; y < s.rows; y++ {
			l := tm.Line(y)
			if n := len([]rune(l)); n > narrow {
				t.Errorf("line %d is %d columns wide, terminal is %d: %q", y, n, narrow, l)
			}
			if strings.HasSuffix(l, "…") {
				cut++
			}
		}
		if cut == 0 {
			t.Fatalf("expected at least one truncated row at %d columns:\n%s", narrow, tm.Screen())
		}
		for y := s.rows + 1; y < tm.Height(); y++ {
			if l := tm.Line(y); l != "" {
				t.Errorf("nothing may follow the rule, line %d = %q", y, l)
			}
		}
	})
}

// parallelChecks: rows driven by independent goroutines.
func parallelChecks(t *testing.T, bin string, s shape) {
	args := append(s.args(), "-mode=parallel")

	t.Run("ConcurrentText", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, args...)
		if err := tm.WaitUntil(ctx, func(tm *termoscope.Terminal) bool { return s.complete(tm) && activeRows(tm, s) >= 2 }); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "mid-run")
		if n := activeRows(tm, s); n < 2 {
			t.Fatalf("want at least 2 rows with live text, got %d:\n%s", n, tm.Screen())
		}
		assertMatrix(t, tm, s)
		assertNoFooter(t, tm)
		_ = tm.Wait()
	})

	t.Run("IndependentClear", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, args...)
		// The first row runs fastest and the last slowest, so the first
		// finishes (live text gone) while the last still shows some.
		firstDoneLastLive := func(tm *termoscope.Terminal) bool {
			return rowSettled(tm, s, s.first()) && rowActive(tm, s, s.last())
		}
		if err := tm.WaitUntil(ctx, func(tm *termoscope.Terminal) bool { return s.complete(tm) && firstDoneLastLive(tm) }); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "first-done-last-live")
		x, y := cellAt(t, tm, s.first(), s.cols-2)
		assertHue(t, fg(tm, x, y), "green")
		assertExitCode(t, tm.Wait(), 1)
		settle(t, tm, s)
		snapshot(t, tm, "final")
		if screen := tm.Screen(); strings.Contains(screen, "Processing") {
			t.Fatalf("all rows done, no text may remain:\n%s", screen)
		}
		assertOutcomeCells(t, tm)
	})

	t.Run("IssuesInOrderOfAppearance", func(t *testing.T) {
		t.Parallel()
		tm, _ := start(t, bin, append(args, "-issues=log")...) // log mode keeps the chronological list under the rule
		timeline := observeOutcomes(tm, s)
		assertExitCode(t, tm.Wait(), 1)
		settle(t, tm, s)
		snapshot(t, tm, "final")
		assertIssues(t, tm, timeline)
	})
}

// beatsChecks: rows in lockstep on a shared beat, each following a script
// where "_" is a beat with nothing happening on that row:
//
//	Init        A B C D E
//	Services    _ A B _ C D E
//	Activation  A _ _ B C _ _ D E
//	Migrations  A B C D _ _ _ _ E
//	Warmup      _ _ A _ B _ C D E
func beatsChecks(t *testing.T, bin string, s shape) {
	args := append(s.args(), "-mode=beats")

	// The longest stall per shape: the row, how many cells it has finished
	// when it stalls, and how many beats it idles.
	stallRow, doneAtStall := "Activation", 1 // "A__BC__DE": idles two beats after A
	if s.rows >= 4 {
		stallRow, doneAtStall = "Migrations", 4 // "ABCD____E": idles four beats after D
	}

	t.Run("StallBeats", func(t *testing.T) {
		t.Parallel()
		tm, ctx := start(t, bin, args...)
		// During the stall the row shows exactly doneAtStall finished tiles,
		// nothing running, no text, while other rows keep moving.
		stalled := func(tm *termoscope.Terminal) bool {
			hues := rowHues(tm, stallRow, s.cols)
			if len(hues) != s.cols {
				return false
			}
			for i, h := range hues {
				switch {
				case i < doneAtStall && h != "green":
					return false
				case i >= doneAtStall && h != "grey":
					return false
				}
			}
			l, _ := findLine(tm, stallRow)
			return strings.HasSuffix(l, s.cells()) && activeRows(tm, s) >= 1
		}
		if err := tm.WaitUntil(ctx, func(tm *termoscope.Terminal) bool { return s.complete(tm) && stalled(tm) }); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "stall")
		assertMatrix(t, tm, s)
		assertNoFooter(t, tm)
		// Lockstep: Init runs every beat, so it is never behind the stalled row.
		initDone := 0
		for _, h := range rowHues(tm, s.first(), s.cols) {
			if h == "green" || h == "amber" || h == "red" {
				initDone++
			}
		}
		if initDone < doneAtStall {
			t.Errorf("Init has %d done tiles while %s stalls at %d; rows must advance in lockstep", initDone, stallRow, doneAtStall)
		}
		_ = tm.Wait()
	})

	t.Run("ClearsWhenDone", func(t *testing.T) {
		t.Parallel()
		tm, _ := start(t, bin, args...)
		_ = observeOutcomes(tm, s) // blocks until the program exits
		assertExitCode(t, tm.Wait(), 1)
		settle(t, tm, s)
		snapshot(t, tm, "final")
		if screen := tm.Screen(); strings.Contains(screen, "Processing") {
			t.Fatalf("finished rows must not keep status text:\n%s", screen)
		}
		assertMatrix(t, tm, s)
		assertOutcomeCells(t, tm)
		assertInlineIssues(t, tm, s)
	})
}

// plainChecks: non-TTY output, one log line per change.
func plainChecks(t *testing.T, bin string) {
	t.Run("Forced", func(t *testing.T) {
		t.Parallel()
		out := plainRun(t, bin, "-error=false")
		for _, want := range []string{"[warning] Services #4: " + warnText, "[warning] " + doneText} {
			if !bytes.Contains(out, []byte(want)) {
				t.Fatalf("missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("Piped", func(t *testing.T) {
		t.Parallel()
		out, err := exec.Command(bin, "-fast", "-error=false").CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if bytes.Contains(out, []byte("\x1b[")) {
			t.Fatalf("non-TTY stdout must fall back to plain:\n%s", out)
		}
	})

	t.Run("RowText", func(t *testing.T) {
		t.Parallel()
		out := plainRun(t, bin, "-layout=row", "-error=false")
		if !bytes.Contains(out, []byte("[running] Services: Processing Auth Service")) {
			t.Fatalf("row text must be logged with its row label:\n%s", out)
		}
	})

	t.Run("Notes", func(t *testing.T) {
		t.Parallel()
		out := plainRun(t, bin)
		for _, o := range outcomes {
			want := "[" + map[string]string{"red": "error", "amber": "warning"}[o.hue] + "] " +
				labels[o.row] + " #" + strconv.Itoa(o.col+1) + ": " + o.text
			if !bytes.Contains(out, []byte(want)) {
				t.Fatalf("missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("ScatteredOrder", func(t *testing.T) {
		t.Parallel()
		out := plainRun(t, bin, "-mode=scattered", "-error=false")
		// Init's cells must not complete left to right.
		var cols []int
		for _, l := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(l, "[done] Init #") {
				n, _ := strconv.Atoi(strings.TrimPrefix(l, "[done] Init #"))
				cols = append(cols, n)
			}
		}
		if len(cols) < 3 || sort.IntsAreSorted(cols) {
			t.Fatalf("scattered mode must complete Init cells out of order, got %v:\n%s", cols, out)
		}
	})

	t.Run("ParallelInterleave", func(t *testing.T) {
		t.Parallel()
		out := plainRun(t, bin, "-mode=parallel")
		// Rows run concurrently, so the log must switch between row labels
		// rather than list each row as one contiguous block.
		switches, prev := 0, ""
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			label := l
			if i := strings.Index(label, "] "); i >= 0 {
				label = label[i+2:]
			}
			if i := strings.IndexAny(label, " :#"); i > 0 {
				label = label[:i]
			}
			if prev != "" && label != prev {
				switches++
			}
			prev = label
		}
		if switches < 3 {
			t.Fatalf("expected interleaved row logs, got %d switches:\n%s", switches, out)
		}
	})
}

// assertInlineIssues checks the default issue mode in row layouts: every
// injected note sits on its own row after the tiles, colored by its status,
// the rule is the last line of the frame (row r's text on line r, rule on
// line rows), and nothing follows it, so a host can continue rows+2 down.
func assertInlineIssues(t *testing.T, tm *termoscope.Terminal, s shape) {
	t.Helper()
	for _, o := range outcomes {
		if o.row >= s.rows {
			continue
		}
		y, l := line(t, tm, labels[o.row])
		if y != o.row {
			t.Errorf("row %q rendered on line %d, want %d", labels[o.row], y, o.row)
		}
		runes := []rune(l)
		at := strings.Index(l, o.text)
		if at < 0 {
			t.Errorf("note %q missing from its row:\n%s", o.text, tm.Screen())
			continue
		}
		x := len([]rune(l[:at]))
		assertHue(t, fg(tm, x, y), o.hue)
		if x <= len(runes) && !strings.Contains(l, s.cells()+"  ") {
			t.Errorf("note must follow the tiles and two spaces, got %q", l)
		}
	}
	assertRuleWidth(t, tm)
	if y := ruleLine(t, tm); y != s.rows {
		t.Errorf("rule on line %d, want %d (one line per row)", y, s.rows)
	}
	for y := ruleLine(t, tm) + 1; y < tm.Height(); y++ {
		if l := tm.Line(y); l != "" {
			t.Errorf("nothing may follow the rule in inline mode, line %d = %q", y, l)
		}
	}
}

// variableRowColsChecks: rows with different active column counts. Rows
// with fewer active cells render blank space (no glyph) for trailing
// columns, keeping the grid aligned. CELL-XXX.
func variableRowColsChecks(t *testing.T, bin string) {
	// 3 rows, 5 max cols, but row 1 (Services) gets only 3 active cells.
	t.Run("BlankTrailingCells", func(t *testing.T) {
		t.Parallel()
		tm, _ := start(t, bin, "-rows=3", "-cols=5", "-rowcols=5,3,5", "-error=false", "-mode=parallel")
		_ = tm.Wait()

		ctx2, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := tm.WaitUntil(ctx2, func(tm *termoscope.Terminal) bool {
			_, ok := findLine(tm, theme.RuleChar)
			return ok
		}); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "final")

		// Row 0 (Init): 5 tiles.
		initTiles := countTiles(tm, "Init")
		if initTiles != 5 {
			t.Errorf("Init: want 5 tiles, got %d\n%s", initTiles, tm.Screen())
		}

		// Row 1 (Services): 3 active tiles, trailing 2 should be blank.
		svcTiles := countTiles(tm, "Services")
		if svcTiles != 3 {
			t.Errorf("Services: want 3 tiles, got %d\n%s", svcTiles, tm.Screen())
		}

		// Row 2 (Activation): 5 tiles.
		actTiles := countTiles(tm, "Activation")
		if actTiles != 5 {
			t.Errorf("Activation: want 5 tiles, got %d\n%s", actTiles, tm.Screen())
		}
	})

	t.Run("RowDoneWithFewerCells", func(t *testing.T) {
		t.Parallel()
		tm, _ := start(t, bin, "-rows=3", "-cols=5", "-rowcols=5,3,5", "-error=false")
		_ = tm.Wait()

		ctx2, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := tm.WaitUntil(ctx2, func(tm *termoscope.Terminal) bool {
			_, ok := findLine(tm, theme.RuleChar)
			return ok
		}); err != nil {
			t.Fatal(err)
		}
		snapshot(t, tm, "final")

		// All 3 tiles on Services should be green (done), not grey (pending).
		hues := rowHues(tm, "Services", 3)
		for i, h := range hues {
			if h != "green" {
				t.Errorf("Services tile %d: want green, got %s\n%s", i, h, tm.Screen())
			}
		}
	})
}

// countTiles returns the number of ▄ glyphs on label's row.
func countTiles(tm *termoscope.Terminal, label string) int {
	l, ok := findLine(tm, label)
	if !ok {
		return 0
	}
	n := 0
	for _, r := range l {
		if r == cellRune {
			n++
		}
	}
	return n
}

// rowSettled reports whether label's row has finished: every tile is
// terminal (no grey or blue) and no live "Processing" text remains. In the
// default inline mode a finished row may still show its notes.
func rowSettled(tm *termoscope.Terminal, s shape, label string) bool {
	hues := rowHues(tm, label, s.cols)
	if len(hues) != s.cols {
		return false
	}
	for _, h := range hues {
		if h == "grey" || h == "blue" {
			return false
		}
	}
	return !rowActive(tm, s, label)
}
