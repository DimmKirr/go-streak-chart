package e2e

import (
	"os"
	"testing"
)

// TestMain strips CI from the environment the example inherits. termenv
// (under Lip Gloss) treats any non-empty CI as "not a terminal" and drops to
// the 16-color profile even inside a real PTY with COLORTERM=truecolor, which
// turns the amber warning ramp into plain red and fails every hue check on
// GitHub Actions. The PTY is the truth here, so the hint must not be.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("CI")
	os.Exit(m.Run())
}
