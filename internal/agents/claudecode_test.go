package agents_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/executable"
)

// Ported from the Claude Code assertions of tests/Feature/SetupCommandTest.php
// and tests/Feature/UninstallCommandTest.php. The PHP CLI looks claude up
// with `command -v claude`; the Go CLI looks it up on the PATH.

// claudeEnv is a test environment with claude on the PATH, a custom
// MEMRY_CONFIG and the '/opt/memry/memry' executable, like the PHP tests.
func claudeEnv(t *testing.T) *testEnv {
	e := newTestEnv(t)
	e.onPath = []string{"claude"}
	e.vars["MEMRY_CONFIG"] = e.path("memry", "config.json")
	e.vars["MEMRY_EXECUTABLE"] = "'/opt/memry/memry'"
	e.vars["CLAUDE_CONFIG_DIR"] = e.path("claude")
	return e
}

// memryCommand is the command setup writes for the configured executable,
// prefixed with the custom MEMRY_CONFIG.
func (e *testEnv) memryCommand(subcommand string) string {
	return "MEMRY_CONFIG='" + e.vars["MEMRY_CONFIG"] + "' '/opt/memry/memry' " + subcommand
}

// claudeServer is the JSON add-json gets for the memry server at memry.test.
func (e *testEnv) claudeServer(helper string) string {
	return `{"type":"http","url":"https://memry.test/mcp/memory","headersHelper":"` + helper + `"}`
}

func TestClaudeCodeRegistersTheMemryMCPServerWithAHeadersHelper(t *testing.T) {
	e := claudeEnv(t)

	e.agent(t, "claude-code").Install("https://memry.test")

	want := []string{"claude", "mcp", "add-json", "--scope", "user", "memry", e.claudeServer(e.memryCommand("mcp-headers"))}
	if !hasRun(e, want) {
		t.Errorf("ran %q, want it to include %q", e.runner.ran, want)
	}
}

func hasRun(e *testEnv, argv []string) bool {
	for _, ran := range e.runner.ran {
		if reflect.DeepEqual(ran, argv) {
			return true
		}
	}
	return false
}

// Where the PHP CLI points it at its PHAR (or PHP and the memry script).
func TestClaudeCodePointsTheHeadersHelperAtTheRunningBinaryWhenNoExecutableIsConfigured(t *testing.T) {
	e := claudeEnv(t)
	delete(e.vars, "MEMRY_CONFIG")
	delete(e.vars, "MEMRY_EXECUTABLE")

	e.agent(t, "claude-code").Install("https://memry.test")

	want := []string{"claude", "mcp", "add-json", "--scope", "user", "memry", e.claudeServer("'/usr/local/bin/memry' mcp-headers")}
	if !hasRun(e, want) {
		t.Errorf("ran %q, want it to include %q", e.runner.ran, want)
	}
}

func TestClaudeCodeLooksUpClaudeRemovesTheLegacyAndExistingEntriesAddsMemryAndVerifiesItInThatOrder(t *testing.T) {
	e := claudeEnv(t)

	e.agent(t, "claude-code").Install("https://memry.test")

	want := [][]string{
		{"lookpath", "claude"},
		{"claude", "mcp", "remove", "--scope", "user", "db-memory"},
		{"claude", "mcp", "remove", "--scope", "user", "memry"},
		{"claude", "mcp", "add-json", "--scope", "user", "memry", e.claudeServer(e.memryCommand("mcp-headers"))},
		{"claude", "mcp", "get", "memry"},
	}
	if !reflect.DeepEqual(e.runner.ran, want) {
		t.Errorf("ran %q, want %q", e.runner.ran, want)
	}
}

func TestClaudeCodeFailsWithManualInstructionsWhenAddingTheMCPServerFails(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"add-json": false}

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertHasLine(t, result, "error", "Could not register the memry MCP server in Claude Code.")
	assertHasLine(t, result, "line", "Your login was saved. Register the server manually with:")
	assertHasLine(t, result, "line", "  claude mcp add-json --scope user memry "+executable.EscapeShellArg(e.claudeServer(e.memryCommand("mcp-headers"))))
}

func TestClaudeCodeFailsWithManualInstructionsWhenTheMCPServerCannotBeVerified(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"get": false}

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertHasLine(t, result, "error", "Could not register the memry MCP server in Claude Code.")
	if !hasRun(e, []string{"claude", "mcp", "get", "memry"}) {
		t.Errorf("ran %q, want claude mcp get memry", e.runner.ran)
	}
}

