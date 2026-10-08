package streak

import "time"

// clock abstracts time so tests can drive ticks deterministically.
type clock interface {
	// Ticker returns a channel that delivers ticks and a stop function.
	Ticker(d time.Duration) (<-chan time.Time, func())
}

type realClock struct{}

func (realClock) Ticker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// tickAcker is implemented by test clocks that want to know when the
// loader has finished processing a tick.
type tickAcker interface{ tickDone() }
