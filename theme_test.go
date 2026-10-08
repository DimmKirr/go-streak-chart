package streak

import "testing"

func TestDefaultTheme_RampsComplete(t *testing.T) {
	th := DefaultTheme()
	for _, s := range []Status{Pending, Running, Done, Warning, Error} {
		ramp, ok := th.Ramp[s]
		if !ok {
			t.Fatalf("missing ramp for %v", s)
		}
		for i, c := range ramp {
			if c == nil {
				t.Errorf("%v level %d nil color", s, i)
			}
		}
	}
	if th.Glyph == "" || th.MessageGlyph == "" || th.Gap == "" || th.RuleChar == "" {
		t.Fatal("glyph, message glyph, gap and rule char must have defaults")
	}
}

func TestTheme_CellColor_IndexesAndClamps(t *testing.T) {
	th := DefaultTheme()
	if th.CellColor(Done, MaxLevel) != th.Ramp[Done][MaxLevel] {
		t.Fatal("CellColor must index the ramp")
	}
	if th.CellColor(Done, 9) != th.Ramp[Done][MaxLevel] {
		t.Fatal("must clamp high")
	}
	if th.CellColor(Status(42), 2) != th.Ramp[Pending][2] {
		t.Fatal("unknown status must fall back to Pending ramp")
	}
}

func TestCellLevel(t *testing.T) {
	if CellLevel(Pending) != 0 {
		t.Fatal("pending cells are level 0")
	}
	for _, s := range []Status{Running, Done, Warning, Error} {
		if CellLevel(s) != MaxLevel {
			t.Fatalf("%v must be MaxLevel", s)
		}
	}
}

func TestDefaultTheme_GlyphsAreDistinct(t *testing.T) {
	th := DefaultTheme()
	if th.Glyph != "▄" {
		t.Fatalf("matrix glyph must be a single lower half block: a square tile with equal gaps on both axes, got %q", th.Glyph)
	}
	if th.MessageGlyph != "▄" {
		t.Fatalf("message glyph must match the tile weight, got %q", th.MessageGlyph)
	}
}

func TestDefaultTheme_LayoutIsFooter(t *testing.T) {
	if DefaultTheme().Layout != FooterLayout {
		t.Fatal("default layout must be FooterLayout")
	}
}

func TestLayout_String(t *testing.T) {
	for l, want := range map[Layout]string{FooterLayout: "footer", RowLayout: "row", Layout(7): "Layout(7)"} {
		if got := l.String(); got != want {
			t.Errorf("%d: got %q want %q", int(l), got, want)
		}
	}
}
