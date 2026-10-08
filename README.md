# go-streak-chart

[![dev](https://github.com/dimmkirr/go-streak-chart/actions/workflows/dev.yaml/badge.svg)](https://github.com/dimmkirr/go-streak-chart/actions/workflows/dev.yaml)

A GitHub-contribution-style status matrix for Go loading screens. Each
component is a tile; hue encodes state, intensity encodes progress.

<!-- asset:footer-layout -->
![footer-layout](docs/assets/footer-layout.svg)
<!-- /asset:footer-layout -->

States: `Pending` (grey), `Running` (blue, pulsing), `Done` (green),
`Warning` (amber), `Error` (red).

```sh
go get github.com/dimmkirr/go-streak-chart
```

Requires Go 1.26. Depends on Lip Gloss v1 and Bubble Tea v1.

## Usage

`Loader` redraws in place without raw mode or the alternate screen, so the
terminal can be handed to another process afterwards. When stdout is not a
TTY it prints one plain line per state change instead. All methods are safe
to call from any goroutine.

```go
g := streak.NewGrid([]string{"Init", "Services", "Activation"}, 5)
l := streak.NewLoader(g, streak.WithWriter(os.Stdout))
l.Start()

l.Set(0, 0, streak.Running)
l.Message("Processing Init: Config", streak.Running)
// ... do the work ...
l.Set(0, 0, streak.Done)

l.Finish("All components loaded", streak.Done)
```

### Bubble Tea

`teastreak.Model` renders the same frame inside a Bubble Tea program:

```go
m := teastreak.New(streak.NewGrid(rows, 5))
p := tea.NewProgram(m)
go func() {
    p.Send(teastreak.SetStatus(0, 0, streak.Running))
    p.Send(teastreak.SetMessage("Processing Init: Config", streak.Running))
    p.Send(teastreak.Finish("All components loaded", streak.Done))
}()
_, err := p.Run()
```

### Row layout

`RowLayout` shows each row's own status text after its tiles and clears it
when the row completes:

<!-- asset:row-layout -->
![row-layout](docs/assets/row-layout.svg)
<!-- /asset:row-layout -->

```go
th := streak.DefaultTheme()
th.Layout = streak.RowLayout
l := streak.NewLoader(g, streak.WithTheme(th))
l.RowMessage(1, "Processing Messaging Service", streak.Running)
```

### Concurrent rows

Drive rows from separate goroutines; `Snapshot().Worst()` summarizes the
outcome once they join:

<!-- asset:parallel-rows -->
![parallel-rows](docs/assets/parallel-rows.svg)
<!-- /asset:parallel-rows -->

```go
var wg sync.WaitGroup
for r := range rows {
    wg.Add(1)
    go func(r int) {
        defer wg.Done()
        for c := range cols {
            l.Set(r, c, streak.Running)
            l.RowMessage(r, "Processing "+name(r, c), streak.Running)
            // ... work ...
            l.Set(r, c, streak.Done)
        }
    }(r)
}
wg.Wait()
l.Finish("", l.Snapshot().Worst())
```

Cells may complete in any order:

<!-- asset:scattered -->
![scattered](docs/assets/scattered.svg)
<!-- /asset:scattered -->

Rows may also pause and resume independently:

<!-- asset:beats -->
![beats](docs/assets/beats.svg)
<!-- /asset:beats -->

### Warnings and errors

`Warn` and `Fail` set the cell and attach a note. In row layout a finished
row keeps its notes on its own line, in the order they happened and colored
by status, and nothing is printed under the rule. The frame is always
`rows + 1` lines, so a host can hand the terminal to the next process right
below it. A finished row whose last message carries `Warning` or `Error`
keeps that message too, for summaries like "Secrets: 18 resolved, 1 failed".

```go
l.Warn(1, 3, "Search Service answered slowly, using cached index")
l.Fail(2, 2, "Health checks failed: 2 of 3 probes timed out")
```

`WithIssues` selects the mode: `InlineIssues` (default), `LogIssues` (one
line per note under the rule; what footer layout always uses, since it has
no row text) or `NoIssues`. Lines are cut with an ellipsis at the terminal
width so a long note never wraps, and the rule stretches to that width;
`WithWidth` overrides the detected width, and `teastreak` follows
`tea.WindowSizeMsg`.

### Theming

Start from `DefaultTheme()`. Ramps are five colors per status; adaptive
colors switch between light and dark terminals.

```go
th := streak.DefaultTheme()
th.Ramp[streak.Done] = streak.Ramp{grey, lime1, lime2, lime3, lime4}
th.Glyph = "██"
l := streak.NewLoader(g, streak.WithTheme(th))
```

`streak.Render(grid, message, theme)` returns one frame as a string with no
I/O, for custom hosts and tests.

## Development

```sh
task test       # unit tests with -race
task test:e2e   # headless PTY tests via termproof; artifacts in test/results/
task lint
task assets     # re-record the animations above (or: task assets -- beats)
```

The animations are recorded from `examples/simple-loader` with
[termproof](https://github.com/dimmkirr/termproof) and embedded between the
`<!-- asset:NAME -->` markers in this file. `task docs:check` keeps them in
sync.

## License

MIT
