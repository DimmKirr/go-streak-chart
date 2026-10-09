package teastreak

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	streak "github.com/dimmkirr/go-streak-chart"
)

func TestModel_UpdateSetStatus(t *testing.T) {
	m := New(streak.NewGrid([]string{"A"}, 2))
	next, _ := m.Update(SetStatus(0, 1, streak.Done))
	m = next.(Model)
	if s, _ := m.Grid().Get(0, 1); s != streak.Done {
		t.Fatal("status not applied")
	}
}

func TestModel_FinishQuits(t *testing.T) {
	m := New(streak.NewGrid([]string{"A"}, 1))
	next, cmd := m.Update(Finish("ok", streak.Done))
	if cmd == nil {
		t.Fatal("Finish must return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Finish must return tea.Quit")
	}
	if got := streak.StripANSI(next.View()); got == "" || !contains(got, "ok") {
		t.Fatalf("final view must show finish text, got %q", got)
	}
}

func TestModel_CtrlCQuits(t *testing.T) {
	m := New(streak.NewGrid([]string{"A"}, 1))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c must return tea.Quit")
	}
}

func TestModel_ViewMatchesRender(t *testing.T) {
	g := streak.NewGrid([]string{"A"}, 1)
	m := New(g)
	next, _ := m.Update(SetMessage("hi", streak.Running))
	m = next.(Model)
	want := streak.Render(g, streak.Message{Text: "hi", Status: streak.Running, Level: 1}, streak.DefaultTheme()) + "\n"
	if streak.StripANSI(m.View()) != streak.StripANSI(want) {
		t.Fatalf("View must delegate to Render\n got %q\nwant %q", m.View(), want)
	}
}

func TestModel_TickPulsesOnlyWhenRunning(t *testing.T) {
	m := New(streak.NewGrid([]string{"A"}, 1))
	next, _ := m.Update(SetMessage("hi", streak.Running))
	m = next.(Model)
	next, cmd := m.Update(tickMsg{})
	m = next.(Model)
	if m.msg.Level != 2 {
		t.Fatalf("running message must pulse to level 2, got %d", m.msg.Level)
	}
	if cmd == nil {
		t.Fatal("tick must schedule the next tick")
	}
	next, _ = m.Update(SetMessage("warn", streak.Warning))
	m = next.(Model)
	next, _ = m.Update(tickMsg{})
	m = next.(Model)
	if m.msg.Level != 1 {
		t.Fatalf("non-running message must not pulse, got %d", m.msg.Level)
	}
}

func TestModel_InitSchedulesTick(t *testing.T) {
	if New(streak.NewGrid(nil, 0)).Init() == nil {
		t.Fatal("Init must return the tick command")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestModel_UpdateSetRowMessage(t *testing.T) {
	m := New(streak.NewGrid([]string{"A"}, 1))
	next, _ := m.Update(SetRowMessage(0, "row text", streak.Running))
	m = next.(Model)
	if got := m.Grid().RowMessage(0); got.Text != "row text" || got.Status != streak.Running {
		t.Fatalf("got %+v", got)
	}
}

func TestModel_WarnAndFail(t *testing.T) {
	m := New(streak.NewGrid([]string{"A"}, 2))
	next, _ := m.Update(Warn(0, 0, "slow"))
	next, _ = next.(Model).Update(Fail(0, 1, "broken"))
	m = next.(Model)
	issues := m.Grid().Issues()
	if len(issues) != 2 || issues[0].Status != streak.Warning || issues[1].Text != "broken" {
		t.Fatalf("got %+v", issues)
	}
	if !strings.Contains(streak.StripANSI(m.View()), "■ broken") {
		t.Fatalf("view must list issues:\n%s", m.View())
	}
}

func TestModel_WithIssuesNone(t *testing.T) {
	g := streak.NewGrid([]string{"A"}, 1)
	m := New(g, WithIssues(streak.NoIssues))
	mm, _ := m.Update(Fail(0, 0, "boom"))
	if out := streak.StripANSI(mm.View()); strings.Contains(out, "boom") {
		t.Fatalf("issues disabled, got:\n%s", out)
	}
}

func TestModel_WindowSizeTruncatesRows(t *testing.T) {
	g := streak.NewGrid([]string{"A"}, 1)
	th := streak.DefaultTheme()
	th.Layout = streak.RowLayout
	m := New(g, WithTheme(th))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	mm, _ = mm.Update(Fail(0, 0, "a very long note that certainly exceeds twenty columns"))
	for _, line := range strings.Split(streak.StripANSI(mm.View()), "\n") {
		if w := len([]rune(line)); w > 20 {
			t.Fatalf("line wider than the window: %d %q", w, line)
		}
	}
}
