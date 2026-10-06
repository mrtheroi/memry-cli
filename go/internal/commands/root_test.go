package commands_test

import (
	"bytes"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/commands"
)

// The PHP CLI prints "Memry <version>" for --version and its -V shortcut.
func TestVersion(t *testing.T) {
	for _, flag := range []string{"--version", "-V"} {
		var out bytes.Buffer
		root := commands.NewRoot("v0.8.0")
		root.SetOut(&out)
		root.SetArgs([]string{flag})

		if err := root.Execute(); err != nil {
			t.Fatalf("Execute(%s): %v", flag, err)
		}

		if want := "Memry v0.8.0\n"; out.String() != want {
			t.Errorf("%s output = %q, want %q", flag, out.String(), want)
		}
	}
}

// Like the PHP CLI (a Symfony global option), `memry setup --version` and
// `-V` print the version instead of logging in.
func TestSetupVersionPrintsTheVersionInsteadOfLoggingIn(t *testing.T) {
	for _, flag := range []string{"--version", "-V"} {
		var out bytes.Buffer
		root := commands.NewRoot("v0.8.0")
		root.SetOut(&out)
		root.SetArgs([]string{"setup", "--url", "http://127.0.0.1:1", "--email", "ana@example.com", flag})

		if err := root.Execute(); err != nil {
			t.Fatalf("Execute(setup %s): %v (output %q)", flag, err, out.String())
		}

		if want := "Memry v0.8.0\n"; out.String() != want {
			t.Errorf("setup %s output = %q, want %q", flag, out.String(), want)
		}
	}
}

// The description of the PHP CLI (composer.json).
func TestDescription(t *testing.T) {
	want := "CLI for memry: log in from the terminal and wire the memry memory MCP into Claude Code."
	if got := commands.NewRoot("dev").Short; got != want {
		t.Errorf("Short = %q, want %q", got, want)
	}
}
