package streak

import "testing"

func TestStatus_String(t *testing.T) {
	cases := map[Status]string{
		Pending: "pending", Running: "running", Done: "done",
		Warning: "warning", Error: "error", Status(99): "Status(99)",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("%d: got %q want %q", int(s), got, want)
		}
	}
}

func TestStatus_Severity_Order(t *testing.T) {
	order := []Status{Done, Pending, Running, Warning, Error}
	for i := 1; i < len(order); i++ {
		if order[i].Severity() <= order[i-1].Severity() {
			t.Fatalf("%v must be more severe than %v", order[i], order[i-1])
		}
	}
}

func TestLevel_Clamp(t *testing.T) {
	for in, want := range map[Level]Level{-3: 0, 0: 0, 2: 2, 4: 4, 9: 4} {
		if got := in.Clamp(); got != want {
			t.Errorf("%d: got %d want %d", in, got, want)
		}
	}
}

func TestNextPulse_Bounces(t *testing.T) {
	l, dir := Level(1), 1
	var seen []Level
	for i := 0; i < 7; i++ {
		l, dir = NextPulse(l, dir)
		seen = append(seen, l)
	}
	want := []Level{2, 3, 4, 3, 2, 1, 2}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("got %v want %v", seen, want)
		}
	}
}
