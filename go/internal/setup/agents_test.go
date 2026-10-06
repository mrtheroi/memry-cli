package setup_test

import (
	"reflect"
	"strings"
	"testing"
)

// Agent selection and wiring arrive with the agent adapters (phase 4). Until
// then the Go build must never change the saved selection: the PHP CLI reads
// the same config file, and a saved empty or different selection would make
// it unwire the user's agents on its next run.

const notWired = "Agent wiring is not implemented in the Go build yet: no agent was set up or removed, and the saved agent selection was left unchanged.\n"

func TestLeavesTheSavedAgentSelectionUntouched(t *testing.T) {
	for _, agentsArg := range []string{"", "--agents=claude-code", "--agents="} {
		t.Run("arg "+agentsArg, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []any{"codex"}})
			args := []string{"--url", s.URL, "--email", "ana@example.com"}
			if agentsArg != "" {
				args = append(args, agentsArg)
			}

			output, code := h.run(args, code123456)

			assertExit(t, code, 0, output)
			if got, want := h.config()["agents"], []any{"codex"}; !reflect.DeepEqual(got, want) {
				t.Errorf("agents = %v, want %v", got, want)
			}
		})
	}
}

func TestWritesNoAgentSelectionWithoutASavedOne(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com"}, code123456)

	assertExit(t, code, 0, output)
	if _, ok := h.config()["agents"]; ok {
		t.Errorf("config has agents %v, want no agents key", h.config()["agents"])
	}
}

func TestSaysOnceThatAgentWiringIsNotImplemented(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com", "--agents=claude-code,codex"}, code123456)

	assertExit(t, code, 0, output)
	if !strings.HasSuffix(output, "\n"+notWired) || strings.Count(output, notWired) != 1 {
		t.Errorf("output = %q, want it to end with %q once", output, notWired)
	}
	assertNotContains(t, output, "No agents selected")
	assertNotContains(t, output, "memry is set up")
}
