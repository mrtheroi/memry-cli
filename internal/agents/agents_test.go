package agents_test

import (
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// Ported from tests/Unit/AgentRegistryTest.php.
func TestRegistrySupportsClaudeCodeCodexOpenCodeAntigravityAndWindsurfInThatOrder(t *testing.T) {
	registry := agents.New(agents.Env{})

	var names []string
	for _, agent := range registry.All() {
		names = append(names, agent.Name())
	}
	if got, want := registry.Keys(), []string{"claude-code", "codex", "opencode", "antigravity", "windsurf"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Keys() = %q, want %q", got, want)
	}
	if want := []string{"Claude Code", "Codex", "OpenCode", "Antigravity", "Windsurf"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %q, want %q", names, want)
	}
}

// fakeAgent is an Agent that records its calls, like the PHP tests' FakeAgent.
type fakeAgent struct {
	key       string
	installed bool
	calls     []string
}

func (f *fakeAgent) Key() string       { return f.key }
func (f *fakeAgent) Name() string      { return f.key }
func (f *fakeAgent) IsInstalled() bool { return f.installed }
func (f *fakeAgent) Install(url string) agents.Result {
	f.calls = append(f.calls, "install "+url)
	return agents.Result{Successful: true}
}
func (f *fakeAgent) Uninstall() agents.Result {
	f.calls = append(f.calls, "uninstall")
	return agents.Result{Successful: true}
}

// Like PHP's AgentRegistry::only(): the agents with the given keys, in
// display order, not in the order of the keys.
func TestRegistryOnlyReturnsTheAgentsWithTheGivenKeysInDisplayOrder(t *testing.T) {
	claude, codex, windsurf := &fakeAgent{key: "claude-code"}, &fakeAgent{key: "codex"}, &fakeAgent{key: "windsurf"}
	registry := agents.NewSupported(claude, codex, windsurf)

	got := registry.Only([]string{"windsurf", "claude-code", "unknown"})

	if want := []agents.Agent{claude, windsurf}; !reflect.DeepEqual(got, want) {
		t.Errorf("Only = %v, want %v", got, want)
	}
}
