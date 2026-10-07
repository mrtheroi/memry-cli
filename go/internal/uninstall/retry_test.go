package uninstall_test

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/setup"
	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// When memry cannot be removed from an agent, uninstall keeps what a retry
// needs: the login, and the agents still to clean up in agents_to_remove.
// It neither revokes the token nor deletes the config. (The PHP CLI
// revokes the token and deletes the config anyway, forgetting the agents.)

const retryHint = "Fix the problems above, then run `memry uninstall` again.\n"

func TestUninstallKeepsTheLoginAndTheAgentsStillToCleanUpWhenAnAgentFails(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex, windsurf := &fakeAgent{key: "claude-code", name: "Claude Code"}, &fakeAgent{key: "codex", name: "Codex", fails: true}, &fakeAgent{key: "windsurf", name: "Windsurf", fails: true}
	h.useAgents(claude, codex, windsurf)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token", "project": "kept", "agents": []string{"windsurf", "claude-code", "codex"}})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	want := "Unwired Claude Code.\nCould not unwire Codex.\nCould not unwire Windsurf.\n" +
		"Kept the login and the agents still to clean up in " + h.configPath + ".\n" + retryHint
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if got := s.received(); len(got) != 0 {
		t.Errorf("sent %+v, want no revoke", got)
	}
	wantConfig := map[string]any{"url": s.URL, "token": "old-token", "project": "kept", "agents": []any{}, "agents_to_remove": []any{"codex", "windsurf"}}
	if got := h.config(); !reflect.DeepEqual(got, wantConfig) {
		t.Errorf("config = %v, want %v", got, wantConfig)
	}
}

// Without a config file, uninstall cleans up Claude Code, as setup did up
// to 0.4.0, and a retry does the same: a failure writes no config file.
func TestUninstallWritesNoConfigFileWhenThereWasNone(t *testing.T) {
	h := newHarness(t)
	h.useAgents(&fakeAgent{key: "claude-code", name: "Claude Code", fails: true})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	if want := "Could not unwire Claude Code.\n" + retryHint; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if h.configExists() {
		t.Error("wrote a config file")
	}
}

// What a failed cleanup leaves, or what delete-account leaves: agents
// without a login. Uninstall then retries the agents and deletes it.
func TestUninstallRetriesTheAgentsOfAConfigWithoutALogin(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := &fakeAgent{key: "claude-code", name: "Claude Code"}, &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(claude, codex)
	h.previousConfig(map[string]any{"agents": []string{"codex"}})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	want := "Not logged in; no token to revoke.\nUnwired Codex.\nDeleted " + h.configPath + ".\nRun `brew uninstall memry` to remove the CLI.\n"
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	assertCalls(t, codex, "uninstall")
	if h.configExists() || len(s.received()) != 0 {
		t.Error("kept the config or sent a request")
	}
}

// The account is gone, and its token with it: delete-account keeps only
// the agents still to clean up, for `memry uninstall` to retry.
func TestDeleteAccountKeepsOnlyTheAgentsStillToCleanUpWhenAnAgentFails(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(&fakeAgent{key: "claude-code", name: "Claude Code", fails: true}, &fakeAgent{key: "codex", name: "Codex"})
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"claude-code", "codex"}})

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "ana@example.com"})

	assertExit(t, code, 1, output)
	want := warning + "Deleted your memry account and all its memories.\n" +
		"Could not unwire Claude Code.\nUnwired Codex.\n" +
		"Kept only the agents still to clean up in " + h.configPath + ".\n" + retryHint
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if got, want := h.config(), map[string]any{"agents": []any{}, "agents_to_remove": []any{"claude-code"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("config = %v, want %v", got, want)
	}
	if got := s.received(); len(got) != 1 {
		t.Errorf("requests = %+v, want only the deletion", got)
	}
}

// Uninstall removes memry from the selected agents and from the ones a
// failed removal left in agents_to_remove.
func TestUninstallAlsoRemovesTheAgentsToRemove(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex, windsurf := &fakeAgent{key: "claude-code", name: "Claude Code"}, &fakeAgent{key: "codex", name: "Codex"}, &fakeAgent{key: "windsurf", name: "Windsurf"}
	h.useAgents(claude, codex, windsurf)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"windsurf"}, "agents_to_remove": []string{"codex", "windsurf"}})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertCalls(t, codex, "uninstall")
	assertCalls(t, windsurf, "uninstall")
	assertCalls(t, claude)
	if h.configExists() {
		t.Error("the config file still exists")
	}
}

// What delete-account leaves is only agents_to_remove: uninstall then
// retries those agents, not Claude Code as for a config from before 0.4.0.
func TestUninstallRetriesOnlyTheAgentsToRemoveOfAConfigWithoutAgents(t *testing.T) {
	h := newHarness(t)
	claude, codex := &fakeAgent{key: "claude-code", name: "Claude Code"}, &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(claude, codex)
	h.previousConfig(map[string]any{"agents_to_remove": []string{"codex"}})

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertCalls(t, codex, "uninstall")
	assertCalls(t, claude)
}

// A partial uninstall, then `setup -n`: the saved selection is empty, so
// setup installs nothing and retries the pending removal, instead of
// defaulting to every installed agent.
func TestSetupAfterAPartialUninstallInstallsNothingAndRetriesTheRemoval(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := &fakeAgent{key: "claude-code", name: "Claude Code", fails: true}, &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(claude, codex)
	h.previousConfig(map[string]any{"url": s.URL, "token": "admin-token", "agents": []string{"claude-code", "codex"}})

	output, code := h.run(uninstall.Uninstall, force)
	assertExit(t, code, 1, output)

	claude.fails, claude.calls, codex.calls = false, nil, nil
	var out bytes.Buffer
	code = setup.Run(setup.Env{
		Args: []string{"--url", s.URL, "--token=admin-token", "-n"},
		LookupEnv: func(key string) (string, bool) {
			value, ok := h.env[key]
			return value, ok
		},
		Unsetenv: func(string) error { return nil },
		Out:      &out,
		Agents:   h.agents,
		HTTP:     client.New("test", 5*time.Second),
	})

	assertExit(t, code, 0, out.String())
	assertCalls(t, claude, "uninstall")
	assertCalls(t, codex)
	if got, want := h.config(), map[string]any{"url": s.URL, "token": "admin-token", "agents": []any{}}; !reflect.DeepEqual(got, want) {
		t.Errorf("config = %v, want %v", got, want)
	}
}
