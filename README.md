# go-streak-chart

[![dev](https://github.com/dimmkirr/go-streak-chart/actions/workflows/dev.yaml/badge.svg)](https://github.com/dimmkirr/go-streak-chart/actions/workflows/dev.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/dimmkirr/go-streak-chart.svg)](https://pkg.go.dev/github.com/dimmkirr/go-streak-chart)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A GitHub-contribution-style status matrix for Go loading screens. Rows are
stages, tiles are components, hue is state. The frame is always `rows + 1`
lines, however many components it tracks.

<!-- asset:hero -->
![hero](docs/assets/hero.svg)
<!-- /asset:hero -->

```go
th := streak.DefaultTheme()
th.Layout = streak.RowLayout
g := streak.NewGrid([]string{"Init", "Services", "Activation", "Migrations", "Warmup"}, 10)
l := streak.NewLoader(g, streak.WithTheme(th))
l.Start()
defer l.Close()

// from any goroutine, in any order
l.Set(row, col, streak.Running)
l.RowMessage(row, "Processing Auth Service", streak.Running)
l.Warn(1, 3, "Search Service answered slowly, using cached index")
l.Fail(2, 2, "Health checks failed: 2 of 3 probes timed out")

wg.Wait()
l.Finish("", l.Snapshot().Worst())
```

```sh
go get github.com/dimmkirr/go-streak-chart
```

Requires Go 1.26. Depends on Lip Gloss v1; `teastreak` adds Bubble Tea v1.
Used by [devcell](https://github.com/DimmKirr/devcell) for its `cell open`
loading screen.

## Features

- One tile per component, one line per row, so the frame has a fixed height.
- States: `Pending` (grey), `Running` (blue, pulsing), `Done` (green),
  `Warning` (amber), `Error` (red), each with five intensity levels.
- Warnings and errors stay on their row, in the order they happened.
- Plain output when the writer is not a TTY: one line per state change,
  no redraws.
- No raw mode and no alternate screen. `Finish` is synchronous, so the
  terminal can be handed to another process right after it.
- All methods are safe to call from any goroutine.
- `teastreak` renders the same frame inside a Bubble Tea program.
- `Render(grid, message, theme)` returns one frame as a string, no I/O.

This is not a progress bar: there are no totals, ETAs or byte counters.

## Usage

### Footer layout

The default: tiles per row, one status line under the rule.

<!-- asset:footer-layout -->
![footer-layout](docs/assets/footer-layout.svg)
<!-- /asset:footer-layout -->

```go
g := streak.NewGrid([]string{"Init", "Services", "Activation"}, 5)
l := streak.NewLoader(g, streak.WithWriter(os.Stdout))
l.Start()
defer l.Close() // finishes with the worst status if Finish is skipped

l.Set(0, 0, streak.Running)
l.Message("Processing Init: Config", streak.Running)
// ... work ...
l.Set(0, 0, streak.Done)

l.Finish("All components loaded", streak.Done)
```

### Row layout

Each row carries its own status text and clears it when the row completes.

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

One goroutine per row. Join them, then `Finish` with the worst status.

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

Cells may land in any order:

<!-- asset:scattered -->
![scattered](docs/assets/scattered.svg)
<!-- /asset:scattered -->

Rows may stall and resume independently:

<!-- asset:beats -->
![beats](docs/assets/beats.svg)
<!-- /asset:beats -->

### Warnings and errors

`Warn` and `Fail` set the cell and attach a note. A finished row keeps its
notes on its line; a row whose last message carried `Warning` or `Error`
keeps that message too, for summaries like "Secrets: 18 resolved, 1 failed".

```go
l.Warn(1, 3, "Search Service answered slowly, using cached index")
l.Fail(2, 2, "Health checks failed: 2 of 3 probes timed out")
```

`WithIssues` picks `InlineIssues` (default), `LogIssues` (one line per
note under the rule, which footer layout always uses) or `NoIssues`. Lines
are cut with an ellipsis at the terminal width so a note never wraps.
`WithWidth` overrides the detected width.

### Plain output

When the writer is not a TTY, or with `WithPlain(true)`, the loader logs one
line per state change and never redraws. The final status is written once.

```
[running] Init #1
[running] Processing Init: Config
[done] Init #1
[warning] Services #4: Search Service answered slowly, using cached index
[error] Activation #3: Health checks failed: 2 of 3 probes timed out
```

### Handing the terminal over

`Finish` stops the ticker, waits for it, and writes the final frame plus a
newline in one write. After it returns nothing from the loader can reach
the screen, so `exec` or `os.Exit` is safe on the next line.

- `defer l.Close()` right after `Start`: every exit path stops the ticker.
- Join producers before `Finish`. Mutators called afterwards return
  `ErrFinished` and write nothing.
- Keep other output off the loader's writer while it runs. It redraws by
  moving the cursor up over its own lines.
- On SIGINT call `Finish` or `Close` before exiting.
- `bufio.Writer` and friends are flushed by `Finish`.

### Bubble Tea

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

Send `Finish` after producers are joined and wait for `Run` to return
before using the terminal.

### Theming

Ramps are five colors per status and adapt to light and dark terminals.
The default tile is `■`; `▄` is heavier, `██` with a two-space gap is a
square at full text height.

```go
th := streak.DefaultTheme()
th.Ramp[streak.Done] = streak.Ramp{grey, lime1, lime2, lime3, lime4}
th.Glyph, th.MessageGlyph, th.Gap = "██", "██", "  "
l := streak.NewLoader(g, streak.WithTheme(th))
```

## Development

```sh
task test       # unit tests with -race
task test:e2e   # headless PTY tests via termoscope; artifacts in test/results/
task lint
task assets     # re-record the animations above (or: task assets -- hero)
```

The animations are recorded from [`examples/simple-loader`](examples/simple-loader)
with [termoscope](https://github.com/dimmkirr/termoscope) and embedded between
`<!-- asset:NAME -->` markers in this file. `task docs:check` keeps them in
sync.

## License

MIT
