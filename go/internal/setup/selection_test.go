package setup_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/prompt"
)

// Tests ported from tests/Feature/SetupAgentSelectionTest.php.

const whichAgents = "Which agents do you use?"

// useAgents makes setup use the given fake agents, in that order.
func (h *harness) useAgents(fakes ...*fakeAgent) {
	list := make([]agents.Agent, len(fakes))
	for i, fake := range fakes {
		list[i] = fake
	}
	h.agents = agents.NewSupported(list...)
}

func newClaude() *fakeAgent { return &fakeAgent{key: "claude-code", name: "Claude Code"} }
func newCodex() *fakeAgent  { return &fakeAgent{key: "codex", name: "Codex"} }

func assertCalls(t *testing.T, agent *fakeAgent, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(agent.calls, want) && (len(want) > 0 || len(agent.calls) > 0) {
		t.Errorf("%s calls = %q, want %q", agent.key, agent.calls, want)
	}
}

func assertSavedAgents(t *testing.T, h *harness, want ...string) {
	t.Helper()
	wantAny := []any{}
	for _, key := range want {
		wantAny = append(wantAny, key)
	}
	if got := h.config()["agents"]; !reflect.DeepEqual(got, wantAny) {
		t.Errorf("saved agents = %#v, want %#v", got, wantAny)
	}
}

func selectionArgs(url string, extra ...string) []string {
	return append([]string{"--url", url, "--email", "ana@example.com"}, extra...)
}

func TestFailsBeforeLoggingInWhenAgentsHasAnUnknownKey(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	h.useAgents(claude, codex)

	output, code := h.run(selectionArgs(s.URL, "--agents", "claude-code,cursor"))

	assertExit(t, code, 1, output)
	assertContains(t, output, `Unknown agent "cursor". Valid agents: claude-code, codex.`)
	if got := s.received(); len(got) != 0 {
		t.Errorf("sent %+v, want nothing", got)
	}
	assertCalls(t, claude)
	assertCalls(t, codex)
	h.assertNoConfig()
}

func TestWiresOnlyTheAgentsGivenWithAgentsAndSavesThem(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	h.useAgents(claude, codex)

	output, code := h.run(selectionArgs(s.URL, "--agents", "codex"), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Wired Codex.\n")
	assertCalls(t, codex, "install "+s.URL)
	assertCalls(t, claude)
	assertDeepEqual(t, "config", h.config(), map[string]any{"url": s.URL, "token": "secret-token", "agents": []any{"codex"}})
}

func TestAsksWhichAgentsToWireLabellingTheOnesNotInstalled(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	codex.notInstalled = true
	h.useAgents(claude, codex)

	output, code := h.run(selectionArgs(s.URL), code123456, answer{
		label:    whichAgents,
		choices:  []prompt.Choice{{Value: "claude-code", Label: "Claude Code"}, {Value: "codex", Label: "Codex (not installed)"}},
		defaults: []string{"claude-code"},
		selected: []string{"codex"},
	})

	assertExit(t, code, 0, output)
	assertCalls(t, codex, "install "+s.URL)
	assertCalls(t, claude)
	assertSavedAgents(t, h, "codex")
}

// The PHP tests of a run without interaction log in with --email and
// still answer the login code, which only Pest's mocked questions allow:
// the real command asks nothing with -n. These ports log in with a token,
// which asks nothing either.
func TestWiresTheInstalledAgentsWithoutAskingWhenNotInteractive(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	codex.notInstalled = true
	h.useAgents(claude, codex)

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "--no-interaction"})

	assertExit(t, code, 0, output)
	assertCalls(t, claude, "install "+s.URL)
	assertCalls(t, codex)
	assertSavedAgents(t, h, "claude-code")
}

func TestWiresThePreviouslySavedAgentsWithoutAskingWhenNotInteractive(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	codex.notInstalled = true
	h.useAgents(claude, codex)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"codex"}})

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "--no-interaction"})

	assertExit(t, code, 0, output)
	assertCalls(t, codex, "install "+s.URL)
	assertCalls(t, claude)
	assertSavedAgents(t, h, "codex")
}

func TestDropsSavedAgentsThisVersionDoesNotSupport(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(newClaude(), newCodex())
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"cursor", "codex"}})

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "--no-interaction"})

	assertExit(t, code, 0, output)
	assertSavedAgents(t, h, "codex")
}

func TestRemovesMemryFromPreviouslySavedAgentsThatAreNoLongerSelected(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	h.useAgents(claude, codex)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"claude-code", "codex"}})

	output, code := h.run(selectionArgs(s.URL, "--agents", "codex"), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Unwired Claude Code.\n")
	assertCalls(t, claude, "uninstall")
	assertCalls(t, codex, "install "+s.URL)
}

