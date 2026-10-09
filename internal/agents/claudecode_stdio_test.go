package agents_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// The Claude Code tests for Windows run on every OS: the mode depends only
// on the GOOS seam, and the runner is fake.

const windowsExe = `C:\Program Files\memry\memry.exe`

// windowsClaudeEnv is a Windows environment with claude on the PATH, the
// running binary at windowsExe and no custom config or MEMRY_EXECUTABLE.
func windowsClaudeEnv(t *testing.T) *testEnv {
	e := claudeEnv(t)
	e.goos = "windows"
	e.self = windowsExe
	delete(e.vars, "MEMRY_CONFIG")
	delete(e.vars, "MEMRY_EXECUTABLE")
	return e
}

// S2.1.a: remove the legacy and existing entries, add memry over stdio
// with everything after -- verbatim, then verify it; as argv elements.
func TestClaudeCodeOnWindowsRegistersMemryOverStdioWithTheExactArgvSequence(t *testing.T) {
	e := windowsClaudeEnv(t)

	result := e.agent(t, "claude-code").Install("https://memry.test")

	want := [][]string{
		{"lookpath", "claude"},
		{"claude", "mcp", "remove", "--scope", "user", "db-memory"},
		{"claude", "mcp", "remove", "--scope", "user", "memry"},
		{"claude", "mcp", "add", "--transport", "stdio", "--scope", "user", "memry", "--", windowsExe, "mcp"},
		{"claude", "mcp", "get", "memry"},
	}
	if !reflect.DeepEqual(e.runner.ran, want) {
		t.Errorf("ran %q, want %q", e.runner.ran, want)
	}
	assertSuccessful(t, result, true)
}

func hasAnyRunWith(e *testEnv, arg string) bool {
	for _, ran := range e.runner.ran {
		if slices.Contains(ran, arg) {
			return true
		}
	}
	return false
}

func indexOfRun(e *testEnv, argv []string) int {
	for i, ran := range e.runner.ran {
		if reflect.DeepEqual(ran, argv) {
			return i
		}
	}
	return -1
}

// S2.1.b: a custom config has no env pair; it travels as --config argv
// after the executable.
func TestClaudeCodeOnWindowsPassesACustomConfigAsAnArgvFlag(t *testing.T) {
	e := windowsClaudeEnv(t)
	config := `C:\Users\ana b\memry\config.json`
	e.vars["MEMRY_CONFIG"] = config

	e.agent(t, "claude-code").Install("https://memry.test")

	want := []string{"claude", "mcp", "add", "--transport", "stdio", "--scope", "user", "memry", "--", windowsExe, "--config", config, "mcp"}
	if !hasRun(e, want) {
		t.Errorf("ran %q, want it to include %q", e.runner.ran, want)
	}
}

// S2.1.c: MEMRY_EXECUTABLE is the executable as it is, with no quoting.
func TestClaudeCodeOnWindowsUsesMEMRY_EXECUTABLEVerbatim(t *testing.T) {
	e := windowsClaudeEnv(t)
	e.vars["MEMRY_EXECUTABLE"] = `D:\my tools\memry.exe`

	e.agent(t, "claude-code").Install("https://memry.test")

	want := []string{"claude", "mcp", "add", "--transport", "stdio", "--scope", "user", "memry", "--", `D:\my tools\memry.exe`, "mcp"}
	if !hasRun(e, want) {
		t.Errorf("ran %q, want it to include %q", e.runner.ran, want)
	}
}

// S2.1.d: running setup twice adds twice, each after removing the entry.
func TestClaudeCodeOnWindowsRunTwiceAddsTwiceEachAfterARemove(t *testing.T) {
	e := windowsClaudeEnv(t)

	e.agent(t, "claude-code").Install("https://memry.test")
	e.agent(t, "claude-code").Install("https://memry.test")

	var commands []string
	for _, ran := range e.runner.ran {
		if ran[0] == "claude" {
			commands = append(commands, ran[2]+" "+ran[len(ran)-1])
		}
	}
	want := []string{"remove db-memory", "remove memry", "add mcp", "get memry", "remove db-memory", "remove memry", "add mcp", "get memry"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("commands = %q, want %q", commands, want)
	}
}

// S2.1.e: an existing HTTP registration is removed before the add, and
// add-json is never used.
func TestClaudeCodeOnWindowsRemovesAnExistingHTTPRegistrationBeforeAddingAndNeverUsesAddJSON(t *testing.T) {
	e := windowsClaudeEnv(t)

	e.agent(t, "claude-code").Install("https://memry.test")

	if hasAnyRunWith(e, "add-json") {
		t.Errorf("ran %q, want no add-json", e.runner.ran)
	}
	remove := indexOfRun(e, []string{"claude", "mcp", "remove", "--scope", "user", "memry"})
	add := indexOfRun(e, []string{"claude", "mcp", "add", "--transport", "stdio", "--scope", "user", "memry", "--", windowsExe, "mcp"})
	if remove < 0 || add < remove {
		t.Errorf("remove at %d, add at %d: want the remove first", remove, add)
	}
}

