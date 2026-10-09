package agents_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// S2.2.d: a .cmd runs as `cmd.exe /d /s /c ""script" "arg"..."`, every
// argument quoted so a path with spaces stays one argument.
func TestCmdLine_QuotesSpaces(t *testing.T) {
	got, err := agents.CmdLine(`C:\Program Files\claude\claude.cmd`, []string{"mcp", "add", `C:\My Dir\memry.exe`})
	if err != nil {
		t.Fatal(err)
	}
	want := `cmd.exe /d /s /c ""C:\Program Files\claude\claude.cmd" "mcp" "add" "C:\My Dir\memry.exe""`
	if got != want {
		t.Errorf("CmdLine = %s, want %s", got, want)
	}
}

// A trailing backslash would escape the closing quote for the program that
// parses the command line, so it is doubled.
func TestCmdLine_DoublesTrailingBackslash(t *testing.T) {
	got, err := agents.CmdLine(`C:\x\claude.cmd`, []string{`C:\dir\`, `a\\`, `a\b`, ``})
	if err != nil {
		t.Fatal(err)
	}
	want := `cmd.exe /d /s /c ""C:\x\claude.cmd" "C:\dir\\" "a\\\\" "a\b" """`
	if got != want {
		t.Errorf("CmdLine = %s, want %s", got, want)
	}
}

// Reject set: cmd.exe expands %VAR% and !VAR! even inside double quotes and
// a quote ends the quoted word, so these make the run refuse to start.
func TestCmdLine_Rejects(t *testing.T) {
	for name, arg := range map[string]string{
		"double quote": `a"b`,
		"percent":      `%PATH%`,
		"bang":         `!x!`,
		"CR":           "a\rb",
		"LF":           "a\nb",
		"NUL":          "a\x00b",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := agents.CmdLine(`C:\x\claude.cmd`, []string{"ok", arg}); err == nil {
				t.Errorf("CmdLine accepted an argument with %s: %s", name, got)
			}
			if got, err := agents.CmdLine(`C:\x\a`+arg+`.cmd`, nil); err == nil {
				t.Errorf("CmdLine accepted a script with %s: %s", name, got)
			}
		})
	}
}

// Pin (already green): inside double quotes cmd.exe treats these as plain
// text, so they are not rejected.
func TestCmdLine_AllowsMetacharactersInsideQuotes(t *testing.T) {
	arg := "^&|<>() ;,='"
	got, err := agents.CmdLine(`C:\x\claude.cmd`, []string{arg})
	if err != nil {
		t.Fatal(err)
	}
	if want := `cmd.exe /d /s /c ""C:\x\claude.cmd" "` + arg + `""`; got != want {
		t.Errorf("CmdLine = %s, want %s", got, want)
	}
}
