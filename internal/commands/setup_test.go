package commands_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/commands"
)

func execute(args ...string) (string, error) {
	var out bytes.Buffer
	root := commands.NewRoot("dev")
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// setup reads its own options (flags.Scan owns argv), prints only its own
// lines and fails with exit code 1.
func TestSetupRunsWithItsOwnOptions(t *testing.T) {
	output, err := execute("setup", "--url=ftp://memry.test", "--email", "ana@example.com")

	if err == nil {
		t.Error("Execute() = nil, want an error for exit code 1")
	}
	if want := "Invalid server address given with --url. Use an http:// or https:// URL.\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

// `memry setup --help` (or -h) prints the usage, with the options, instead
// of running setup.
func TestSetupHelpPrintsTheUsage(t *testing.T) {
	for _, args := range [][]string{{"setup", "--help"}, {"setup", "--url=ftp://memry.test", "-h"}} {
		output, err := execute(args...)

		if err != nil {
			t.Errorf("Execute(%q) = %v, want nil", args, err)
		}
		for _, want := range []string{
			"Log in to memry with an email one-time code or a token",
			"memry setup [options]\n",
			"--url", "--email", "--token", "--agents", "--no-interaction",
		} {
			if !strings.Contains(output, want) {
				t.Errorf("Execute(%q) output does not contain %q:\n%s", args, want, output)
			}
		}
		if strings.Contains(output, "Invalid server address") {
			t.Errorf("Execute(%q) ran setup:\n%s", args, output)
		}
	}
}
