package executable_test

import (
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/executable"
)

// Ported from tests/Feature/ExecutableTest.php. Where the PHP CLI runs its
// PHAR (or PHP and the memry script), the Go CLI runs its own binary.

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestCommandRunsTheExecutableGivenInMEMRY_EXECUTABLE(t *testing.T) {
	exe := executable.Executable{
		Getenv: env(map[string]string{"MEMRY_EXECUTABLE": "/opt/homebrew/opt/memry/bin/memry"}),
		Self:   "/usr/local/Cellar/memry/1.0.0/bin/memry",
	}

	if got, want := exe.Command("mcp-headers"), "/opt/homebrew/opt/memry/bin/memry mcp-headers"; got != want {
		t.Errorf("Command = %q, want %q", got, want)
	}
}

func TestCommandPrefixesACustomMEMRY_CONFIGToTheConfiguredExecutable(t *testing.T) {
	exe := executable.Executable{Getenv: env(map[string]string{
		"MEMRY_CONFIG":     "/tmp/memry config.json",
		"MEMRY_EXECUTABLE": "'/opt/memry/memry'",
	})}

	if got, want := exe.Command("mcp-headers"), "MEMRY_CONFIG='/tmp/memry config.json' '/opt/memry/memry' mcp-headers"; got != want {
		t.Errorf("Command = %q, want %q", got, want)
	}
}

// The PHP CLI runs escapeshellarg(Phar::running(false)) when it has no
// MEMRY_EXECUTABLE; the Go CLI quotes its own binary the same way.
func TestCommandRunsTheQuotedRunningBinaryWhenNoExecutableIsConfigured(t *testing.T) {
	exe := executable.Executable{Getenv: env(nil), Self: "/Users/ana's mac/bin/memry"}

	if got, want := exe.Command("hook:session-start"), `'/Users/ana'\''s mac/bin/memry' hook:session-start`; got != want {
		t.Errorf("Command = %q, want %q", got, want)
	}
}

func TestArgumentsSplitTheCommandIntoTheExecutableGivenInMEMRY_EXECUTABLEAndItsArguments(t *testing.T) {
	exe := executable.Executable{
		Getenv: env(map[string]string{"MEMRY_EXECUTABLE": "/opt/homebrew/opt/memry/bin/memry"}),
		Self:   "/usr/local/Cellar/memry/1.0.0/bin/memry",
	}

	if got, want := exe.Arguments("mcp"), []string{"/opt/homebrew/opt/memry/bin/memry", "mcp"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Arguments = %q, want %q", got, want)
	}
}

// Where the PHP CLI splits it into the PHP binary and the memry script.
func TestArgumentsSplitTheCommandIntoTheRunningBinaryWhenNoExecutableIsConfigured(t *testing.T) {
	exe := executable.Executable{Getenv: env(nil), Self: "/usr/local/bin/memry"}

	if got, want := exe.Arguments("mcp"), []string{"/usr/local/bin/memry", "mcp"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Arguments = %q, want %q", got, want)
	}
}

func TestEnvironmentIsWhatACustomMEMRY_CONFIGNeedsAndNoneByDefault(t *testing.T) {
	vars := map[string]string{}
	exe := executable.Executable{Getenv: env(vars)}

	if got := exe.Environment(); len(got) != 0 {
		t.Errorf("Environment = %q, want none", got)
	}

	vars["MEMRY_CONFIG"] = "/tmp/memry config.json"

	if got, want := exe.Environment(), map[string]string{"MEMRY_CONFIG": "/tmp/memry config.json"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Environment = %q, want %q", got, want)
	}
}