func TestClaudeCodeConfirmsTheMCPServerRegistration(t *testing.T) {
	e := claudeEnv(t)

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertHasLine(t, result, "info", "Registered the memry MCP server in Claude Code (user scope).")
}

func TestClaudeCodeWarnsWithManualInstructionsAndSkipsRegistrationWhenTheCLIIsMissing(t *testing.T) {
	e := claudeEnv(t)
	e.onPath = nil

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertHasLine(t, result, "warn", "Claude Code CLI not found; skipped MCP registration.")
	assertHasLine(t, result, "line", "  claude mcp add-json --scope user memry "+executable.EscapeShellArg(e.claudeServer(e.memryCommand("mcp-headers"))))
	for _, ran := range e.runner.ran {
		if ran[0] == "claude" {
			t.Errorf("ran %q without the claude CLI", ran)
		}
	}
}

// The PHP test fakes the remove with exit code 1 and "No MCP server found",
// the output the hybrid rule takes for a server that is not registered.
func TestClaudeCodeIgnoresFailuresToRemoveTheLegacyAndMemryEntriesWhenTheyDoNotExist(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"remove": notFound}

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertHasLine(t, result, "info", "Registered the memry MCP server in Claude Code (user scope).")
}

func TestClaudeCodePassesACustomMEMRY_CONFIGOnToTheHeadersHelperOfTheRunningBinary(t *testing.T) {
	e := claudeEnv(t)
	delete(e.vars, "MEMRY_EXECUTABLE")

	e.agent(t, "claude-code").Install("https://memry.test")

	helper := "MEMRY_CONFIG='" + e.vars["MEMRY_CONFIG"] + "' '/usr/local/bin/memry' mcp-headers"
	if want := []string{"claude", "mcp", "add-json", "--scope", "user", "memry", e.claudeServer(helper)}; !hasRun(e, want) {
		t.Errorf("ran %q, want it to include %q", e.runner.ran, want)
	}
}

// hookGroup is the SessionStart group setup installs, as the PHP CLI
// writes it inside settings.json, for command.
func hookGroup(command string) string {
	return `{
                "matcher": "startup|resume|clear|compact",
                "hooks": [
                    {
                        "type": "command",
                        "command": "` + command + `",
                        "timeout": 10
                    }
                ]
            }`
}

// hookSettings is settings.json holding only the SessionStart groups.
func hookSettings(groups string) string {
	return "{\n    \"hooks\": {\n        \"SessionStart\": [\n            " + groups + "\n        ]\n    }\n}\n"
}

func TestClaudeCodeInstallsTheMemrySessionStartHookInANewSettingsFile(t *testing.T) {
	e := claudeEnv(t)
	settings := e.path("claude", "settings.json")

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertHasLine(t, result, "info", "Installed the memry SessionStart hook in "+settings+".")
	assertContents(t, settings, hookSettings(hookGroup(e.memryCommand("hook:session-start"))))
}

func TestClaudeCodeFailsWithManualInstructionsAndLeavesAnInvalidSettingsFileUntouched(t *testing.T) {
	e := claudeEnv(t)
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, "{not json")

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertContents(t, settings, "{not json")
	assertHasLine(t, result, "error", "Could not install the memry SessionStart hook: "+settings+" is not valid JSON.")
	assertHasLine(t, result, "line", `Fix the file, then add this group to the "hooks.SessionStart" array by hand:`)
	assertHasLine(t, result, "line", `{
    "matcher": "startup|resume|clear|compact",
    "hooks": [
        {
            "type": "command",
            "command": "`+e.memryCommand("hook:session-start")+`",
            "timeout": 10
        }
    ]
}`)
	assertHasLine(t, result, "info", "Registered the memry MCP server in Claude Code (user scope).")
}

func TestClaudeCodeKeepsEveryOtherSettingEventAndHookWhenInstallingTheHook(t *testing.T) {
	e := claudeEnv(t)
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, `{"model":"opus","permissions":{"allow":["Bash(ls)"],"deny":[]},"env":{},"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo pre"}]}],"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"other-tool context"}]}]}}`)

	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, settings, `{
    "model": "opus",
    "permissions": {
        "allow": [
            "Bash(ls)"
        ],
        "deny": []
    },
    "env": {},
    "hooks": {
        "PreToolUse": [
            {
                "matcher": "Bash",
                "hooks": [
                    {
                        "type": "command",
                        "command": "echo pre"
                    }
                ]
            }
        ],
        "SessionStart": [
            {
                "matcher": "startup",
                "hooks": [
                    {
                        "type": "command",
                        "command": "other-tool context"
                    }
                ]
            },
            `+hookGroup(e.memryCommand("hook:session-start"))+`
        ]
    }
}
`)
}

