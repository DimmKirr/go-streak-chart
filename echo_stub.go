//go:build !(linux || darwin)

package streak

import "os"

func disableEcho(*os.File) (func(), error) { return func() {}, nil }
