package streak

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-cmp/cmp"
	"github.com/muesli/termenv"
)

func TestRender_Layout(t *testing.T) {
	g := NewGrid([]string{"Init", "Activation"}, 3)
	_ = g.Set(0, 0, Done)
	out := Render(g, Message{Text: "Processing Service: Messaging Service", Status: Running, Level: 2}, DefaultTheme())
	lines := strings.Split(StripANSI(out), "\n")
	want := []string{
		"Init        ■ ■ ■",
		"Activation  ■ ■ ■",
		"─────────────────",
		"■ Processing Service: Messaging Service",
	}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Fatal(diff)
	}
}

// glyphOptions are the tile shapes worth comparing against the default
// "■". Shared with the e2e Glyph group, which records each one. "██" is the
// only text-height square: two full blocks with a two-space gap keep
// tile:gap at 1:1 horizontally. The rest are single-cell glyphs whose
// rendered size depends on the font.
var glyphOptions = []struct{ name, glyph, gap string }{
	{"square", "■", " "},
	{"half", "▄", " "},
	{"full", "██", "  "},
	{"medium-square", "◼", " "},
	{"circle", "●", " "},
	{"block", "█", " "},
}

// TestRender_GlyphOptions renders every glyph option through the default
// theme and checks the tiles, the message square and the rule width.
func TestRender_GlyphOptions(t *testing.T) {
	for _, tc := range glyphOptions {
		t.Run(tc.name, func(t *testing.T) {
			th := DefaultTheme()
			th.Glyph, th.MessageGlyph, th.Gap = tc.glyph, tc.glyph, tc.gap
			g := NewGrid([]string{"Init", "Activation"}, 3)
			_ = g.Set(0, 0, Done)
			out := Render(g, Message{Text: "Processing Service: Messaging Service", Status: Running, Level: 2}, th)
			lines := strings.Split(StripANSI(out), "\n")
			row := strings.TrimSuffix(strings.Repeat(tc.glyph+tc.gap, 3), tc.gap)
			want := []string{
				"Init        " + row,
				"Activation  " + row,
				strings.Repeat("─", lipgloss.Width("Activation  "+row)),
				tc.glyph + " Processing Service: Messaging Service",
			}
			if diff := cmp.Diff(want, lines); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestRender_EmptyMessageOmitsLine(t *testing.T) {
	g := NewGrid([]string{"A"}, 1)
	out := StripANSI(Render(g, Message{}, DefaultTheme()))
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("want grid+rule only, got %q", out)
	}
}

func TestLines_CountMatchesRender(t *testing.T) {
	g := NewGrid([]string{"A", "B"}, 2)
	if n := len(Lines(g, Message{Text: "x"}, DefaultTheme())); n != 4 {
		t.Fatalf("got %d lines", n)
	}
}

func TestStripANSI(t *testing.T) {
	if got := StripANSI("\x1b[38;2;1;2;3m██\x1b[0m x\x1b[2A\x1b[J"); got != "██ x" {
		t.Fatalf("got %q", got)
	}
}

// assertRuleWidth checks the rule is made of the rule char and is exactly as
// wide as the widest line of the frame or the theme width, whichever is
// larger, and returns that width.
func assertRuleWidth(t *testing.T, g *Grid, th Theme) int {
	t.Helper()
	lines := Lines(g, Message{}, th)
	rule := StripANSI(lines[len(lines)-1])
	if strings.Trim(rule, "─") != "" {
		t.Fatalf("rule must be made of the rule char, got %q", rule)
	}
	want := th.Width
	for _, l := range lines[:len(lines)-1] {
		if n := len([]rune(StripANSI(l))); n > want {
			want = n
		}
	}
	if got := len([]rune(rule)); got != want {
		t.Fatalf("rule width %d, want %d = max(widest line, Width %d):\n%s", got, want, th.Width, StripANSI(strings.Join(lines, "\n")))
	}
	return want
}

func TestRender_RuleSpansWidestRow(t *testing.T) {
	g := NewGrid([]string{"A", "LongerLabel"}, 2)
	if got, want := assertRuleWidth(t, g, DefaultTheme()), len([]rune("LongerLabel  ■ ■")); got != want {
		t.Fatalf("rule width %d, want %d", got, want)
	}
}

func TestRender_RuleSpansWidthWhenSet(t *testing.T) {
	th := DefaultTheme()
	th.Width = 40
	if got := assertRuleWidth(t, NewGrid([]string{"A"}, 1), th); got != 40 {
		t.Fatalf("rule width %d, want the theme width 40", got)
	}
}

func TestRender_RuleTextOverridesSpan(t *testing.T) {
	th := DefaultTheme()
	th.RuleText = "---"
	lines := Lines(NewGrid([]string{"A"}, 1), Message{}, th)
	if got := StripANSI(lines[len(lines)-1]); got != "---" {
		t.Fatalf("got %q", got)
	}
}

func rowTheme() Theme {
	th := DefaultTheme()
	th.Layout = RowLayout
	return th
}

func TestRender_RowLayout_ShowsTextPerRow(t *testing.T) {
	g := NewGrid([]string{"Init", "Activation"}, 3)
	_ = g.Set(0, 0, Done)
	_ = g.Set(0, 1, Running)
	_ = g.SetRowMessage(0, Message{Text: "Processing Init: Logger", Status: Running})
	_ = g.SetRowMessage(1, Message{Text: "Waiting", Status: Pending})
	lines := strings.Split(StripANSI(Render(g, Message{Text: "ignored footer"}, rowTheme())), "\n")
	want := []string{
		"Init        ■ ■ ■  Processing Init: Logger",
		"Activation  ■ ■ ■  Waiting",
		strings.Repeat("─", 42), // the widest line, text included
	}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Fatal(diff)
	}
}

func TestRender_RowLayout_HidesTextWhenRowDone(t *testing.T) {
	g := NewGrid([]string{"Init"}, 2)
	_ = g.Set(0, 0, Done)
	_ = g.Set(0, 1, Warning)
	_ = g.SetRowMessage(0, Message{Text: "Processing Init: Logger", Status: Running})
	lines := strings.Split(StripANSI(Render(g, Message{}, rowTheme())), "\n")
	if lines[0] != "Init  ■ ■" {
		t.Fatalf("done row must drop its text, got %q", lines[0])
	}
}

func TestRender_RowLayout_NoFooterLine(t *testing.T) {
	g := NewGrid([]string{"A"}, 1)
	lines := Lines(g, Message{Text: "footer", Status: Running}, rowTheme())
	if len(lines) != 2 {
		t.Fatalf("row layout must render rows + rule only, got %d lines", len(lines))
	}
}

// Without a known width the rule follows the widest line, text included;
// with one, lines are cut to it and the rule spans exactly the width.
func TestRender_RowLayout_RuleSpansWidestLine(t *testing.T) {
	g := NewGrid([]string{"A"}, 1)
	_ = g.SetRowMessage(0, Message{Text: strings.Repeat("x", 40), Status: Running})
	th := rowTheme()
	if got, want := assertRuleWidth(t, g, th), len([]rune("A  ■  "))+40; got != want {
		t.Fatalf("rule width %d, want %d (row text included)", got, want)
	}
	th.Width = 20
	if got := assertRuleWidth(t, g, th); got != 20 {
		t.Fatalf("rule width %d, want the theme width 20", got)
	}
}

func TestRender_FooterLayout_IgnoresRowMessages(t *testing.T) {
	g := NewGrid([]string{"A"}, 1)
	_ = g.SetRowMessage(0, Message{Text: "row text", Status: Running})
	out := StripANSI(Render(g, Message{Text: "footer"}, DefaultTheme()))
	if strings.Contains(out, "row text") {
		t.Fatalf("footer layout must not show row text:\n%s", out)
	}
}

func issueGrid() *Grid {
	g := NewGrid([]string{"Init", "Activation"}, 3)
	for c := 0; c < 3; c++ {
		_ = g.Set(0, c, Done)
		_ = g.Set(1, c, Done)
	}
	_ = g.Set(1, 1, Error)
	_ = g.SetNote(1, 1, "Build of ABC failed")
	_ = g.Set(0, 2, Warning)
	_ = g.SetNote(0, 2, "Activation X did not go through")
	return g
}

func TestRender_FooterLayout_ListsIssuesBetweenRuleAndFooter(t *testing.T) {
	lines := strings.Split(StripANSI(Render(issueGrid(), Message{Text: "Loading finished with errors", Status: Error, Level: 4}, DefaultTheme())), "\n")
	want := []string{
		"Init        ■ ■ ■",
		"Activation  ■ ■ ■",
		"─────────────────",
		"■ Build of ABC failed",
		"■ Activation X did not go through",
		"■ Loading finished with errors",
	}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Fatal(diff)
	}
}

func TestRender_IssueSquareUsesStatusColor(t *testing.T) {
	th := DefaultTheme()
	th.Ramp[Error] = Ramp{lipgloss.Color("1"), lipgloss.Color("1"), lipgloss.Color("1"), lipgloss.Color("1"), lipgloss.Color("#ff0000")}
	th.Ramp[Warning] = Ramp{lipgloss.Color("3"), lipgloss.Color("3"), lipgloss.Color("3"), lipgloss.Color("3"), lipgloss.Color("#ffaa00")}
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	lines := Lines(issueGrid(), Message{}, th)
	if !strings.Contains(lines[4], "255;170;0") {
		t.Fatalf("warning issue square must use the warning ramp top color: %q", lines[4])
	}
	if !strings.Contains(lines[3], "255;0;0") {
		t.Fatalf("error issue square must use the error ramp top color: %q", lines[3])
	}
}

// Inline issues are the default: in RowLayout a finished row keeps its
// notes on its own line and nothing is printed under the rule, so a host
// can start the next program exactly rows+2 lines down.
func TestRender_RowLayout_InlineIssuesByDefault(t *testing.T) {
	lines := strings.Split(StripANSI(Render(issueGrid(), Message{}, rowTheme())), "\n")
	want := []string{
		"Init        ■ ■ ■  Activation X did not go through",
		"Activation  ■ ■ ■  Build of ABC failed",
		strings.Repeat("─", 50), // the widest line, text included
	}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Fatal(diff)
	}
}

