package uninstall_test

import (
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// S1.7.a: a winget install is removed with winget.
func TestWindowsWingetPathsEndWithTheWingetHint(t *testing.T) {
	for _, exe := range []string{
		`C:\Users\Ana\AppData\Local\Microsoft\WinGet\Packages\mrtheroi.memry_x\memry.exe`,
		`C:\Users\Ana\AppData\Local\Microsoft\WinGet\Links\memry.exe`,
		`c:\users\ana\appdata\local\microsoft\winget\links\memry.exe`,
	} {
		h, s := newHarness(t), newServer(t)
		h.goos, h.executable = "windows", exe
		h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

		output, _ := h.run(uninstall.Uninstall, []string{"--force"})

		if want := "Run `winget uninstall memry` to remove the CLI.\n"; !strings.HasSuffix(output, want) {
			t.Errorf("%s: output = %q, want it to end with %q", exe, output, want)
		}
	}
}

// S1.7.b: elsewhere on Windows the file is deleted by hand, and brew is
// never mentioned.
func TestWindowsOtherPathsEndWithTheManualDeleteHint(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.goos, h.executable = "windows", `C:\Users\Ana\go\bin\memry.exe`
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, _ := h.run(uninstall.Uninstall, []string{"--force"})

	if want := "Delete `C:\\Users\\Ana\\go\\bin\\memry.exe` to remove the CLI.\n"; !strings.HasSuffix(output, want) {
		t.Errorf("output = %q, want it to end with %q", output, want)
	}
	if strings.Contains(output, "brew") {
		t.Errorf("output = %q, want no mention of brew", output)
	}
}

// The delete-account command gives the same hint.
func TestWindowsDeleteAccountEndsWithTheWingetHint(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.goos, h.executable = "windows", `C:\Users\Ana\AppData\Local\Microsoft\WinGet\Links\memry.exe`
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, _ := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "ana@example.com"})

	if want := "Run `winget uninstall memry` to remove the CLI.\n"; !strings.HasSuffix(output, want) {
		t.Errorf("output = %q, want it to end with %q", output, want)
	}
}
