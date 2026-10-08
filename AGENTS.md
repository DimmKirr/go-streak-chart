# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
`CLAUDE.md` is a symlink to `AGENTS.md`; edit `AGENTS.md`.

## What this is

`streak` is a Go library that renders a GitHub-contribution-style status matrix for loading screens: rows are
components, tiles are cells, hue is state (Pending/Running/Done/Warning/Error), intensity is a 0..4 level. Three
layers: a pure renderer (`Grid`, `Theme`, `Render`), an inline cooked-mode `Loader`, and `teastreak`, a Bubble Tea
v1 adapter. Module `github.com/dimmkirr/go-streak-chart`, Go 1.26 (patch pinned in go.mod on purpose; leave it).
First consumer is devcell's `cell open` loading screen, which hands the TTY to `docker exec` afterwards, so the
Loader must never enter raw mode or the alternate screen.

## Commands

```sh
task test                      # go test -race ./...            (cgo: on nix run CC=cc task test)
task test:e2e                  # go test ./test/e2e/... -v -count=1   (~35s, 23 sub-tests, PTY-backed)
task lint                      # golangci-lint + docs:check
task assets                    # re-record all README animations; task assets -- beats scattered  (subset)
task assets:embed              # only rewrite README embeds between <!-- asset:NAME --> markers
task docs:check                # assets ↔ README markers ↔ scene table consistency (also in CI lint)
task example                   # go run ./examples/simple-loader

go test -run 'TestLoader_Pulse' -v .                               # single unit test
go test ./test/e2e/... -count=1 -run 'TestExample_SimpleLoader/Row/5x10' -v   # single e2e group
go run ./examples/simple-loader -layout=row -mode=scattered -rows=5 -cols=10  # eyeball a scenario
```

Linters: govet, staticcheck, errcheck, revive, gofmt (`.golangci.yml`). gofmt sweeps must exclude `.devcell/`
(dev-container tooling with stray Go files; never edit it). `test/results/` and `.scratch/` are gitignored.

## Local testing: what actually runs

- **Unit tests** (root, `teastreak`) are pure and fast. `loader_test.go` drives the ticker through the unexported
  `clock` interface in `clock.go`; the `tickAcker` hook lets tests block until a tick is processed.
- **`-race` needs cgo.** This nix shell has no `gcc`; export `CC=cc` (clang wrapper) or the race build fails
  with "C compiler gcc not found". CI's ubuntu runner has gcc, so CI never hits this. Plain `go test` is fine
  without it.
- **E2E** (`test/e2e/example_simple_loader_test.go`) builds `examples/simple-loader -fast` with `-buildvcs=false`
  (git stamping fails in sandboxes) and runs it under termproof's headless PTY at 100x20. Tree:
  `TestExample_SimpleLoader/<Mode>/<Shape>/<Check>` with modes Footer, Row (scattered), Parallel, Beats, Plain and
  shapes 3x5, 5x10. Sub-tests run in parallel. Each PTY check leaves PNG + SVG per screen and `recording.svg`
  under `test/results/<ts>-<Test>/`; look at them when a color/layout assertion fails.
- **Screen reads race the child by design.** Wait predicates sample a live screen, so helpers like `rowHues`
  must return "not ready" (nil) when the screen changed between two reads, never index or dereference blindly.
  `shape.complete` is ANDed into every mid-run wait because a 5x10 frame exceeds one PTY read.
- **Flake triage:** an e2e panic inside `termproof/raster` or `x/image/font` is termproof's concurrent-Render bug
  (present in termproof v0.1.0, fixed in v0.1.1, which go.mod now pins); a panic in this repo's test helpers is a missing nil/mid-redraw guard; a
  single failure right after a `-race` build that does not reproduce is the cgo/non-cgo build-cache switch;
  "want amber, got red" on every hue check is the color profile: termenv reports "not a TTY" whenever `CI` is set,
  so `test/e2e/main_test.go` unsets it before the example inherits the environment (reproduce with `CI=true go test`).
- CI (`.github/workflows/test.yaml`, shared by pr/dev/release) runs unit tests with `-race` excluding `/test/e2e`,
  then e2e separately and uploads `test/results/` as the `e2e-screenshots` artifact.

