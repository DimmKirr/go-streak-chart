package streak

import "fmt"

// Status is the state of one component cell.
type Status int

// Cell states, ordered by lifecycle.
const (
	Pending Status = iota
	Running
	Done
	Warning
	Error
)

func (s Status) String() string {
	switch s {
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Done:
		return "done"
	case Warning:
		return "warning"
	case Error:
		return "error"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// Severity orders statuses for summaries; higher is worse.
func (s Status) Severity() int {
	switch s {
	case Error:
		return 4
	case Warning:
		return 3
	case Running:
		return 2
	case Pending:
		return 1
	default:
		return 0
	}
}

// Level is a GitHub-style intensity from 0 (none) to MaxLevel (most saturated).
type Level int

// MaxLevel is the most saturated intensity, matching GitHub's five-step ramp.
const MaxLevel Level = 4

// Clamp bounds the level to [0, MaxLevel].
func (l Level) Clamp() Level {
	if l < 0 {
		return 0
	}
	if l > MaxLevel {
		return MaxLevel
	}
	return l
}

// NextPulse advances a pulsing level one step, bouncing between 1 and
// MaxLevel. dir is +1 or -1 and is returned flipped at the bounds.
func NextPulse(l Level, dir int) (Level, int) {
	if dir == 0 {
		dir = 1
	}
	n := l + Level(dir)
	if n > MaxLevel {
		return MaxLevel - 1, -1
	}
	if n < 1 {
		return 2, 1
	}
	return n, dir
}
