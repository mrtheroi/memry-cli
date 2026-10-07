package uninstall_test

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// Tests ported from tests/Feature/UninstallCommandTest.php.

func TestRevokesTheTokenOnTheServerItBelongsTo(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Revoked the memry token.\n")
	got := s.received()
	if len(got) != 1 || got[0].Method != http.MethodDelete || got[0].Path != "/api/auth/token" ||
		got[0].Header.Get("Authorization") != "Bearer old-token" || got[0].Header.Get("Accept") != "application/json" {
		t.Errorf("requests = %+v, want one DELETE /api/auth/token with Bearer old-token", got)
	}
}

func TestWarnsAndFailsWhenTheTokenCannotBeRevoked(t *testing.T) {
	for name, revoke := range map[string]*reply{"server error": {status: 500, body: "Server Error"}, "unreachable": nil} {
		t.Run(name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t, func(s *server) { s.revoke = revoke })
			h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

			output, code := h.run(uninstall.Uninstall, force)

			assertExit(t, code, 1, output)
			assertContains(t, output, "Could not revoke the memry token.\n")
		})
	}
}

func TestDoesNotFollowARedirectFromTheRevokeToAPageThatAnswers200(t *testing.T) {
	h := newHarness(t)
	s := newServer(t, func(s *server) { s.revoke = &reply{status: 302} })
	s.revoke.location = s.URL + "/login"
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not revoke the memry token.\n")
	for _, r := range s.received() {
		if r.Path == "/login" {
			t.Error("followed the redirect to /login")
		}
	}
}

func TestCountsATokenTheServerNoLongerAcceptsAsAlreadyRevoked(t *testing.T) {
	h, s := newHarness(t), newServer(t, func(s *server) { s.revoke = &reply{status: 401, body: `{"message":"Unauthenticated."}`} })
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "The memry token was already revoked.\n")
	assertNotContains(t, output, "Could not revoke")
}

func TestSkipsTheRevokeWhenNotLoggedIn(t *testing.T) {
	for name, config := range map[string]map[string]any{"no config file": nil, "config without a token": {"url": "https://memry.test"}} {
		t.Run(name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			if config != nil {
				h.previousConfig(config)
			}

			output, code := h.run(uninstall.Uninstall, force)

			assertExit(t, code, 0, output)
			assertContains(t, output, "Not logged in; no token to revoke.\n")
			if got := s.received(); len(got) != 0 {
				t.Errorf("sent %+v, want nothing", got)
			}
		})
	}
}

func TestRemovesTheMemryAndLegacyDbMemoryMCPServersFromClaudeCode(t *testing.T) {
	h := newHarness(t)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Removed the memry MCP server from Claude Code.\n")
	assertRan(t, h.claude, []string{"claude", "mcp", "remove", "--scope", "user", "memry"})
	assertRan(t, h.claude, []string{"claude", "mcp", "remove", "--scope", "user", "db-memory"})
}

func TestIgnoresMCPServersThatAreNotRegistered(t *testing.T) {
	h := newHarness(t)
	h.claude.results["remove"] = agents.RunResult{Started: true, ExitCode: 1, Output: "No MCP server found with name: memry"}

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "The memry MCP server was not registered in Claude Code.\n")
	assertNotContains(t, output, "Removed the memry MCP server")
}

func TestRemovesOnlyTheMemrySessionStartHookFromTheClaudeCodeSettings(t *testing.T) {
	h := newHarness(t)
	writeRaw(t, h.settingsPath, `{"hooks":{"SessionStart":[`+
		`{"matcher":"startup","hooks":[{"type":"command","command":"'/opt/memry/memry' hook:session-start","timeout":10}]},`+
		`{"matcher":"startup","hooks":[{"type":"command","command":"other-tool"}]}]}}`)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Removed the memry SessionStart hook from "+h.settingsPath+".\n")
	var got any
	if err := json.Unmarshal([]byte(readText(t, h.settingsPath)), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"hooks": map[string]any{"SessionStart": []any{
		map[string]any{"matcher": "startup", "hooks": []any{map[string]any{"type": "command", "command": "other-tool"}}},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("settings = %v, want %v", got, want)
	}
}

func TestWarnsFailsAndLeavesMalformedClaudeCodeSettingsUntouched(t *testing.T) {
	h := newHarness(t)
	writeRaw(t, h.settingsPath, `{"hooks": `)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not remove the memry SessionStart hook: "+h.settingsPath+" is not valid JSON.\n")
	if got := readText(t, h.settingsPath); got != `{"hooks": ` {
		t.Errorf("settings = %q, want them untouched", got)
	}
}

func TestReportsWhenThereIsNoMemrySessionStartHookToRemove(t *testing.T) {
	h := newHarness(t)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "No memry SessionStart hook to remove.\n")
	if _, err := os.Stat(h.settingsPath); !os.IsNotExist(err) {
		t.Errorf("settings exist (%v), want none", err)
	}
}

func TestDeletesTheConfigFile(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Deleted "+h.configPath+".\n")
	if h.configExists() {
		t.Error("the config file still exists")
	}
}

func TestWarnsAndFailsWhenTheConfigFileCannotBeDeleted(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})
	if err := os.Chmod(h.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(h.dir, 0o700) })

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not delete "+h.configPath+".\n")
	if !h.configExists() {
		t.Error("the config file was deleted")
	}
}