func TestRender_RowLayout_InlineIssues_JoinsNotesInOrderRecorded(t *testing.T) {
	g := NewGrid([]string{"Init"}, 3)
	for c := 0; c < 3; c++ {
		_ = g.Set(0, c, Done)
	}
	_ = g.Set(0, 2, Error)
	_ = g.SetNote(0, 2, "second")
	_ = g.Set(0, 0, Warning)
	_ = g.SetNote(0, 0, "first")
	lines := strings.Split(StripANSI(Render(g, Message{}, rowTheme())), "\n")
	if lines[0] != "Init  ■ ■ ■  second; first" {
		t.Fatalf("notes must be joined in the order recorded, got %q", lines[0])
	}
}

// A done row whose last message carries Warning or Error keeps that message
// inline even without cell notes (e.g. "Secrets: 18 resolved, 1 failed").
func TestRender_RowLayout_InlineIssues_KeepsWarningRowMessage(t *testing.T) {
	g := NewGrid([]string{"Configure"}, 2)
	_ = g.Set(0, 0, Done)
	_ = g.Set(0, 1, Warning)
	_ = g.SetRowMessage(0, Message{Text: "Secrets: 18 resolved, 1 failed", Status: Warning})
	lines := strings.Split(StripANSI(Render(g, Message{}, rowTheme())), "\n")
	if lines[0] != "Configure  ■ ■  Secrets: 18 resolved, 1 failed" {
		t.Fatalf("warning row message must stay after completion, got %q", lines[0])
	}
	if len(lines) != 2 {
		t.Fatalf("nothing may follow the rule, got %d lines:\n%s", len(lines), strings.Join(lines, "\n"))
	}
}

