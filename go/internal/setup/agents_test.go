package setup_test

import (
	"reflect"
	"strings"
	"testing"
)

// Tests ported from tests/Feature/SetupAgentSelectionTest.php: the
// selection is resolved and saved; asking for it and wiring the agents are
// not ported yet. (The PHP tests answer the login code with
// --no-interaction, which only Laravel's question mock allows; here the
// selection is never asked, so they run interactively.)

func noAgentArgs(url string) []string {
	return []string{"--url", url, "--email", "ana@example.com"}
}

// Ported from "wires the installed agents without asking when not
// interactive".
func TestSelectsTheInstalledAgentsWithoutASavedSelection(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.agents = fakeAgents{keys: []string{"claude-code", "codex", "windsurf"}, installed: map[string]bool{"windsurf": true, "claude-code": true}}

	output, code := h.run(noAgentArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	if got, want := h.config()["agents"], []any{"claude-code", "windsurf"}; !reflect.DeepEqual(got, want) {
		t.Errorf("agents = %v, want %v", got, want)
	}
}

// Ported from "wires the previously saved agents without asking when not
// interactive" and "drops saved agents this version does not support". The
// saved order is kept, like PHP's array_intersect, and an empty saved
// selection stays empty.
func TestSelectsTheSavedAgentsThisVersionSupports(t *testing.T) {
	tests := []struct {
		saved []any
		want  []any
	}{
		{[]any{"codex"}, []any{"codex"}},
		{[]any{"cursor", "codex"}, []any{"codex"}},
		{[]any{"codex", "claude-code"}, []any{"codex", "claude-code"}},
		{[]any{}, []any{}},
	}
	for _, tt := range tests {
		h, s := newHarness(t), newServer(t)
		h.agents = fakeAgents{keys: []string{"claude-code", "codex"}, installed: map[string]bool{"claude-code": true}}
		h.writeConfig(map[string]any{"url": s.URL, "token": "secret-token", "agents": tt.saved})

		output, code := h.run(noAgentArgs(s.URL), code123456)

		assertExit(t, code, 0, output)
		if got := h.config()["agents"]; !reflect.DeepEqual(got, tt.want) {
			t.Errorf("saved %v: agents = %v, want %v", tt.saved, got, tt.want)
		}
	}
}

const notWired = "Agent wiring is not implemented in the Go build yet; no agent was set up or removed.\n"

// Ported from "warns but keeps the login when no agent is selected" (with
// --agents= instead of the prompt).
func TestWarnsButKeepsTheLoginWhenNoAgentIsSelected(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com", "--agents="}, code123456)

	assertExit(t, code, 0, output)
	want := "Credentials saved to " + h.configPath + ".\nNo agents selected; memry is not wired into any agent. Run `memry setup` again to choose some.\n\n"
	if !strings.HasSuffix(output, want) {
		t.Errorf("output = %q, want it to end with %q", output, want)
	}
	if got := h.config()["agents"]; !reflect.DeepEqual(got, []any{}) {
		t.Errorf("agents = %v, want []", got)
	}
}

// Agent wiring is phase 4: setup says so, once, instead of the PHP
// per-agent lines, whenever agents would be set up or removed.
func TestSaysOnceThatAgentWiringIsNotImplemented(t *testing.T) {
	tests := []struct {
		name   string
		agents string
		saved  []any
	}{
		{"agents to set up", "--agents=claude-code,codex", nil},
		{"an agent to remove", "--agents=", []any{"codex"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			if tt.saved != nil {
				h.writeConfig(map[string]any{"agents": tt.saved})
			}

			output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com", tt.agents}, code123456)

			assertExit(t, code, 0, output)
			if !strings.HasSuffix(output, "\n\n"+notWired) || strings.Count(output, notWired) != 1 {
				t.Errorf("output = %q, want it to end with a blank line and %q", output, notWired)
			}
			assertNotContains(t, output, "memry is set up")
		})
	}
}