func TestWarnsKeepsGoingAndFailsWhenTheClaudeCodeCLIIsNotFound(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.claude.missing = true
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Claude Code CLI not found; skipped removing the memry MCP server.\n")
	// Unlike the PHP CLI, which deletes the config anyway, the login and
	// the agent are kept for a retry (see retry_test.go).
	assertContains(t, output, "No memry SessionStart hook to remove.\n")
	assertContains(t, output, retryHint)
	if len(h.claude.ran) != 0 {
		t.Errorf("ran %q, want no claude command", h.claude.ran)
	}
}

func TestRemovesMemryFromTheAgentsSavedBySetup(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := &fakeAgent{key: "claude-code", name: "Claude Code"}, &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(claude, codex)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"codex"}})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Unwired Codex.\n")
	assertCalls(t, codex, "uninstall")
	assertCalls(t, claude)
}

// Setup saved no agents up to 0.4.0, when it only wired Claude Code.
func TestRemovesMemryFromClaudeCodeWhenSetupSavedNoAgents(t *testing.T) {
	for name, config := range map[string]map[string]any{"config without agents": {"token": "old-token"}, "no config file": nil} {
		t.Run(name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			claude, codex := &fakeAgent{key: "claude-code", name: "Claude Code"}, &fakeAgent{key: "codex", name: "Codex"}
			h.useAgents(claude, codex)
			if config != nil {
				config["url"] = s.URL
				h.previousConfig(config)
			}

			output, code := h.run(uninstall.Uninstall, force)

			assertExit(t, code, 0, output)
			assertCalls(t, claude, "uninstall")
			assertCalls(t, codex)
		})
	}
}

const confirmation = "Remove memry from your agents and delete your login?"

func TestRemovesNothingWhenTheConfirmationIsDeclined(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.Uninstall, nil, answer{label: confirmation, value: "no"})

	assertExit(t, code, 1, output)
	if output != "Aborted; nothing was removed.\n" {
		t.Errorf("output = %q, want only the abort", output)
	}
	if len(s.received()) != 0 || len(h.claude.ran) != 0 {
		t.Errorf("sent %+v and ran %q, want nothing", s.received(), h.claude.ran)
	}
	if !h.configExists() {
		t.Error("the config file was deleted")
	}
}

func TestRemovesEverythingAfterTheConfirmationAndEndsWithTheHomebrewHint(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.Uninstall, nil, answer{label: confirmation, value: "yes"})

	assertExit(t, code, 0, output)
	if want := "Run `brew uninstall memry` to remove the CLI.\n"; !strings.HasSuffix(output, want) {
		t.Errorf("output = %q, want it to end with %q", output, want)
	}
	if h.configExists() {
		t.Error("the config file still exists")
	}
}

// Like Symfony, a confirmation without interaction takes its default, no.
func TestRemovesNothingWithoutInteractionUnlessForced(t *testing.T) {
	for _, option := range []string{"-n", "--no-interaction", "-q"} {
		h, s := newHarness(t), newServer(t)
		h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

		output, code := h.run(uninstall.Uninstall, []string{option})

		assertExit(t, code, 1, output)
		if !h.configExists() || len(s.received()) != 0 {
			t.Errorf("%s: removed something", option)
		}
	}
}

// Like Symfony, the input ending before an answer aborts.
func TestAbortsWhenTheInputEndsBeforeTheConfirmation(t *testing.T) {
	h := newHarness(t)

	output, code := h.run(uninstall.Uninstall, nil, answer{label: confirmation, aborted: true})

	assertExit(t, code, 1, output)
	if want := "\n            \n  Aborted.  \n            \n\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

func TestRejectsArgumentsLikeSymfony(t *testing.T) {
	tests := map[string]string{
		"--force=yes": `The "--force" option does not accept a value.`,
		"-f":          `The "-f" option does not exist.`,
		"--bogus":     `The "--bogus" option does not exist.`,
		"extra":       `No arguments expected for "uninstall" command, got "extra".`,
	}
	for arg, message := range tests {
		h := newHarness(t)

		output, code := h.run(uninstall.Uninstall, []string{"--force", arg})

		assertExit(t, code, 1, output)
		blank := strings.Repeat(" ", len(message)+4)
		if want := "\n" + blank + "\n  " + message + "  \n" + blank + "\n\n"; output != want {
			t.Errorf("%s: output = %q, want %q", arg, output, want)
		}
	}
}

func TestPrintsNothingWhenQuiet(t *testing.T) {
	h := newHarness(t)

	output, code := h.run(uninstall.Uninstall, []string{"--force", "-q"})

	assertExit(t, code, 0, output)
	if output != "" {
		t.Errorf("output = %q, want none", output)
	}
}
