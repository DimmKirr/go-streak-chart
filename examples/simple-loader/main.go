// Command simple-loader demonstrates streak.Loader and doubles as the
// fixture for the e2e tests in test/e2e.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	streak "github.com/dimmkirr/go-streak-chart"
)

var (
	defaultNames = [][]string{
		{"Config", "Logger", "Telemetry", "Cache", "Secrets"},
		{"Auth Service", "Messaging Service", "Storage Service", "Search Service", "Billing Service"},
		{"Warm caches", "Register routes", "Health checks", "Announce", "Ready"},
	}
)

func main() {
	fast := flag.Bool("fast", false, "run with short delays (used by e2e tests and recordings)")
	plain := flag.Bool("plain", false, "force plain-text output")
	withErr := flag.Bool("error", true, "inject three error cells (Init #5, Services #2, Activation #3)")
	layout := flag.String("layout", "footer", `"footer": one status line under the matrix; "row": status text per row`)
	mode := flag.String("mode", "sequential", `"sequential": rows one after another, cells in order; `+
		`"parallel": rows concurrently, cells in order (implies -layout=row); `+
		`"scattered": rows concurrently, two workers per row over a strided cell order (implies -layout=row); `+
		`"beats": rows in lockstep on a shared beat, each following a script with stall beats (implies -layout=row)`)
	issues := flag.Bool("issues", true, "list warning and error notes under the rule")
	nRows := flag.Int("rows", 3, "number of groups (rows)")
	nCols := flag.Int("cols", 5, "number of components per group (columns)")
	flag.Parse()

	th := streak.DefaultTheme()
	switch *layout {
	case "footer":
		th.Layout = streak.FooterLayout
	case "row":
		th.Layout = streak.RowLayout
	default:
		fmt.Fprintf(os.Stderr, "unknown -layout %q\n", *layout)
		os.Exit(2)
	}
	switch *mode {
	case "sequential":
	case "parallel", "scattered", "beats":
		th.Layout = streak.RowLayout
	default:
		fmt.Fprintf(os.Stderr, "unknown -mode %q\n", *mode)
		os.Exit(2)
	}

	step := 400 * time.Millisecond
	if *fast {
		step = 120 * time.Millisecond
	}

	rows, names := shape(*nRows, *nCols)
	g := streak.NewGrid(rows, *nCols)
	opts := []streak.Option{streak.WithWriter(os.Stdout), streak.WithTheme(th), streak.WithIssueLog(*issues)}
	if *plain {
		opts = append(opts, streak.WithPlain(true))
	}
	l := streak.NewLoader(g, opts...)
	l.Start()

	// runCell processes one component: mark it running, publish its text,
	// wait d, then land the outcome (one injected warning, three errors).
	runCell := func(r, c int, d time.Duration) {
		_ = l.Set(r, c, streak.Running)
		if th.Layout == streak.RowLayout {
			_ = l.RowMessage(r, "Processing "+names[r][c], streak.Running)
		} else {
			l.Message("Processing "+rows[r]+": "+names[r][c], streak.Running)
		}
		time.Sleep(d)
		switch {
		case r == 1 && c == 3:
			_ = l.Warn(r, c, "Search Service answered slowly, using cached index")
		case *withErr && r == 0 && c == 4:
			_ = l.Fail(r, c, "Secrets vault unreachable, continuing without rotation")
		case *withErr && r == 1 && c == 1:
			_ = l.Fail(r, c, "Messaging Service: broker handshake timed out")
		case *withErr && r == 2 && c == 2:
			_ = l.Fail(r, c, "Health checks failed: 2 of 3 probes timed out")
		default:
			_ = l.Set(r, c, streak.Done)
		}
	}

	// runRow feeds a row's cells, in the given order, to `workers` goroutines.
	runRow := func(r int, d time.Duration, order []int, workers int) {
		queue := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for c := range queue {
					runCell(r, c, d)
				}
			}()
		}
		for _, c := range order {
			queue <- c
		}
		close(queue)
		wg.Wait()
	}

	// Rows in the concurrent modes get their own pace (base step plus 3/8
	// per row) so they visibly desynchronize.
	pace := func(r int) time.Duration { return step + step*3*time.Duration(r)/8 }

	switch *mode {
	case "sequential":
		for r := range names {
			runRow(r, step, sequence(*nCols), 1)
		}
	case "parallel", "scattered":
		var wg sync.WaitGroup
		for r := range names {
			wg.Add(1)
			go func(r int) {
				defer wg.Done()
				if *mode == "scattered" {
					runRow(r, pace(r), strided(*nCols, r), 2)
				} else {
					runRow(r, pace(r), sequence(*nCols), 1)
				}
			}(r)
		}
		wg.Wait()
	case "beats":
		// Every row follows its script on a shared beat: a letter processes
		// the next cell during that beat, "_" skips the beat.
		start := time.Now()
		var wg sync.WaitGroup
		for r := range names {
			wg.Add(1)
			go func(r int) {
				defer wg.Done()
				next := 0
				for k, action := range beatScript(r, *nCols) {
					time.Sleep(time.Until(start.Add(time.Duration(k) * step)))
					if action == '_' {
						_ = l.RowMessage(r, "", streak.Pending) // idle this beat
						continue
					}
					runCell(r, next, step*9/10)
					next++
				}
			}(r)
		}
		wg.Wait()
	}

	final := l.Snapshot().Worst()
	text := "All components loaded"
	switch final {
	case streak.Warning:
		text = "Loaded with warnings"
	case streak.Error:
		text = "Loading finished with errors"
	}
	if th.Layout == streak.RowLayout {
		text = "" // rows carry their own status; no footer line
	}
	l.Finish(text, final)
	if final == streak.Error {
		os.Exit(1)
	}
}

