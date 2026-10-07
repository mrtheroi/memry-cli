package agents_test

import (
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// Ported from tests/Unit/ProtocolTest.php.

func assertContainsAll(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q does not contain %q", text, part)
		}
	}
}

func TestProtocolTellsTheAgentToLoadTheProjectContextAtTheStartOfEverySession(t *testing.T) {
	assertContainsAll(t, agents.Protocol, "At the start of every session, call get-context", "`.memry.json`")
}

func TestProtocolTellsTheAgentHowToSearchSaveAndSummarizeMemories(t *testing.T) {
	assertContainsAll(t, agents.Protocol, "search-memory", "get-memory", "save-memory", "topic_key", "session-summary")
}

func TestProtocolStaysShortEnoughForTheSmallestGlobalRulesFile(t *testing.T) {
	// Windsurf limits its global rules to 6,000 characters.
	if got := len(agents.Protocol); got >= 1500 {
		t.Errorf("len(Protocol) = %d, want < 1500", got)
	}
}

// The exact text of the PHP CLI's Protocol::text(), so both write the
// same instructions block.
func TestProtocolIsThePHPCLIText(t *testing.T) {
	want := "## memry memory\n\n" +
		"memry gives you a persistent memory through the `memry` MCP tools.\n\n" +
		"- At the start of every session, call get-context with the project name: the `project` in `.memry.json` at the repository root if there is one, otherwise the repository (or folder) name.\n" +
		"- Use search-memory to find older memories and get-memory to read one in full when they are relevant to the task.\n" +
		"- Save decisions, bug fixes and discoveries with save-memory for that project, with a topic_key for evolving topics.\n" +
		"- Before ending the session, save a summary with session-summary (project and repository name)."
	if agents.Protocol != want {
		t.Errorf("Protocol = %q, want %q", agents.Protocol, want)
	}
}
