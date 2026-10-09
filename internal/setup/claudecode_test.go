package setup_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// Tests ported from the Claude Code wiring tests of
// tests/Feature/SetupCommandTest.php: setup with the real agents, the
// claude CLI faked and the Claude Code settings in a temporary directory.

// claudeHarness is a harness wiring the real agents, with
// CLAUDE_CONFIG_DIR in its directory, like the PHP tests' beforeEach.
func claudeHarness(t *testing.T) (*harness, *fakeClaude, string) {
	t.Helper()
	h := newHarness(t)
	claude := h.useRealAgents()
	settingsPath := filepath.Join(h.dir, "claude", "settings.json")
	h.env["CLAUDE_CONFIG_DIR"] = filepath.Dir(settingsPath)
	return h, claude, settingsPath
}

// configuredExecutable is the memry executable the PHP tests configure.
const configuredExecutable = "'/opt/memry/memry'"

// memryCommand is the command that runs subcommand with the configured
// executable and the harness's custom MEMRY_CONFIG.
func (h *harness) memryCommand(subcommand string) string {
	return "MEMRY_CONFIG='" + h.configPath + "' " + configuredExecutable + " " + subcommand
}

// phpJSON encodes value like PHP's json_encode with JSON_UNESCAPED_SLASHES.
func phpJSON(t *testing.T, value any) string {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// mcpServerJSON is the memry MCP server setup registers.
func mcpServerJSON(t *testing.T, url, helper string) string {
	t.Helper()
	return `{"type":"http","url":` + phpJSON(t, url+"/mcp/memory") + `,"headersHelper":` + phpJSON(t, helper) + `}`
}

// memryHookGroup is the SessionStart matcher group setup installs.
func memryHookGroup(command string) map[string]any {
	return map[string]any{
		"matcher": "startup|resume|clear|compact",
		"hooks":   []any{map[string]any{"type": "command", "command": command, "timeout": float64(10)}},
	}
}

func readJSON(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return value
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	writeRaw(t, path, phpJSON(t, value))
}

func writeRaw(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// sessionStart returns the SessionStart groups of the settings at path.
func sessionStart(t *testing.T, path string) any {
	t.Helper()
	return readJSON(t, path).(map[string]any)["hooks"].(map[string]any)["SessionStart"]
}

func assertDeepEqual(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", what, got, want)
	}
}

// addJSON returns the `claude mcp add-json` command setup ran.
func addJSON(t *testing.T, claude *fakeClaude) []string {
	t.Helper()
	for _, argv := range claude.claudeRan() {
		if len(argv) == 7 && argv[2] == "add-json" {
			return argv
		}
	}
	t.Fatalf("ran %q, want claude mcp add-json", claude.ran)
	return nil
}

func failing(output string) agents.RunResult {
	return agents.RunResult{Started: true, ExitCode: 1, Output: output}
}

func TestRegistersTheMemryMCPServerInClaudeCodeWithAHeadersHelper(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	want := []string{"claude", "mcp", "add-json", "--scope", "user", "memry", mcpServerJSON(t, s.URL, h.memryCommand("mcp-headers"))}
	assertDeepEqual(t, "add-json", addJSON(t, claude), want)
}

func TestPointsTheHeadersHelperAtTheRunningMemryBinaryWhenNoExecutableIsConfigured(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	delete(h.env, "MEMRY_CONFIG")
	h.configPath = filepath.Join(h.dir, ".config", "memry", "config.json")
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	want := []string{"claude", "mcp", "add-json", "--scope", "user", "memry", mcpServerJSON(t, s.URL, "'/usr/local/bin/memry' mcp-headers")}
	assertDeepEqual(t, "add-json", addJSON(t, claude), want)
}

func TestLooksUpClaudeRemovesTheLegacyAndMemryEntriesAddsMemryAndVerifiesItInThatOrder(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	var got [][]string
	for _, argv := range claude.ran {
		got = append(got, argv[:min(len(argv), 6)])
	}
	want := [][]string{
		{"lookpath", "claude"},
		{"claude", "mcp", "remove", "--scope", "user", "db-memory"},
		{"claude", "mcp", "remove", "--scope", "user", "memry"},
		{"claude", "mcp", "add-json", "--scope", "user", "memry"},
		{"claude", "mcp", "get", "memry"},
	}
	assertDeepEqual(t, "commands", got, want)
}

func TestFailsWithManualInstructionsButKeepsTheConfigWhenAddingTheMCPServerFails(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	claude.results["add-json"] = failing("Invalid configuration")
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not register the memry MCP server in Claude Code.\n")
	server := mcpServerJSON(t, s.URL, h.memryCommand("mcp-headers"))
	assertContains(t, output, "claude mcp add-json --scope user memry '"+strings.ReplaceAll(server, "'", `'\''`)+"'\n")
	assertDeepEqual(t, "config", h.config(), map[string]any{"url": s.URL, "token": "secret-token", "agents": []any{"claude-code"}})
}

func TestFailsWithManualInstructionsWhenTheMCPServerCannotBeVerified(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	claude.results["get"] = failing("")
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not register the memry MCP server in Claude Code.\n")
	assertContains(t, output, "claude mcp add-json --scope user memry")
	if ran := claude.claudeRan(); len(ran) == 0 || !reflect.DeepEqual(ran[len(ran)-1], []string{"claude", "mcp", "get", "memry"}) {
		t.Errorf("ran %q, want claude mcp get memry last", ran)
	}
}

func TestConfirmsTheMCPServerRegistration(t *testing.T) {
	h, _, _ := claudeHarness(t)
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Registered the memry MCP server in Claude Code (user scope).\n")
}

func TestWarnsWithManualInstructionsAndSkipsRegistrationWhenTheClaudeCodeCLIIsMissing(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	claude.missing = true
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Claude Code CLI not found; skipped MCP registration.\n")
	assertContains(t, output, "claude mcp add-json --scope user memry")
	if ran := claude.claudeRan(); len(ran) != 0 {
		t.Errorf("ran %q, want no claude command", ran)
	}
	if got := h.config()["token"]; got != "secret-token" {
		t.Errorf("token = %v, want secret-token", got)
	}
}

func TestIgnoresFailuresToRemoveTheLegacyAndMemryEntriesWhenTheyDoNotExist(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	claude.results["remove"] = failing("No MCP server found with name: memry")
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Registered the memry MCP server in Claude Code (user scope).\n")
}

func TestNeverPassesTheTokenToClaudeCodeOrPrintsItInTheManualInstructions(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	claude.results["add-json"] = failing("")
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertNotContains(t, output, "secret-token")
	ranAddJSON := false
	for _, argv := range claude.ran {
		ranAddJSON = ranAddJSON || (len(argv) > 2 && argv[2] == "add-json")
		if strings.Contains(strings.Join(argv, " "), "secret-token") {
			t.Errorf("ran %q with the token", argv)
		}
	}
	if !ranAddJSON {
		t.Error("never ran claude mcp add-json")
	}
}

func TestPassesACustomMEMRY_CONFIGOnToTheHeadersHelper(t *testing.T) {
	h, claude, _ := claudeHarness(t)
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	helper := "MEMRY_CONFIG='" + h.configPath + "' '/usr/local/bin/memry' mcp-headers"
	assertDeepEqual(t, "add-json", addJSON(t, claude)[6], mcpServerJSON(t, s.URL, helper))
}

func TestInstallsTheMemrySessionStartHookInANewClaudeCodeSettingsFile(t *testing.T) {
	h, _, settingsPath := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Installed the memry SessionStart hook in "+settingsPath+".\n")
	want := map[string]any{"hooks": map[string]any{"SessionStart": []any{memryHookGroup(h.memryCommand("hook:session-start"))}}}
	assertDeepEqual(t, "settings", readJSON(t, settingsPath), want)
}

func TestKeepsEveryOtherSettingEventAndHookWhenInstallingTheSessionStartHook(t *testing.T) {
	h, _, settingsPath := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	existing := `{"model":"opus","permissions":{"allow":["Bash(ls)"],"deny":[]},"env":{},"hooks":{` +
		`"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo pre"}]}],` +
		`"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"other-tool context"}]}]}}`
	writeRaw(t, settingsPath, existing)
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	var want map[string]any
	if err := json.Unmarshal([]byte(existing), &want); err != nil {
		t.Fatal(err)
	}
	hooks := want["hooks"].(map[string]any)
	hooks["SessionStart"] = append(hooks["SessionStart"].([]any), memryHookGroup(h.memryCommand("hook:session-start")))
	assertDeepEqual(t, "settings", readJSON(t, settingsPath), want)
}

func TestReplacesAPreviouslyInstalledMemrySessionStartHook(t *testing.T) {
	h, _, settingsPath := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	otherTool := map[string]any{"matcher": "startup", "hooks": []any{map[string]any{"type": "command", "command": "other-tool context"}}}
	mixed := map[string]any{"matcher": "resume", "hooks": []any{
		map[string]any{"type": "command", "command": "'/old/memry' hook:session-start"},
		map[string]any{"type": "command", "command": "echo resumed"},
	}}
	writeJSON(t, settingsPath, map[string]any{"hooks": map[string]any{"SessionStart": []any{
		otherTool, memryHookGroup("'/old/memry' hook:session-start"), mixed,
	}}})
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	want := []any{
		otherTool,
		map[string]any{"matcher": "resume", "hooks": []any{map[string]any{"type": "command", "command": "echo resumed"}}},
		memryHookGroup(h.memryCommand("hook:session-start")),
	}
	assertDeepEqual(t, "SessionStart", sessionStart(t, settingsPath), want)
}

func TestLeavesExactlyOneMemrySessionStartHookAfterRunningSetupTwice(t *testing.T) {
	h, _, settingsPath := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	s := newServer(t)

	for range 2 {
		output, code := h.run(emailArgs(s.URL), code123456)
		assertExit(t, code, 0, output)
	}

	assertDeepEqual(t, "SessionStart", sessionStart(t, settingsPath), []any{memryHookGroup(h.memryCommand("hook:session-start"))})
}

func TestFailsWithManualInstructionsAndLeavesAnInvalidSettingsFileUntouched(t *testing.T) {
	h, _, settingsPath := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	writeRaw(t, settingsPath, "{not json")
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not install the memry SessionStart hook: "+settingsPath+" is not valid JSON.\n")
	assertContains(t, output, `"command": `+phpJSON(t, h.memryCommand("hook:session-start")))
	assertContains(t, output, "Registered the memry MCP server in Claude Code (user scope).\n")
	if data, _ := os.ReadFile(settingsPath); string(data) != "{not json" {
		t.Errorf("settings = %q, want them untouched", data)
	}
	if got := h.config()["token"]; got != "secret-token" {
		t.Errorf("token = %v, want secret-token", got)
	}
}

func TestRunsTheHookWithTheRunningMemryBinaryAndACustomMEMRY_CONFIG(t *testing.T) {
	h, _, settingsPath := claudeHarness(t)
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	command := "MEMRY_CONFIG='" + h.configPath + "' '/usr/local/bin/memry' hook:session-start"
	assertDeepEqual(t, "SessionStart", sessionStart(t, settingsPath), []any{memryHookGroup(command)})
}

func TestInstallsTheHookInClaudeSettingsJSONWhenCLAUDE_CONFIG_DIRIsNotSet(t *testing.T) {
	h, _, _ := claudeHarness(t)
	delete(h.env, "CLAUDE_CONFIG_DIR")
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	want := map[string]any{"hooks": map[string]any{"SessionStart": []any{memryHookGroup(h.memryCommand("hook:session-start"))}}}
	assertDeepEqual(t, "settings", readJSON(t, filepath.Join(h.dir, ".claude", "settings.json")), want)
}

func TestKeepsThePermissionsOfAnExistingSettingsFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits; Windows has no 0640 mode")
	}
	h, _, settingsPath := claudeHarness(t)
	writeRaw(t, settingsPath, "{}")
	if err := os.Chmod(settingsPath, 0o640); err != nil {
		t.Fatal(err)
	}
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o640 {
		t.Errorf("mode = %o, want 640", mode)
	}
}