## External dependencies

- **Library deps:** root imports only Lip Gloss v1; `teastreak` adds Bubble Tea v1. Stay on v1 of both (devcell
  pins bubbletea 1.3.x / lipgloss 1.1.x). go-cmp and termenv are test-only.
- **termproof** (`github.com/dimmkirr/termproof`, lowercase path) is test/tooling-only: imported by
  `test/e2e` and run as `go tool termproof` (a `tool` directive in go.mod) by `scripts/assets.sh`. Pinned to a
  tagged release in go.mod. For local work against a sibling checkout use a workspace and do not commit it:
  `go work init . ../termproof` (go.work is gitignored). Workspace mode cannot satisfy a placeholder version, so
  the require line must always be a real tag/pseudo-version or CI fails with "no required module provides
  package". After editing go.mod, tidy with `GOWORK=off`.
- **Tool invocations set `CGO_ENABLED=0`** (`scripts/assets.sh`, Taskfile) so `go build`/`go run` of helpers work
  without a C compiler; only `-race` needs cgo.
- **Chromium** is optional: termproof's own font test uses it; nothing here does. `.scratch/` is where ad-hoc
  screenshots and tools go.

## Architecture

- **Model** (`grid.go`, `status.go`): `Grid` holds labels, a fixed column count, cell `Status`es, one `Message`
  per row and per-cell notes with an insertion sequence so `Issues()` is chronological even when rows finish out
  of order. Bounds errors are returned (`ErrOutOfRange`), never panicked. `Status.Severity()` orders
  Error > Warning > Running > Pending > Done for `Worst()`. `Level` 0..4 indexes the five-color ramps.
- **Presentation** (`theme.go`, `render.go`): `Theme` carries `Layout` (FooterLayout: one status line under the
  rule; RowLayout: text after each row), `Issues` (InlineIssues default: a finished row keeps its notes on its
  line, each colored by status, nothing under the rule, so the frame is always rows+1 lines; LogIssues: one line
  per note under the rule, which FooterLayout always uses; NoIssues), `Width` (0 = unlimited; Loader fills it
  from the TTY per draw, teastreak from WindowSizeMsg; lines are cut with `…` so inline notes never wrap and
  break the redraw), `Ramp map[Status]Ramp`,
  glyphs and rule chars. `Render`/`Lines` are pure and deterministic; the rule spans the full `Width` when it is
  known (Loader/teastreak), else the matrix width, and never the text, so it never jitters. Default tile is `▄` with a one-space gap, chosen so tile:gap is 1:1 on
  both axes (see the Linear design doc linked from README).
- **Loader** (`loader.go`, `clock.go`): all methods take one mutex. `Start` spawns a ticker goroutine that
  pulses the message square's level 1→4→1. Redraw is `\r` + `ESC[nA` + `ESC[J` + frame, with frame lines joined
  by `\r\n` so it stays aligned in raw-mode hosts; it only redraws after the first tick (ghost-row fix mirrored
  from devcell). Plain mode (`WithPlain(true)` or a non-TTY writer) logs one line per change. `Finish` is
  idempotent, draws the final frame and appends a newline. `Snapshot()` returns a `Grid` clone for post-join
  summaries.
- **teastreak** (`teastreak/model.go`): `tea.Model` whose `View` delegates to `Render`; typed messages
  (`SetStatus`, `SetMessage`, `SetRowMessage`, `Warn`, `Fail`, `Finish`) mirror the Loader API. Public on purpose
  (consumers import it); `internal/` is reserved for private tooling.
- **Example** (`examples/simple-loader`): the fixture for e2e and asset recording. `-mode` selects sequential,
  parallel (rows concurrent), scattered (two workers per row over a strided cell order), beats (rows in lockstep
  on stall scripts); row-based modes imply `-layout=row`. It injects one warning and three errors and exits 1
  when it did, which is part of the demo, so recorders ignore its exit code.
- **README assets** (`scripts/assets.sh`): scene table `<name> <flags>`, one SVG per scene recorded at 80x16,
  embedded between `<!-- asset:NAME -->` markers. Adding a scene means: a table line, a marker pair in README,
  then `task assets -- NAME`; `docs:check` enforces all three.