// sequence is 0..n-1 in order.
func sequence(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// strided is a deterministic permutation of 0..n-1 that visits cells in
// jumps of three, offset by the row, so progress looks scattered rather
// than left-to-right (for n=5, row 0: 0 3 1 4 2). Stride 3 is coprime with
// every n not divisible by 3; other n fall back to a +1 stride.
func strided(n, row int) []int {
	stride := 3
	if n%3 == 0 {
		stride = 1
		if n > 1 {
			stride = n - 1
		}
	}
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, (i*stride+row)%n)
	}
	return out
}

// shape builds row labels and component names for an R×C demo grid. The
// first rows and columns use the built-in names; anything beyond is
// generated so any shape can be exercised.
func shape(r, c int) (labels []string, names [][]string) {
	base := []string{"Init", "Services", "Activation", "Migrations", "Warmup", "Routing", "Observability", "Handoff"}
	for i := 0; i < r; i++ {
		if i < len(base) {
			labels = append(labels, base[i])
		} else {
			labels = append(labels, fmt.Sprintf("Group %d", i+1))
		}
		row := make([]string, 0, c)
		for j := 0; j < c; j++ {
			if i < len(defaultNames) && j < len(defaultNames[i]) {
				row = append(row, defaultNames[i][j])
			} else {
				row = append(row, fmt.Sprintf("%s step %d", labels[i], j+1))
			}
		}
		names = append(names, row)
	}
	return labels, names
}

// beatScripts are per-row rhythms for -mode=beats: letters are cells in
// order, "_" is a beat on which the row does nothing. Rows beyond the table
// reuse it cyclically; cells beyond a script run one per beat.
var beatScripts = []string{
	"ABCDE",
	"_AB_CDE",
	"A__BC__DE",
	"ABCD____E",
	"__A_B_CDE",
}

// beatScript expands the row's script to exactly cols cells.
func beatScript(row, cols int) string {
	var b strings.Builder
	cells := 0
	for _, ch := range beatScripts[row%len(beatScripts)] {
		if ch == '_' {
			b.WriteByte('_')
			continue
		}
		if cells == cols {
			break
		}
		b.WriteByte('X')
		cells++
	}
	for ; cells < cols; cells++ {
		b.WriteByte('X')
	}
	return b.String()
}