func TestStillInstallsTheHookButFailsWhenTheMCPServerCannotBeRegistered(t *testing.T) {
	h, claude, settingsPath := claudeHarness(t)
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	claude.missing = true
	s := newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Installed the memry SessionStart hook in "+settingsPath+".\n")
	assertDeepEqual(t, "SessionStart", sessionStart(t, settingsPath), []any{memryHookGroup(h.memryCommand("hook:session-start"))})
}

func TestDoesNotTouchTheClaudeCodeSettingsWhenTheLoginFails(t *testing.T) {
	h, claude, settingsPath := claudeHarness(t)
	s := newServer(t, func(s *server) { s.token = &reply{status: 422, body: `{"message":"Invalid or expired code."}`} })

	output, code := h.run(emailArgs(s.URL), answer{label: "Login code", value: "000000"})

	assertExit(t, code, 1, output)
	if _, err := os.Stat(filepath.Dir(settingsPath)); !os.IsNotExist(err) {
		t.Errorf("settings directory exists (%v), want none", err)
	}
	if len(claude.ran) != 0 {
		t.Errorf("ran %q, want nothing", claude.ran)
	}
}

// Unwiring a deselected Claude Code without its CLI skips the MCP server
// with a warning and still removes the hook: not a failure (the PHP CLI
// fails).
func TestUnwiresADeselectedClaudeCodeWithoutItsCLI(t *testing.T) {
	h, claude, settingsPath := claudeHarness(t)
	claude.missing = true
	h.env["MEMRY_EXECUTABLE"] = configuredExecutable
	writeJSON(t, settingsPath, map[string]any{"hooks": map[string]any{"SessionStart": []any{memryHookGroup(h.memryCommand("hook:session-start"))}}})
	s := newServer(t)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"claude-code"}})

	output, code := h.run(selectionArgs(s.URL, "--agents", "codex"), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Claude Code CLI not found; skipped removing the memry MCP server.\n")
	assertContains(t, output, "Claude Code: memry was removed.\n")
	assertDeepEqual(t, "settings", readJSON(t, settingsPath), map[string]any{})
}
