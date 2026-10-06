package agents_test

import (
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// The keys of PHP's AgentRegistry, in its display order.
func TestStubListsThePHPAgentKeysInOrder(t *testing.T) {
	want := []string{"claude-code", "codex", "opencode", "antigravity", "windsurf"}
	if got := (agents.Stub{}).Keys(); !reflect.DeepEqual(got, want) {
		t.Errorf("Keys() = %q, want %q", got, want)
	}
}