func TestRender_RowLayout_InlineIssues_DropsDoneRowMessage(t *testing.T) {
	g := NewGrid([]string{"Boot"}, 2)
	_ = g.Set(0, 0, Done)
	_ = g.Set(0, 1, Done)
	_ = g.SetRowMessage(0, Message{Text: "Session services ready", Status: Done})
	lines := strings.Split(StripANSI(Render(g, Message{}, rowTheme())), "\n")
	if lines[0] != "Boot  ■ ■" {
		t.Fatalf("a clean done row shows no text, got %q", lines[0])
	}
}

func TestRender_RowLayout_InlineIssues_ColoredByStatus(t *testing.T) {
	g := NewGrid([]string{"Init"}, 1)
	_ = g.Set(0, 0, Error)
	_ = g.SetNote(0, 0, "boom")
	out := Render(g, Message{}, rowTheme())
	red := lipgloss.NewStyle().Foreground(DefaultTheme().CellColor(Error, MaxLevel)).Render("boom")
	if !strings.Contains(out, red) {
		t.Fatalf("inline note must use the Error color:\n%q", out)
	}
}

func TestRender_RowLayout_LogIssues_OptIn(t *testing.T) {
	th := rowTheme()
	th.Issues = LogIssues
	lines := strings.Split(StripANSI(Render(issueGrid(), Message{}, th)), "\n")
	want := []string{
		"Init        ■ ■ ■",
		"Activation  ■ ■ ■",
		"─────────────────",
		"■ Build of ABC failed",
		"■ Activation X did not go through",
	}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Fatal(diff)
	}
}

