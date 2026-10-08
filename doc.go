// Package streak renders a GitHub-contribution-style status matrix for
// multi-component loading screens.
//
// Hue encodes state (pending, running, done, warning, error) and intensity
// encodes progress using GitHub's five contribution levels. The package has
// three layers: a pure renderer (Grid, Theme, Render), an inline cooked-mode
// Loader that redraws in place without raw mode or the alternate screen, and
// the teastreak sub-package that adapts the renderer to Bubble Tea.
//
// README animations are regenerated with `go generate ./...`.
//
//go:generate sh scripts/assets.sh
package streak
