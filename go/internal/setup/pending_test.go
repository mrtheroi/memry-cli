package setup_test

import (
	"os"
	"reflect"
	"testing"
)

// A deselected agent memry could not be removed from is kept in
// agents_to_remove, apart from the selection in agents, and later setups
// retry the removal until it succeeds. (The PHP CLI forgets it; it keeps
// the agents_to_remove key it does not know as it is.)

func (h *harness) agentsToRemove() (any, bool) {
	h.t.Helper()
	value, ok := h.config()["agents_to_remove"]
	return value, ok
}

func TestKeepsADeselectedAgentToRemoveUntilMemryIsRemovedFromIt(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex, windsurf := newClaude(), newCodex(), &fakeAgent{key: "windsurf", name: "Windsurf"}
	claude.fails, windsurf.fails = true, true
	h.useAgents(claude, codex, windsurf)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"windsurf", "claude-code"}})

	output, code := h.run(selectionArgs(s.URL, "--agents", "codex"), code123456)

	assertExit(t, code, 1, output)
	assertSavedAgents(t, h, "codex")
	if got, _ := h.agentsToRemove(); !reflect.DeepEqual(got, []any{"claude-code", "windsurf"}) {
		t.Errorf("agents_to_remove = %v, want [claude-code windsurf]", got)
	}
}

// Without interaction the default is the saved selection: the agent to
// remove is removed again, never reinstalled.
func TestRetriesTheRemovalWithoutReinstallingTheAgent(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	h.useAgents(claude, codex)
	if err := os.WriteFile(h.configPath, []byte(`{"url":"`+s.URL+`","token":"admin-token","agents":["codex"],"agents_to_remove":["claude-code"]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "-n"})

	assertExit(t, code, 0, output)
	assertCalls(t, claude, "uninstall")
	assertCalls(t, codex, "install "+s.URL)
	assertContains(t, output, "Claude Code: memry was removed.\n")
	// Once there is nothing left to remove, the key is gone: the file is
	// what the PHP CLI writes.
	data, _ := os.ReadFile(h.configPath)
	if want := "{\n    \"url\": \"" + s.URL + "\",\n    \"token\": \"admin-token\",\n    \"agents\": [\n        \"codex\"\n    ]\n}\n"; string(data) != want {
		t.Errorf("config = %q, want %q", data, want)
	}
}

func TestKeepsTheAgentToRemoveWhenTheRetryFailsAgain(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude := newClaude()
	claude.fails = true
	h.useAgents(claude, newCodex())
	h.writeConfig(map[string]any{"url": s.URL, "token": "admin-token", "agents": []string{"codex"}, "agents_to_remove": []string{"claude-code"}})

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "-n"})

	assertExit(t, code, 1, output)
	assertCalls(t, claude, "uninstall")
	if got, _ := h.agentsToRemove(); !reflect.DeepEqual(got, []any{"claude-code"}) {
		t.Errorf("agents_to_remove = %v, want [claude-code]", got)
	}
}

// Selecting the agent again installs it instead of removing it.
func TestSelectingAnAgentToRemoveAgainInstallsIt(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := newClaude(), newCodex()
	h.useAgents(claude, codex)
	h.writeConfig(map[string]any{"url": s.URL, "token": "admin-token", "agents": []string{"codex"}, "agents_to_remove": []string{"claude-code"}})

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "--agents=claude-code,codex"})

	assertExit(t, code, 0, output)
	assertCalls(t, claude, "install "+s.URL)
	if got, ok := h.agentsToRemove(); ok {
		t.Errorf("agents_to_remove = %v, want no key", got)
	}
	assertSavedAgents(t, h, "claude-code", "codex")
}
