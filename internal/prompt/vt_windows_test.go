//go:build windows

package prompt_test

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/mrtheroi/memry-cli/internal/prompt"
)

// Needs a real console: CI runners have none, so it skips there.
func TestEnableVTTurnsOnVirtualTerminalProcessingOnAConsole(t *testing.T) {
	handle := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		t.Skip("stdout is not a console (CI); the manual checklist covers VT on a real console")
	}
	t.Cleanup(func() { _ = windows.SetConsoleMode(handle, mode) })

	if !prompt.EnableVT(os.Stdout) {
		t.Fatal("EnableVT = false on a console")
	}
	var after uint32
	if err := windows.GetConsoleMode(handle, &after); err != nil || after&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 || after&windows.ENABLE_PROCESSED_OUTPUT == 0 {
		t.Errorf("console mode = %#x, %v; want VT processing and processed output on", after, err)
	}
}
