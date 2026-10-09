# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-10-09

### Added

- feat(theme): change default Glyph/MessageGlyph from "▄" to "■" — tiles keep a visible gap on both axes instead of touching across rows
- feat(example): add -glyph/-gap flags to the simple-loader example — lets a host preview tile shapes like "██" or "●" without code changes

### Changed

- test(render): add a glyph-options table test covering square, half, full, medium-square, circle, block — guards tile, message square and rule width per shape
- test(e2e): add a Glyph PTY check running each option via -glyph/-gap — leaves a screenshot per shape under test/results
- test(all): update frame assertions to expect "■" — matches the new default
- docs(readme): add Go Reference and license badges, a quickstart, a hero scene, a features list, a plain-output example and shorter usage sections — faster overview for new readers
- chore(assets): record scenes at 100 columns instead of 80 and add the hero scene — the re-recorded SVGs show the square tile at the wider width
- chore(deps): bump termoscope to the commit that renamed svganim/gifanim to svg/gif and added termoscope.Options — this repo calls only Start, Record, SavePNG, SaveSVG and the record CLI, so no code changed
- docs(agents): document the termoscope pin rationale, the ■ default tile, the 100x16 asset width and the recording.gif artifact

## [0.2.0] - 2026-10-08

### Added

- feat(loader): add `Close()`, which finishes with the grid's worst status if `Finish` was skipped, so `defer l.Close()` after `Start` guarantees the ticker goroutine stops on every exit path before the host hands the TTY to another process
- feat(loader): add `ErrFinished`, returned by `Set`, `RowMessage`, `Warn` and `Fail` after `Finish` or `Close`; `Message` becomes a no-op, so a straggler goroutine can no longer corrupt the final frame
- feat(loader): `Finish` flushes the writer when it exposes `Flush() error`, covering `bufio.Writer` hosts

### Fixed

- fix(loader): write the final frame and its trailing newline in a single `Write`, so another writer on the same TTY cannot interleave between them

### Changed

- docs: new "Handing the terminal over" section in README and a "Usage" checklist in AGENTS.md spelling out the contract; the example loader defers `Close`
- chore(deps): the e2e PTY harness and README asset recorder now use `github.com/dimmkirr/termoscope` (formerly termproof), pinned to a post-rename pseudo-version until termoscope tags a release; `scripts/assets.sh` runs `go tool termoscope`
- ci: drop the tag workflow's `gh release create` job; releases are drafted first and publishing creates the tag, so the tag push only runs the shared test pipeline

## [0.1.2] - 2026-10-08

### Fixed

- fix(e2e): unset CI in TestMain before the example inherits the environment, so the PTY's COLORTERM=truecolor is honored on GitHub Actions and the hue checks see the real amber and red ramps instead of a 16-color fallback; the library itself is unchanged

### Changed

- docs: add the amber-vs-red symptom and its CI=true reproduction to the flake-triage guidance

## [0.1.1] - 2026-10-08

### Fixed

- fix(e2e): pin termproof v0.1.1, which serializes raster.Render under concurrent calls, so parallel snapshots no longer crash the suite

### Changed

- docs: note the fixed version in the flake-triage guidance

## [0.1.0] - 2026-10-08

### Added

- feat(streak): Grid, Status, Theme and pure Render — a five-level color matrix with footer or per-row status text and a chronological issue log
- feat(loader): cooked-mode inline Loader with pulsing message square and plain-text fallback — redraws in place without raw mode or alt screen, safe from any goroutine
- feat(teastreak): Bubble Tea v1 adapter with typed messages — embed the same frame in a tea.Program
- feat(examples): simple-loader demo with sequential, parallel, scattered and beats modes, injected warnings and errors, -rows/-cols
- feat(streak): a finished row keeps its last message when that message is Warning or Error — summaries like "Secrets: 18 resolved, 1 failed" stay visible
- feat(streak): Theme.Width truncates every line with an ellipsis; Loader reads the TTY width on each draw (WithWidth overrides) and teastreak follows tea.WindowSizeMsg — long inline notes no longer wrap and corrupt the in-place redraw
- feat(streak): Grid gains RowCols/SetRowCols so a row can expose fewer than Cols() active cells, rendered as blank space for alignment and excluded from RowDone
- feat(examples): -issues=inline|log|none replaces the boolean flag
- feat(example): add -rowcols to set each row's active column count independently, falling back to -cols

### Fixed

- fix(theme): pending/level-0 grey is now #484f58 dark / #afb8c1 light (the rule grey) instead of GitHub's #21262d / #ebedf0 — inactive tiles were invisible on black or white backgrounds and downgraded to plain black on 16-color terminals
- fix(loader): frame lines end with \r\n — rows no longer stair-step in raw-mode hosts
- fix(render): size the rule to max(widest line, Theme.Width) instead of the matrix width alone, and cut row/issue/footer lines to Width before measuring it, so a known terminal width always reaches the edge

### Removed

- feat(streak)!: Theme.HideIssues is replaced by Theme.Issues (InlineIssues default, LogIssues, NoIssues) and WithIssueLog by WithIssues in streak and teastreak — in row layout a finished row keeps its notes on its own line, each colored by status, and nothing follows the rule, so a host can start the next program exactly rows+2 lines down. Migration: replace `HideIssues: true` with `Issues: NoIssues` and `WithIssueLog(...)` with `WithIssues(LogIssues)`.

### Changed

- test(e2e): 23 headless PTY sub-tests via termproof with PNG, SVG and recording.svg artifacts per check; unit tests race-clean
- chore(docs): README animations recorded per scene with the termproof CLI (go tool termproof) and embedded between asset markers; docs:check guards assets, markers and embeds in lint
- chore(ci): pr/dev/release workflows sharing a lint, race-unit and e2e pipeline
- chore(deps): require github.com/dimmkirr/termproof v0.1.0 and promote x/ansi and x/term to direct deps — CI no longer fails with "no required module provides package"; the local go.work is no longer needed

[Unreleased]: https://github.com/DimmKirr/go-streak-chart/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/DimmKirr/go-streak-chart/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/DimmKirr/go-streak-chart/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/DimmKirr/go-streak-chart/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/DimmKirr/go-streak-chart/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/DimmKirr/go-streak-chart/commits/v0.1.0