// A remove that cannot run leaves a registration memry must not add over.
func TestClaudeCodeOnWindowsAbortsBeforeTheAddWhenARemoveFails(t *testing.T) {
	e := windowsClaudeEnv(t)
	e.runner.results = map[string]any{"remove memry": agents.RunResult{Started: true, ExitCode: 1, Output: "access denied"}}

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	if hasAnyRunWith(e, "add") {
		t.Errorf("ran %q, want no add", e.runner.ran)
	}
	assertHasLine(t, result, "line", `  claude mcp add --transport stdio --scope user memry -- "`+windowsExe+`" "mcp"`)
}

// S2.1.f: a failing add or get gives the stdio manual command, quoted for
// the user's shell.
func TestClaudeCodeOnWindowsShowsTheStdioManualCommandWhenAddOrGetFails(t *testing.T) {
	for _, failing := range []string{"add", "get"} {
		e := windowsClaudeEnv(t)
		e.vars["MEMRY_CONFIG"] = `C:\cfg dir\config.json`
		e.runner.results = map[string]any{failing: false}

		result := e.agent(t, "claude-code").Install("https://memry.test")

		assertSuccessful(t, result, false)
		assertHasLine(t, result, "error", "Could not register the memry MCP server in Claude Code.")
		assertHasLine(t, result, "line", "Your login was saved. Register the server manually with:")
		assertHasLine(t, result, "line", `  claude mcp add --transport stdio --scope user memry -- "`+windowsExe+`" "--config" "C:\cfg dir\config.json" "mcp"`)
	}
}

// S2.1.h: the token stays in the config file; the runner takes only argv.
func TestClaudeCodeOnWindowsNeverPutsTheTokenInAnyCommand(t *testing.T) {
	e := windowsClaudeEnv(t)
	config := e.path("memry", "config.json")
	e.vars["MEMRY_CONFIG"] = config
	writeFile(t, config, `{"url":"https://memry.test","token":"s3cret-token"}`)

	e.agent(t, "claude-code").Install("https://memry.test")

	for _, ran := range e.runner.ran {
		for _, arg := range ran {
			if strings.Contains(arg, "s3cret-token") {
				t.Errorf("argv %q has the token", ran)
			}
		}
	}
}

// S2.1.g: uninstall on Windows removes the server, the legacy one and the hook.
func TestClaudeCodeOnWindowsUninstallRemovesTheServersAndTheHook(t *testing.T) {
	e := windowsClaudeEnv(t)
	settings := e.path("claude", "settings.json")
	e.agent(t, "claude-code").Install("https://memry.test")
	e.runner.ran = nil

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, true)
	want := [][]string{
		{"lookpath", "claude"},
		{"claude", "mcp", "remove", "--scope", "user", "db-memory"},
		{"claude", "mcp", "remove", "--scope", "user", "memry"},
	}
	if !reflect.DeepEqual(e.runner.ran, want) {
		t.Errorf("ran %q, want %q", e.runner.ran, want)
	}
	assertContents(t, settings, "{}\n")
}

// S2.3.a to S2.3.c are pins: the Windows hook command comes from
// executable.Command, tested there; these check what lands in settings.json.
func TestClaudeCodeOnWindowsInstallsThePlainHookCommand(t *testing.T) {
	e := windowsClaudeEnv(t)

	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, e.path("claude", "settings.json"), hookSettings(hookGroup("memry hook:session-start")))
}

func TestClaudeCodeOnWindowsInstallsTheConfigFormOfTheHookCommand(t *testing.T) {
	e := windowsClaudeEnv(t)
	e.vars["MEMRY_CONFIG"] = `C:\Users\ana b\memry\config.json`

	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, e.path("claude", "settings.json"), hookSettings(hookGroup(`memry --config "C:/Users/ana b/memry/config.json" hook:session-start`)))
}

func TestClaudeCodeOnWindowsInstallsMEMRY_EXECUTABLEVerbatimInTheHook(t *testing.T) {
	e := windowsClaudeEnv(t)
	e.vars["MEMRY_EXECUTABLE"] = `"D:\my tools\memry.exe"`

	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, e.path("claude", "settings.json"), hookSettings(hookGroup(`"D:\my tools\memry.exe" hook:session-start`)))
}

func TestClaudeCodeOnWindowsLeavesOneHookGroupAfterInstallingTwice(t *testing.T) {
	e := windowsClaudeEnv(t)

	e.agent(t, "claude-code").Install("https://memry.test")
	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, e.path("claude", "settings.json"), hookSettings(hookGroup("memry hook:session-start")))
}

func TestClaudeCodeOnWindowsReplacesAUnixFormHook(t *testing.T) {
	e := windowsClaudeEnv(t)
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"MEMRY_CONFIG='/x' '/old/memry' hook:session-start","timeout":10}]}]}}`)

	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, settings, hookSettings(hookGroup("memry hook:session-start")))
}

// D6: a config path a shell would expand or that breaks the quoting makes
// the hook install fail without writing the hook; the MCP registration
// does not depend on it.
func TestClaudeCodeOnWindowsFailsTheHookForAConfigPathItCannotQuote(t *testing.T) {
	e := windowsClaudeEnv(t)
	e.vars["MEMRY_CONFIG"] = `C:\Users\50%\config.json`

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertHasLine(t, result, "error", `Could not install the memry SessionStart hook: the config path C:\Users\50%\config.json has characters a hook command cannot carry (one of " $ `+"`"+` % ! or a line break).`)
	if exists(t, e.path("claude", "settings.json")) {
		t.Error("settings.json written, want the hook left out")
	}
}
