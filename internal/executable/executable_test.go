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

// On Windows the command is the bare `memry` on the PATH: no quotes, no
// absolute path and no MEMRY_CONFIG prefix (R2.3).
func TestCommandOnWindowsRunsMemryFromThePATH(t *testing.T) {
	exe := executable.Executable{GOOS: "windows", Getenv: env(nil), Self: `C:\Program Files\memry\memry.exe`}

	if got, want := exe.Command("hook:session-start"), "memry hook:session-start"; got != want {
		t.Errorf("Command = %q, want %q", got, want)
	}
}

// S2.3.c: MEMRY_EXECUTABLE replaces `memry` verbatim, whatever quotes it has.
func TestCommandOnWindowsRunsMEMRY_EXECUTABLEVerbatim(t *testing.T) {
	exe := executable.Executable{GOOS: "windows", Getenv: env(map[string]string{"MEMRY_EXECUTABLE": `"C:\Program Files\memry\memry.exe"`})}

	if got, want := exe.Command("hook:session-start"), `"C:\Program Files\memry\memry.exe" hook:session-start`; got != want {
		t.Errorf("Command = %q, want %q", got, want)
	}
}

// S2.3.b: a custom config travels as --config, the path double-quoted with
// forward slashes (one form for Git Bash, PowerShell and CMD).
func TestCommandOnWindowsPassesACustomMEMRY_CONFIGAsAQuotedSlashedFlag(t *testing.T) {
	exe := executable.Executable{GOOS: "windows", Getenv: env(map[string]string{"MEMRY_CONFIG": `C:\Users\ana b\memry\config.json`})}

	if got, want := exe.Command("hook:session-start"), `memry --config "C:/Users/ana b/memry/config.json" hook:session-start`; got != want {
		t.Errorf("Command = %q, want %q", got, want)
	}
}

// A path the hook's shell string cannot carry safely through Git Bash,
// PowerShell and CMD is refused (D6).
func TestHookConfigPathIsRefusedWhenAShellWouldExpandOrBreakIt(t *testing.T) {
	for _, bad := range []string{`C:\a"b\c.json`, `C:\$HOME\c.json`, "C:\\`x`\\c.json", `C:\%APPDATA%\c.json`, `C:\a!b\c.json`, "C:\\a\rb\\c.json", "C:\\a\nb\\c.json", "C:\\a\u201cb\\c.json", "C:\\a\u201db\\c.json", "C:\\a\u201eb\\c.json", "C:\\a\u2018b\\c.json", "C:\\a\u2019b\\c.json"} {
		if executable.SafeInHook(bad) {
			t.Errorf("SafeInHook(%q) = true, want false", bad)
		}
	}
	for _, good := range []string{`C:\Users\ana b\memry\config.json`, `\\server\share\config.json`} {
		if !executable.SafeInHook(good) {
			t.Errorf("SafeInHook(%q) = false, want true", good)
		}
	}
}
