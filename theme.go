package streak

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Layout chooses where status text appears.
type Layout int

const (
	// FooterLayout shows one status line under the rule, led by a pulsing
	// square:
	//
	//	Init        ▄ ▄ ▄
	//	─────────────────
	//	▄ Processing Init: Config
	FooterLayout Layout = iota
	// RowLayout shows each row's own status text after its cells and drops
	// it once the row is done. No footer line is rendered:
	//
	//	Init        ▄ ▄ ▄  Processing Init: Config
	//	Services    ▄ ▄ ▄  Waiting
	//	─────────────────
	RowLayout
)

func (l Layout) String() string {
	switch l {
	case FooterLayout:
		return "footer"
	case RowLayout:
		return "row"
	}
	return fmt.Sprintf("Layout(%d)", int(l))
}

// Ramp is five colors indexed by Level.
type Ramp [MaxLevel + 1]lipgloss.TerminalColor

// Theme controls colors and glyphs. Zero fields fall back to DefaultTheme
// values only where documented; callers should start from DefaultTheme and
// override.
type Theme struct {
	// Layout selects FooterLayout (default) or RowLayout.
	Layout Layout
	// Issues selects where Warning and Error notes are shown. The zero
	// value, InlineIssues, keeps them on their row in RowLayout (falling
	// back to a log under the rule in FooterLayout, which has no row text);
	// LogIssues always lists them under the rule; NoIssues hides them.
	Issues IssueMode
	// Ramp maps each status to its five-step intensity ramp.
	Ramp map[Status]Ramp
	// Label styles the row labels.
	Label lipgloss.Style
	// Message styles the text after the message square.
	Message lipgloss.Style
	// Rule styles the separator between the matrix and the message.
	Rule lipgloss.Style
	// Glyph is one matrix cell. Default "▄": a lower half block is roughly
	// square in a terminal cell, and with the default one-space Gap the tile
	// to gap ratio is 1:1 on both axes, like GitHub's contribution graph.
	Glyph string
	// MessageGlyph is the square before the message text. Default "▄".
	MessageGlyph string
	// Gap separates cells. Default " ".
	Gap string
	// RuleChar is repeated to span the widest row. Default "─".
	RuleChar string
	// Width, when > 0, caps every rendered line at this many columns with
	// an ellipsis, so long inline notes never wrap the terminal (a wrapped
	// line breaks the in-place redraw). Loader fills it from the TTY size;
	// teastreak from tea.WindowSizeMsg. 0 means unlimited.
	Width int
	// RuleText, when set, replaces the spanning rule with literal text.
	RuleText string
}

func adaptive(light, dark string) lipgloss.TerminalColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// DefaultTheme uses GitHub's contribution-graph ramp (light and dark) for
// Done and analogous five-step ramps for the other statuses. Pending is the
// flat level-0 grey.
func DefaultTheme() Theme {
	// Pending/level-0 grey matches the rule: visible on black and white
	// backgrounds and on 256- and 16-color terminals (GitHub's #21262d
	// level-0 square downgrades to black and disappears there).
	grey := adaptive("#afb8c1", "#484f58")
	return Theme{
		Ramp: map[Status]Ramp{
			Pending: {grey, grey, grey, grey, grey},
			Done: {grey,
				adaptive("#9be9a8", "#0e4429"), adaptive("#40c463", "#006d32"),
				adaptive("#30a14e", "#26a641"), adaptive("#216e39", "#39d353")},
			Running: {grey,
				adaptive("#b6e3ff", "#0a3069"), adaptive("#54aeff", "#0550ae"),
				adaptive("#0969da", "#1f6feb"), adaptive("#0550ae", "#58a6ff")},
			Warning: {grey,
				adaptive("#fff8c5", "#3b2300"), adaptive("#f2cc60", "#7d4e00"),
				adaptive("#d4a72c", "#bb8009"), adaptive("#9a6700", "#d29922")},
			Error: {grey,
				adaptive("#ffebe9", "#3c0d0d"), adaptive("#ff8182", "#8e1519"),
				adaptive("#fa4549", "#da3633"), adaptive("#cf222e", "#f85149")},
		},
		Label:        lipgloss.NewStyle().Foreground(adaptive("#57606a", "#8b949e")),
		Message:      lipgloss.NewStyle(),
		Rule:         lipgloss.NewStyle().Foreground(adaptive("#d0d7de", "#484f58")),
		Glyph:        "▄",
		MessageGlyph: "▄",
		Gap:          " ",
		RuleChar:     "─",
	}
}

// CellColor returns the ramp color for a status at a level. The level is
// clamped and unknown statuses use the Pending ramp.
func (t Theme) CellColor(s Status, l Level) lipgloss.TerminalColor {
	r, ok := t.Ramp[s]
	if !ok {
		r = t.Ramp[Pending]
	}
	return r[l.Clamp()]
}

// CellLevel is the intensity of a matrix cell: Pending is 0, everything
// else is fully saturated.
func CellLevel(s Status) Level {
	if s == Pending {
		return 0
	}
	return MaxLevel
}

// IssueMode selects where Warning and Error notes appear.
type IssueMode int

const (
	// InlineIssues (default) keeps a finished row's notes on its own line
	// in RowLayout, so nothing is printed under the rule and a host can
	// start the next program exactly rows+2 lines down. FooterLayout has no
	// row text, so it logs under the rule instead.
	InlineIssues IssueMode = iota
	// LogIssues lists notes under the rule, one line each, chronologically.
	LogIssues
	// NoIssues hides notes entirely.
	NoIssues
)

// String returns the mode name used by flags and docs.
func (m IssueMode) String() string {
	switch m {
	case InlineIssues:
		return "inline"
	case LogIssues:
		return "log"
	case NoIssues:
		return "none"
	}
	return fmt.Sprintf("IssueMode(%d)", int(m))
}