func TestClaudeCodeReplacesAPreviouslyInstalledMemrySessionStartHook(t *testing.T) {
	e := claudeEnv(t)
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"'/old/memry' hook:session-start","timeout":10}]}]}}`)

	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, settings, hookSettings(hookGroup(e.memryCommand("hook:session-start"))))
}

func TestClaudeCodeLeavesExactlyOneMemrySessionStartHookAfterInstallingTwice(t *testing.T) {
	e := claudeEnv(t)

	e.agent(t, "claude-code").Install("https://memry.test")
	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, e.path("claude", "settings.json"), hookSettings(hookGroup(e.memryCommand("hook:session-start"))))
}

func TestClaudeCodeRunsTheHookWithTheRunningBinaryAndACustomMEMRY_CONFIG(t *testing.T) {
	e := claudeEnv(t)
	delete(e.vars, "MEMRY_EXECUTABLE")

	e.agent(t, "claude-code").Install("https://memry.test")

	command := "MEMRY_CONFIG='" + e.vars["MEMRY_CONFIG"] + "' '/usr/local/bin/memry' hook:session-start"
	assertContents(t, e.path("claude", "settings.json"), hookSettings(hookGroup(command)))
}

func TestClaudeCodeInstallsTheHookInClaudeSettingsJSONWhenCLAUDE_CONFIG_DIRIsNotSet(t *testing.T) {
	e := claudeEnv(t)
	delete(e.vars, "CLAUDE_CONFIG_DIR")

	e.agent(t, "claude-code").Install("https://memry.test")

	assertContents(t, e.path(".claude", "settings.json"), hookSettings(hookGroup(e.memryCommand("hook:session-start"))))
}

func TestClaudeCodeKeepsThePermissionsOfAnExistingSettingsFile(t *testing.T) {
	e := claudeEnv(t)
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, "{}")
	if err := os.Chmod(settings, 0o640); err != nil {
		t.Fatal(err)
	}

	e.agent(t, "claude-code").Install("https://memry.test")

	if info, _ := os.Stat(settings); info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %o, want 0640", info.Mode().Perm())
	}
}

func TestClaudeCodeStillInstallsTheHookButFailsWhenTheMCPServerCannotBeRegistered(t *testing.T) {
	e := claudeEnv(t)
	e.onPath = nil
	settings := e.path("claude", "settings.json")

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertHasLine(t, result, "info", "Installed the memry SessionStart hook in "+settings+".")
	assertContents(t, settings, hookSettings(hookGroup(e.memryCommand("hook:session-start"))))
}

func TestClaudeCodeRemovesTheMemryAndLegacyDbMemoryMCPServersOnUninstall(t *testing.T) {
	e := claudeEnv(t)

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, true)
	assertHasLine(t, result, "info", "Removed the memry MCP server from Claude Code.")
	for _, want := range [][]string{
		{"claude", "mcp", "remove", "--scope", "user", "memry"},
		{"claude", "mcp", "remove", "--scope", "user", "db-memory"},
	} {
		if !hasRun(e, want) {
			t.Errorf("ran %q, want it to include %q", e.runner.ran, want)
		}
	}
}

func TestClaudeCodeIgnoresMCPServersThatAreNotRegisteredOnUninstall(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"remove": notFound}

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, true)
	assertHasLine(t, result, "line", "The memry MCP server was not registered in Claude Code.")
}

func TestClaudeCodeRemovesOnlyTheMemrySessionStartHookFromTheSettings(t *testing.T) {
	e := claudeEnv(t)
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, `{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"'/opt/memry/memry' hook:session-start","timeout":10}]},{"matcher":"startup","hooks":[{"type":"command","command":"other-tool"}]}]}}`)

	result := e.agent(t, "claude-code").Uninstall()

	assertHasLine(t, result, "info", "Removed the memry SessionStart hook from "+settings+".")
	assertContents(t, settings, hookSettings(`{
                "matcher": "startup",
                "hooks": [
                    {
                        "type": "command",
                        "command": "other-tool"
                    }
                ]
            }`))
}

func TestClaudeCodeWarnsFailsAndLeavesMalformedSettingsUntouchedOnUninstall(t *testing.T) {
	e := claudeEnv(t)
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, `{"hooks": `)

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, false)
	assertHasLine(t, result, "warn", "Could not remove the memry SessionStart hook: "+settings+" is not valid JSON.")
	assertContents(t, settings, `{"hooks": `)
}

func TestClaudeCodeReportsWhenThereIsNoMemrySessionStartHookToRemove(t *testing.T) {
	e := claudeEnv(t)

	result := e.agent(t, "claude-code").Uninstall()

	assertHasLine(t, result, "line", "No memry SessionStart hook to remove.")
	if exists(t, e.path("claude", "settings.json")) {
		t.Error("the settings were created")
	}
}

// Without its CLI, Claude Code cannot be running memry's MCP server: the
// removal is skipped with a warning, not a failure, and the hook is still
// removed. (The PHP CLI fails the uninstall.)
func TestClaudeCodeWarnsAndKeepsGoingWhenTheCLIIsNotFoundOnUninstall(t *testing.T) {
	e := claudeEnv(t)
	e.onPath = nil
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, `{"hooks":{"SessionStart":[`+memryGroup+`]}}`)

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, true)
	assertLines(t, result,
		[2]string{"warn", "Claude Code CLI not found; skipped removing the memry MCP server."},
		[2]string{"info", "Removed the memry SessionStart hook from " + settings + "."},
	)
	for _, ran := range e.runner.ran {
		if ran[0] == "claude" {
			t.Errorf("ran %q without the claude CLI", ran)
		}
	}
}

// Without its CLI, the uninstall fails only when the hook cannot be
// removed.
func TestClaudeCodeFailsWithoutTheCLIOnlyWhenTheHookCannotBeRemoved(t *testing.T) {
	e := claudeEnv(t)
	e.onPath = nil
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, `{"hooks": `)

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, false)
	assertHasLine(t, result, "warn", "Could not remove the memry SessionStart hook: "+settings+" is not valid JSON.")
}

const memryGroup = `{"matcher":"startup","hooks":[{"type":"command","command":"memry hook:session-start","timeout":10}]}`

// The PHP CLI runs `command -v claude`; the Go CLI looks claude up on the PATH.
func TestClaudeCodeIsInstalledWhenTheClaudeCLIIsOnThePATH(t *testing.T) {
	e := claudeEnv(t)
	if !e.agent(t, "claude-code").IsInstalled() {
		t.Error("IsInstalled = false with claude on the PATH")
	}

	e.onPath = nil
	mkdir(t, e.path(".claude"))
	if e.agent(t, "claude-code").IsInstalled() {
		t.Error("IsInstalled = true without claude on the PATH")
	}
}

// Not in the PHP CLI, which throws when it cannot read or write the
// settings: the hook step fails on its own, saying why.
func TestClaudeCodeFailsTheHookStepWhenItCannotReadTheSettings(t *testing.T) {
	e := claudeEnv(t)
	writeFile(t, e.path("claude"), "a file, not a directory")
	settings := e.path("claude", "settings.json")

	install := e.agent(t, "claude-code").Install("https://memry.test")
	uninstall := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, install, false)
	assertHasLine(t, install, "error", "Could not install the memry SessionStart hook: open "+settings+": not a directory.")
	assertSuccessful(t, uninstall, false)
	assertHasLine(t, uninstall, "warn", "Could not remove the memry SessionStart hook: open "+settings+": not a directory.")
}

// Codex review of PR #34: a server JSON that cannot be encoded (an
// executable path or MEMRY_CONFIG that is not valid UTF-8) must fail
// before any lookup or claude command, so the existing registration is
// never removed, and never print an add-json line with an empty payload.
func TestClaudeCodeFailsWithoutRunningAnythingWhenTheServerCannotBeEncoded(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"executable": {"MEMRY_EXECUTABLE": "/opt/\xffmemry"},
		"config":     {"MEMRY_CONFIG": "/tmp/\xffconfig.json"},
	} {
		t.Run(name, func(t *testing.T) {
			e := claudeEnv(t)
			for k, v := range vars {
				e.vars[k] = v
			}

			result := e.agent(t, "claude-code").Install("https://memry.test")

			assertSuccessful(t, result, false)
			if len(e.runner.ran) != 0 {
				t.Errorf("ran %q, want no lookup and no command", e.runner.ran)
			}
			assertHasLine(t, result, "error", "Could not register the memry MCP server in Claude Code: the memry executable path or MEMRY_CONFIG is not valid UTF-8.")
			assertHasLine(t, result, "line", "Set them to valid UTF-8 paths, then run `memry setup` again.")
			for _, line := range result.Lines {
				if strings.Contains(line.Text, "add-json") {
					t.Errorf("printed %q, want no manual add-json line", line.Text)
				}
			}
		})
	}
}

// The hook's manual instructions never hold an empty group either.
func TestClaudeCodePrintsNoEmptyHookGroupWhenTheCommandCannotBeEncoded(t *testing.T) {
	e := claudeEnv(t)
	e.vars["MEMRY_EXECUTABLE"] = "/opt/\xffmemry"
	settings := e.path("claude", "settings.json")
	writeFile(t, settings, "{not json")

	result := e.agent(t, "claude-code").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertContents(t, settings, "{not json")
	assertLines(t, result,
		[2]string{"error", "Could not register the memry MCP server in Claude Code: the memry executable path or MEMRY_CONFIG is not valid UTF-8."},
		[2]string{"line", "Set them to valid UTF-8 paths, then run `memry setup` again."},
		[2]string{"error", "Could not install the memry SessionStart hook: " + settings + " is not valid JSON."},
		[2]string{"line", "Set them to valid UTF-8 paths, then run `memry setup` again."},
	)
}

var (
	notStarted = agents.RunResult{ExitCode: -1}
	timedOut   = agents.RunResult{Started: true, ExitCode: -1, TimedOut: true}
	notFound   = agents.RunResult{Started: true, ExitCode: 1, Output: "No MCP server found"}
)

// Codex review of PR #34, with the user's hybrid rule for `claude mcp
// remove`: exit 0 means removed; a non-zero exit whose output contains
// "No MCP server found" (any case) means not registered, as in PHP; any
// other outcome is a failure. PHP takes every failure for "not registered".
func TestClaudeCodeFailsTheUninstallWhenRemovingMemryCouldNotStartOrTimedOut(t *testing.T) {
	for name, tt := range map[string]struct {
		result agents.RunResult
		why    string
	}{
		"could not start": {notStarted, "`claude mcp remove` could not start."},
		"timed out":       {timedOut, "`claude mcp remove` timed out."},
	} {
		t.Run(name, func(t *testing.T) {
			e := claudeEnv(t)
			e.runner.results = map[string]any{"remove memry": tt.result}

			result := e.agent(t, "claude-code").Uninstall()

			assertSuccessful(t, result, false)
			assertLines(t, result,
				[2]string{"warn", "Could not remove the memry MCP server from Claude Code: " + tt.why},
				[2]string{"line", "No memry SessionStart hook to remove."},
			)
		})
	}
}

func TestClaudeCodeTakesARemoveThatExitedNonZeroWithNoMCPServerFoundForANotRegisteredServer(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"remove": notFound}

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, true)
	assertHasLine(t, result, "line", "The memry MCP server was not registered in Claude Code.")
}

func TestClaudeCodeTakesARemoveThatExitedWith0ForARemovedServer(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"remove": agents.RunResult{Started: true}}

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, true)
	assertHasLine(t, result, "info", "Removed the memry MCP server from Claude Code.")
}

