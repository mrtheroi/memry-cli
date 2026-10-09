//go:build !windows

package prompt

import "io"

// enableVT reports whether out understands ANSI escape sequences, which
// every Unix terminal does.
var enableVT = func(io.Writer) bool { return true }
