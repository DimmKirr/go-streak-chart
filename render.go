package streak

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Message is a status line: the footer line in FooterLayout, or a row's
// text in RowLayout.
type Message struct {
	Text   string
	Status Status
	Level  Level
}

// Lines renders the frame as separate lines without trailing newlines.
//
// FooterLayout: one line per row, the rule, the issue list, then the footer
// message when m.Text is set. RowLayout: one line per row with the row's own
// message after its cells (hidden once the row is done), the rule, then the
// issue list. m is ignored in RowLayout.
//
// The issue list has one line per Warning or Error cell with a note (see
// Grid.SetNote), led by a square in the status color. Theme.HideIssues
// turns it off.
func Lines(g *Grid, m Message, t Theme) []string {
	labelWidth := 0
	for i := 0; i < g.Rows(); i++ {
		if n := lipgloss.Width(g.Label(i)); n > labelWidth {
			labelWidth = n
		}
	}
	matrixWidth := labelWidth + 2 + g.Cols()*lipgloss.Width(t.Glyph)
	if g.Cols() > 1 {
		matrixWidth += (g.Cols() - 1) * lipgloss.Width(t.Gap)
	}

	lines := make([]string, 0, g.Rows()+2)
	for r := 0; r < g.Rows(); r++ {
		var b strings.Builder
		label := g.Label(r)
		b.WriteString(t.Label.Render(label))
		b.WriteString(strings.Repeat(" ", labelWidth-lipgloss.Width(label)+2))
		for c := 0; c < g.Cols(); c++ {
			if c > 0 {
				b.WriteString(t.Gap)
			}
			s, _ := g.Get(r, c)
			b.WriteString(cell(t, t.Glyph, s, CellLevel(s)))
		}
		if t.Layout == RowLayout {
			if rm := g.RowMessage(r); rm.Text != "" && !g.RowDone(r) {
				b.WriteString("  ")
				b.WriteString(t.Message.Render(rm.Text))
			}
		}
		lines = append(lines, b.String())
	}
	lines = append(lines, t.Rule.Render(rule(t, matrixWidth)))
	if !t.HideIssues {
		for _, is := range g.Issues() {
			lines = append(lines, cell(t, t.MessageGlyph, is.Status, MaxLevel)+" "+t.Message.Render(is.Text))
		}
	}
	if t.Layout == FooterLayout && m.Text != "" {
		lines = append(lines, cell(t, t.MessageGlyph, m.Status, m.Level)+" "+t.Message.Render(m.Text))
	}
	return lines
}

// rule returns RuleText when set, otherwise RuleChar repeated to the matrix
// width (labels plus cells) so the separator anchors the frame and does not
// jitter as status text changes.
func rule(t Theme, width int) string {
	if t.RuleText != "" {
		return t.RuleText
	}
	ch := t.RuleChar
	if ch == "" {
		ch = "─"
	}
	return strings.Repeat(ch, width)
}

func cell(t Theme, glyph string, s Status, l Level) string {
	return lipgloss.NewStyle().Foreground(t.CellColor(s, l)).Render(glyph)
}

// Render returns the full frame joined with newlines.
func Render(g *Grid, m Message, t Theme) string {
	return strings.Join(Lines(g, m, t), "\n")
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// StripANSI removes CSI escape sequences (colors and cursor movement). It is
// exported for tests and for hosts that log frames to plain files.
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