// The same rule for the legacy db-memory server, which does not stop the
// memry one from being removed.
func TestClaudeCodeFailsTheUninstallWhenRemovingTheLegacyServerCouldNotStartOrTimedOut(t *testing.T) {
	for name, tt := range map[string]struct {
		result agents.RunResult
		why    string
	}{
		"could not start": {notStarted, "`claude mcp remove` could not start."},
		"timed out":       {timedOut, "`claude mcp remove` timed out."},
	} {
		t.Run(name, func(t *testing.T) {
			e := claudeEnv(t)
			e.runner.results = map[string]any{"remove db-memory": tt.result}

			result := e.agent(t, "claude-code").Uninstall()

			assertSuccessful(t, result, false)
			assertLines(t, result,
				[2]string{"warn", "Could not remove the db-memory MCP server from Claude Code: " + tt.why},
				[2]string{"info", "Removed the memry MCP server from Claude Code."},
				[2]string{"line", "No memry SessionStart hook to remove."},
			)
		})
	}
}

// The same rule for the removes setup runs before adding memry: if one
// could not start or timed out, memry is not added over a registration it
// could not clear.
func TestClaudeCodeFailsTheRegistrationWhenARemoveBeforeItCouldNotStartOrTimedOut(t *testing.T) {
	for name, tt := range map[string]struct {
		server string
		result agents.RunResult
		why    string
	}{
		"legacy could not start": {"db-memory", notStarted, "`claude mcp remove` could not start."},
		"memry timed out":        {"memry", timedOut, "`claude mcp remove` timed out."},
	} {
		t.Run(name, func(t *testing.T) {
			e := claudeEnv(t)
			e.runner.results = map[string]any{"remove " + tt.server: tt.result}

			result := e.agent(t, "claude-code").Install("https://memry.test")

			assertSuccessful(t, result, false)
			assertHasLine(t, result, "error", "Could not register the memry MCP server in Claude Code: "+tt.why)
			assertHasLine(t, result, "line", "  claude mcp add-json --scope user memry "+executable.EscapeShellArg(e.claudeServer(e.memryCommand("mcp-headers"))))
			for _, ran := range e.runner.ran {
				if len(ran) > 2 && (ran[2] == "add-json" || ran[2] == "get") {
					t.Errorf("ran %q after a failed remove", ran)
				}
			}
		})
	}
}