// FooterLayout has no row text, so inline falls back to the list under the
// rule (the footer line stays last).
func TestRender_FooterLayout_InlineFallsBackToList(t *testing.T) {
	th := DefaultTheme()
	if th.Issues != InlineIssues || th.Layout != FooterLayout {
		t.Fatal("test assumes the defaults are FooterLayout + InlineIssues")
	}
	lines := strings.Split(StripANSI(Render(issueGrid(), Message{Text: "done", Status: Error}, th)), "\n")
	if len(lines) != 6 || lines[3] != "■ Build of ABC failed" || lines[5] != "■ done" {
		t.Fatalf("footer layout must list issues under the rule:\n%s", strings.Join(lines, "\n"))
	}
}

func TestRender_NoIssues(t *testing.T) {
	for _, layout := range []Layout{FooterLayout, RowLayout} {
		th := DefaultTheme()
		th.Layout = layout
		th.Issues = NoIssues
		out := StripANSI(Render(issueGrid(), Message{}, th))
		if strings.Contains(out, "failed") || strings.Contains(out, "go through") {
			t.Fatalf("%v: issues must be hidden:\n%s", layout, out)
		}
	}
}

func TestIssueMode_String(t *testing.T) {
	for m, want := range map[IssueMode]string{InlineIssues: "inline", LogIssues: "log", NoIssues: "none", IssueMode(9): "IssueMode(9)"} {
		if got := m.String(); got != want {
			t.Errorf("%d: got %q want %q", m, got, want)
		}
	}
}

// Theme.Width caps every line so inline notes never wrap the terminal,
// which would break the in-place redraw and the rows+2 guarantee.
func TestLines_TruncatesToThemeWidth(t *testing.T) {
	g := NewGrid([]string{"Services"}, 3)
	for c := 0; c < 3; c++ {
		_ = g.Set(0, c, Done)
	}
	_ = g.Set(0, 1, Error)
	_ = g.SetNote(0, 1, "Messaging Service: broker handshake timed out after thirty seconds")
	th := rowTheme()
	th.Width = 30
	for _, l := range Lines(g, Message{}, th) {
		if w := lipgloss.Width(l); w > 30 {
			t.Errorf("line wider than Width: %d %q", w, StripANSI(l))
		}
	}
	first := StripANSI(Lines(g, Message{}, th)[0])
	if !strings.HasSuffix(first, "…") || !strings.HasPrefix(first, "Services  ■ ■ ■  Messaging") {
		t.Fatalf("row must be cut with an ellipsis, got %q", first)
	}
	th.Width = 0
	if got := StripANSI(Lines(g, Message{}, th)[0]); !strings.HasSuffix(got, "thirty seconds") {
		t.Fatalf("Width 0 means unlimited, got %q", got)
	}
}
