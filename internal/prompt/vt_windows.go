//go:build windows

package prompt

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// enableVT turns on ANSI escape sequence processing for the console out
// writes to, and reports whether it ended up on. A writer that is not a
// console (a pipe, a file) has nothing to turn on, and gets the sequences
// as it would on Unix.
var enableVT = func(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return true
	}
	handle := windows.Handle(file.Fd())
	var mode uint32
	if windows.GetConsoleMode(handle, &mode) != nil {
		return true
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