// permissionDenied is a remove that ran and failed for another reason
// than a missing server.
var permissionDenied = agents.RunResult{Started: true, ExitCode: 1, Output: "Error: EACCES: permission denied, open '/x/.claude.json'\n"}

const permissionDeniedWhy = "`claude mcp remove` exited with 1 (Error: EACCES: permission denied, open '/x/.claude.json')."

func TestClaudeCodeFailsTheUninstallWhenARemoveExitedNonZeroWithoutNoMCPServerFound(t *testing.T) {
	for name, tt := range map[string]struct {
		results map[string]any
		want    [][2]string
	}{
		"memry, with output": {
			map[string]any{"remove memry": permissionDenied},
			[][2]string{{"warn", "Could not remove the memry MCP server from Claude Code: " + permissionDeniedWhy}},
		},
		"memry, without output": {
			map[string]any{"remove memry": false},
			[][2]string{{"warn", "Could not remove the memry MCP server from Claude Code: `claude mcp remove` exited with 1."}},
		},
		"legacy": {
			map[string]any{"remove db-memory": permissionDenied},
			[][2]string{
				{"warn", "Could not remove the db-memory MCP server from Claude Code: " + permissionDeniedWhy},
				{"info", "Removed the memry MCP server from Claude Code."},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := claudeEnv(t)
			e.runner.results = tt.results

			result := e.agent(t, "claude-code").Uninstall()

			assertSuccessful(t, result, false)
			assertLines(t, result, append(tt.want, [2]string{"line", "No memry SessionStart hook to remove."})...)
		})
	}
}

