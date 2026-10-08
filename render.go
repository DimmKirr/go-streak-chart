package streak

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
// Grid.SetNote), led by a square in the status color. Theme.Issues selects
// inline (default), log or none; see IssueMode.
func Lines(g *Grid, m Message, t Theme) []string {
	labelWidth := 0
	for i := 0; i < g.Rows(); i++ {
		if n := lipgloss.Width(g.Label(i)); n > labelWidth {
			labelWidth = n
		}
	}
	lines := make([]string, 0, g.Rows()+2)
	for r := 0; r < g.Rows(); r++ {
		var b strings.Builder
		label := g.Label(r)
		b.WriteString(t.Label.Render(label))
		b.WriteString(strings.Repeat(" ", labelWidth-lipgloss.Width(label)+2))
		active := g.RowCols(r)
		for c := 0; c < g.Cols(); c++ {
			if c > 0 {
				b.WriteString(t.Gap)
			}
			if c < active {
				s, _ := g.Get(r, c)
				b.WriteString(cell(t, t.Glyph, s, CellLevel(s)))
			} else {
				b.WriteString(strings.Repeat(" ", lipgloss.Width(t.Glyph)))
			}
		}
		if t.Layout == RowLayout {
			if segs := rowSegments(g, r, t); len(segs) > 0 {
				b.WriteString("  ")
				b.WriteString(renderSegments(t, segs))
			}
		}
		lines = append(lines, fit(t, b.String()))
	}
	ruleWidth := t.Width
	for _, l := range lines {
		if n := lipgloss.Width(l); n > ruleWidth {
			ruleWidth = n
		}
	}
	lines = append(lines, t.Rule.Render(rule(t, ruleWidth)))
	if logIssues(t) {
		for _, is := range g.Issues() {
			lines = append(lines, fit(t, cell(t, t.MessageGlyph, is.Status, MaxLevel)+" "+t.Message.Render(is.Text)))
		}
	}
	if t.Layout == FooterLayout && m.Text != "" {
		lines = append(lines, fit(t, cell(t, t.MessageGlyph, m.Status, m.Level)+" "+t.Message.Render(m.Text)))
	}
	return lines
}

// fit cuts a line to Theme.Width with an ellipsis; 0 leaves it untouched.
func fit(t Theme, line string) string {
	if t.Width > 0 {
		return ansi.Truncate(line, t.Width, "…")
	}
	return line
}

// rule returns RuleText when set, otherwise RuleChar repeated to width: the
// larger of Theme.Width and the widest row line. With a known Width (every
// TTY host) rows are cut to it, so the rule always spans the terminal and
// never jitters; without one it spans the widest row, text included.
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

// issueStyle colors text by status: Warning and Error use the top of their
// ramp so an inline note reads as such; everything else uses Theme.Message.
func issueStyle(t Theme, s Status) lipgloss.Style {
	if s == Warning || s == Error {
		return t.Message.Foreground(t.CellColor(s, MaxLevel))
	}
	return t.Message
}

// logIssues reports whether notes are listed under the rule: always in
// LogIssues; in InlineIssues only for FooterLayout, which has no row text.
func logIssues(t Theme) bool {
	switch t.Issues {
	case LogIssues:
		return true
	case InlineIssues:
		return t.Layout == FooterLayout
	}
	return false
}

// segment is one colored run of a row's inline text.
type segment struct {
	text   string
	status Status
}

// rowSegments returns the text shown after a row's cells in RowLayout: the
// live row message while the row is running; once the row is done, its
// Warning and Error notes in the order recorded, each in its own status
// color (InlineIssues), or the row message itself when that carries a
// Warning or Error status.
func rowSegments(g *Grid, r int, t Theme) []segment {
	rm := g.RowMessage(r)
	if !g.RowDone(r) {
		if rm.Text == "" {
			return nil
		}
		return []segment{{rm.Text, rm.Status}}
	}
	if t.Issues != InlineIssues {
		return nil
	}
	var segs []segment
	for _, is := range g.Issues() {
		if is.Row == r {
			segs = append(segs, segment{is.Text, is.Status})
		}
	}
	if len(segs) == 0 && rm.Text != "" && (rm.Status == Warning || rm.Status == Error) {
		segs = append(segs, segment{rm.Text, rm.Status})
	}
	return segs
}

// renderSegments joins segments with "; ", each colored by its status.
func renderSegments(t Theme, segs []segment) string {
	var b strings.Builder
	for i, s := range segs {
		if i > 0 {
			b.WriteString(t.Message.Render("; "))
		}
		b.WriteString(issueStyle(t, s.status).Render(s.text))
	}
	return b.String()
}
