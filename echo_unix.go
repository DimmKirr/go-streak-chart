//go:build linux || darwin

package streak

import (
	"os"

	"golang.org/x/sys/unix"
)

// disableEcho clears the terminal's ECHO flags so that keypresses during
// a redrawing Loader do not inject stray newlines that shift the cursor
// and corrupt the frame. The returned function restores the original state.
func disableEcho(f *os.File) (restore func(), err error) {
	fd := int(f.Fd())
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	saved := *termios
	termios.Lflag &^= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, termios); err != nil {
		return nil, err
	}
	return func() {
		_ = unix.IoctlSetTermios(fd, ioctlWriteTermios, &saved)
	}, nil
}