func TestKeepsWiringTheOtherAgentsAndFailsWhenOneAgentFails(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	claude.fails = true
	h.useAgents(claude, codex)

	output, code := h.run(selectionArgs(s.URL, "--agents", "claude-code,codex"), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not wire Claude Code.\n")
	assertContains(t, output, "Wired Codex.\n")
	assertCalls(t, claude, "install "+s.URL)
	assertCalls(t, codex, "install "+s.URL)
}

func TestFailsWhenMemryCannotBeRemovedFromADeselectedAgent(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	claude.fails = true
	h.useAgents(claude, codex)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"claude-code"}})

	output, code := h.run(selectionArgs(s.URL, "--agents", "codex"), code123456)

	assertExit(t, code, 1, output)
	assertCalls(t, claude, "uninstall")
}

func TestWarnsButKeepsTheLoginWhenNoAgentIsSelected(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	h.useAgents(claude, codex)

	output, code := h.run(selectionArgs(s.URL), code123456, answer{
		label:    whichAgents,
		choices:  []prompt.Choice{{Value: "claude-code", Label: "Claude Code"}, {Value: "codex", Label: "Codex"}},
		selected: []string{},
	})

	assertExit(t, code, 0, output)
	assertContains(t, output, "No agents selected; memry is not wired into any agent. Run `memry setup` again to choose some.\n")
	assertCalls(t, claude)
	assertCalls(t, codex)
	assertDeepEqual(t, "config", h.config(), map[string]any{"url": s.URL, "token": "secret-token", "agents": []any{}})
}

func TestEndsWithASummaryLinePerAgent(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude := newClaude()
	claude.fails = true
	h.useAgents(claude, newCodex(), &fakeAgent{key: "gemini", name: "Gemini CLI"})
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"gemini"}})

	output, code := h.run(selectionArgs(s.URL, "--agents", "claude-code,codex"), code123456)

	assertExit(t, code, 1, output)
	summary := "\nClaude Code: failed; see the messages above.\nCodex: memry is set up.\nGemini CLI: memry was removed.\n"
	if !strings.HasSuffix(output, summary) {
		t.Errorf("output = %q, want it to end with %q", output, summary)
	}
}

func TestWiresExactlyTheAgentsGivenWithAgentsIntoTheirFiles(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useRealAgents()
	h.env["MEMRY_EXECUTABLE"] = "/opt/homebrew/opt/memry/bin/memry"
	home := h.dir

	output, code := h.run(selectionArgs(s.URL, "--agents", "codex,windsurf"), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Codex: memry is set up.\n")
	assertContains(t, output, "Windsurf: memry is set up.\n")
	toml := readText(t, filepath.Join(home, ".codex", "config.toml"))
	assertContains(t, toml, "[mcp_servers.memry]")
	assertContains(t, readText(t, filepath.Join(home, ".codex", "AGENTS.md")), "<!-- memry:start -->")
	windsurf := readJSON(t, filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")).(map[string]any)
	args := windsurf["mcpServers"].(map[string]any)["memry"].(map[string]any)["args"]
	assertDeepEqual(t, "windsurf args", args, []any{"mcp"})
	assertContains(t, readText(t, filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md")), "<!-- memry:start -->")
	for _, dir := range []string{".config/opencode", ".gemini", ".claude"} {
		if _, err := os.Stat(filepath.Join(home, dir)); !os.IsNotExist(err) {
			t.Errorf("%s exists (%v), want none", dir, err)
		}
	}
	assertNotContains(t, toml+readText(t, filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")), "secret-token")
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A deselected agent memry could not be removed from stays saved, after
// the new selection, so a later setup or uninstall retries the removal.
// (The PHP CLI forgets it.)
func TestKeepsADeselectedAgentSavedUntilMemryIsRemovedFromIt(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex, windsurf := newClaude(), newCodex(), &fakeAgent{key: "windsurf", name: "Windsurf"}
	claude.fails = true
	h.useAgents(claude, codex, windsurf)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"windsurf", "claude-code"}})

	output, code := h.run(selectionArgs(s.URL, "--agents", "codex"), code123456)

	assertExit(t, code, 1, output)
	assertSavedAgents(t, h, "claude-code", "codex")
}

// Each agent is selected once, at its first place. (The PHP CLI saves
// duplicates as given.)
func TestSavesEachSelectedAgentOnce(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(newClaude(), newCodex())

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "--agents=codex,codex,claude-code"})

	assertExit(t, code, 0, output)
	assertSavedAgents(t, h, "codex", "claude-code")

	h.writeConfig(map[string]any{"url": s.URL, "token": "admin-token", "agents": []string{"codex", "cursor", "codex"}})
	output, code = h.run([]string{"--url", s.URL, "--token=admin-token", "-n"})

	assertExit(t, code, 0, output)
	assertSavedAgents(t, h, "codex")
}

func TestAsksWithEachSavedAgentOnceAsTheDefault(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(newClaude(), newCodex())
	h.writeConfig(map[string]any{"url": s.URL, "token": "admin-token", "agents": []string{"codex", "codex"}})

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token"}, answer{
		label:    whichAgents,
		choices:  []prompt.Choice{{Value: "claude-code", Label: "Claude Code"}, {Value: "codex", Label: "Codex"}},
		defaults: []string{"codex"},
		selected: []string{"codex"},
	})

	assertExit(t, code, 0, output)
}
