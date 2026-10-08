// Package teastreak adapts the streak renderer to Bubble Tea v1. Hosts send
// typed messages through tea.Program.Send to update the matrix.
package teastreak

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	streak "github.com/dimmkirr/go-streak-chart"
)

// SetStatusMsg updates one cell.
type SetStatusMsg struct {
	Row, Col int
	Status   streak.Status
}

// SetMessageMsg replaces the status line.
type SetMessageMsg struct {
	Text   string
	Status streak.Status
}

// SetRowMessageMsg sets a row's status text (shown in streak.RowLayout).
type SetRowMessageMsg struct {
	Row    int
	Text   string
	Status streak.Status
}

// NoteMsg sets a cell's status and attaches a note; Warning and Error notes
// are listed under the rule.
type NoteMsg struct {
	Row, Col int
	Status   streak.Status
	Text     string
}

// FinishMsg sets the final status line and quits the program.
type FinishMsg struct {
	Text   string
	Status streak.Status
}

type tickMsg time.Time

// SetStatus builds a SetStatusMsg.
func SetStatus(row, col int, s streak.Status) tea.Msg { return SetStatusMsg{row, col, s} }

// SetMessage builds a SetMessageMsg.
func SetMessage(text string, s streak.Status) tea.Msg { return SetMessageMsg{text, s} }

// SetRowMessage builds a SetRowMessageMsg.
func SetRowMessage(row int, text string, s streak.Status) tea.Msg {
	return SetRowMessageMsg{row, text, s}
}

// Warn builds a NoteMsg marking the cell Warning.
func Warn(row, col int, text string) tea.Msg { return NoteMsg{row, col, streak.Warning, text} }

// Fail builds a NoteMsg marking the cell Error.
func Fail(row, col int, text string) tea.Msg { return NoteMsg{row, col, streak.Error, text} }

// Finish builds a FinishMsg.
func Finish(text string, s streak.Status) tea.Msg { return FinishMsg{text, s} }

// Option configures a Model.
type Option func(*Model)

// WithTheme overrides streak.DefaultTheme.
func WithTheme(t streak.Theme) Option { return func(m *Model) { m.theme = t } }

// WithIssues selects where Warning and Error notes are shown (see streak.IssueMode).
func WithIssues(mode streak.IssueMode) Option { return func(m *Model) { m.theme.Issues = mode } }

// WithTick sets the pulse interval. Default 120ms.
func WithTick(d time.Duration) Option { return func(m *Model) { m.tick = d } }

// Model is a tea.Model rendering a streak.Grid.
type Model struct {
	grid  *streak.Grid
	msg   streak.Message
	theme streak.Theme
	tick  time.Duration
	dir   int
}

// New wraps g. The model owns g after this call.
func New(g *streak.Grid, opts ...Option) Model {
	m := Model{grid: g, theme: streak.DefaultTheme(), tick: 120 * time.Millisecond, dir: 1}
	for _, o := range opts {
		o(&m)
	}
	return m
}

// Grid exposes the underlying grid for inspection.
func (m Model) Grid() *streak.Grid { return m.grid }

// Init starts the pulse ticker.
func (m Model) Init() tea.Cmd { return m.tickCmd() }

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.tick, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update applies messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case SetStatusMsg:
		_ = m.grid.Set(v.Row, v.Col, v.Status)
	case SetMessageMsg:
		m.msg = streak.Message{Text: v.Text, Status: v.Status, Level: 1}
		m.dir = 1
	case NoteMsg:
		if err := m.grid.Set(v.Row, v.Col, v.Status); err == nil {
			_ = m.grid.SetNote(v.Row, v.Col, v.Text)
		}
	case SetRowMessageMsg:
		_ = m.grid.SetRowMessage(v.Row, streak.Message{Text: v.Text, Status: v.Status, Level: streak.MaxLevel})
	case FinishMsg:
		m.msg = streak.Message{Text: v.Text, Status: v.Status, Level: streak.MaxLevel}
		return m, tea.Quit
	case tickMsg:
		if m.msg.Status == streak.Running {
			m.msg.Level, m.dir = streak.NextPulse(m.msg.Level, m.dir)
		}
		return m, m.tickCmd()
	case tea.WindowSizeMsg:
		m.theme.Width = v.Width
	case tea.KeyMsg:
		if v.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the frame with a trailing newline.
func (m Model) View() string {
	return streak.Render(m.grid, m.msg, m.theme) + "\n"
}