func TestClaudeCodeFailsTheRegistrationWhenARemoveBeforeItExitedNonZeroWithoutNoMCPServerFound(t *testing.T) {
	for _, server := range []string{"db-memory", "memry"} {
		t.Run(server, func(t *testing.T) {
			e := claudeEnv(t)
			e.runner.results = map[string]any{"remove " + server: permissionDenied}

			result := e.agent(t, "claude-code").Install("https://memry.test")

			assertSuccessful(t, result, false)
			assertHasLine(t, result, "error", "Could not register the memry MCP server in Claude Code: "+permissionDeniedWhy)
			for _, ran := range e.runner.ran {
				if len(ran) > 2 && (ran[2] == "add-json" || ran[2] == "get") {
					t.Errorf("ran %q after a failed remove", ran)
				}
			}
		})
	}
}

func TestClaudeCodeMatchesNoMCPServerFoundInAnyCaseWithinTheOutput(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"remove": agents.RunResult{Started: true, ExitCode: 1, Output: "Error: no mcp server FOUND with name: memry\n"}}

	result := e.agent(t, "claude-code").Uninstall()

	assertSuccessful(t, result, true)
	assertHasLine(t, result, "line", "The memry MCP server was not registered in Claude Code.")
}

func TestClaudeCodeShowsAShortExcerptOfTheOutputOfAFailedRemove(t *testing.T) {
	e := claudeEnv(t)
	e.runner.results = map[string]any{"remove memry": agents.RunResult{Started: true, ExitCode: 2, Output: "first line\n\tsecond " + strings.Repeat("é", 300)}}

	result := e.agent(t, "claude-code").Uninstall()

	assertHasLine(t, result, "warn", "Could not remove the memry MCP server from Claude Code: `claude mcp remove` exited with 2 (first line second "+strings.Repeat("é", 182)+"…).")
}
