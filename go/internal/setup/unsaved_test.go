package setup_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/prompt"
)

// After an email login the server has issued a token. When setup then
// stops without saving it, it revokes that token, which no one could use
// or revoke otherwise. A token given for a token login is the user's own:
// it is never revoked. (The PHP CLI leaves the new token valid.)

var cancelledSelection = answer{
	label:   whichAgents,
	choices: []prompt.Choice{{Value: "claude-code", Label: "Claude Code"}, {Value: "codex", Label: "Codex"}},
	aborted: true,
}

func revokes(s *server) []request { return s.sent(http.MethodDelete, "/api/auth/token") }

func TestRevokesTheNewTokenWhenTheAgentSelectionIsCancelled(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(newClaude(), newCodex())

	output, code := h.run(selectionArgs(s.URL), code123456, cancelledSelection)

	assertExit(t, code, 1, output)
	if got := revokes(s); len(got) != 1 || got[0].Header.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("revokes = %+v, want one with Bearer secret-token", got)
	}
	assertContains(t, output, "Revoked the new memry token.\n")
	h.assertNoConfig()
}

func TestSaysWhenTheNewTokenCannotBeRevoked(t *testing.T) {
	for name, revoke := range map[string]*reply{"server error": {status: 500}, "redirect": {status: 302, location: "http://127.0.0.1:1/login"}, "unreachable": nil} {
		t.Run(name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t, func(s *server) { s.revoke = revoke })
			h.useAgents(newClaude(), newCodex())

			output, code := h.run(selectionArgs(s.URL), code123456, cancelledSelection)

			assertExit(t, code, 1, output)
			assertContains(t, output, "Could not revoke the new memry token.\n")
		})
	}
}

// A 401 means the token is no longer valid: as good as revoked.
func TestTakesARejectedNewTokenAsRevoked(t *testing.T) {
	h, s := newHarness(t), newServer(t, func(s *server) { s.revoke = &reply{status: 401, body: `{"message":"Unauthenticated."}`} })
	h.useAgents(newClaude(), newCodex())

	output, code := h.run(selectionArgs(s.URL), code123456, cancelledSelection)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Revoked the new memry token.\n")
}

func TestNeverRevokesAGivenTokenWhenTheAgentSelectionIsCancelled(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(newClaude(), newCodex())

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token"}, cancelledSelection)

	assertExit(t, code, 1, output)
	if got := revokes(s); len(got) != 0 {
		t.Errorf("revokes = %+v, want none", got)
	}
	assertNotContains(t, output, "Revoked")
	h.assertNoConfig()
}

func TestRevokesTheNewTokenWhenTheCredentialsCannotBeSaved(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.env["MEMRY_CONFIG"] = filepath.Join(h.dir, "a-file", "config.json")
	if err := os.WriteFile(filepath.Join(h.dir, "a-file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	if got := revokes(s); len(got) != 1 || got[0].Header.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("revokes = %+v, want one with Bearer secret-token", got)
	}

	h, s = newHarness(t), newServer(t)
	if err := os.WriteFile(h.configPath, []byte(`"text"`), 0o600); err != nil {
		t.Fatal(err)
	}

	output, code = h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	if got := revokes(s); len(got) != 1 {
		t.Errorf("revokes = %+v, want one", got)
	}
}

func TestNeverRevokesAGivenTokenWhenTheCredentialsCannotBeSaved(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	if err := os.WriteFile(h.configPath, []byte(`"text"`), 0o600); err != nil {
		t.Fatal(err)
	}

	output, code := h.run(tokenArgs(s.URL, "admin-token"))

	assertExit(t, code, 1, output)
	if got := revokes(s); len(got) != 0 {
		t.Errorf("revokes = %+v, want none", got)
	}
}

// Setup overwrites a config that is not valid JSON, like the PHP CLI, but
// never one it cannot read: that fails, keeping the file, and revokes the
// new token. (The PHP CLI stops with an uncaught ErrorException.)
func TestFailsWithoutOverwritingAConfigItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a mode 000 file")
	}
	h, s := newHarness(t), newServer(t)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"codex"}})
	if err := os.Chmod(h.configPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(h.configPath, 0o600) })

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not save the credentials to "+h.configPath+": ")
	assertContains(t, output, "permission denied")
	if len(revokes(s)) != 1 {
		t.Error("did not revoke the new token")
	}
	_ = os.Chmod(h.configPath, 0o600)
	if got := h.config()["token"]; got != "old-token" {
		t.Errorf("token = %v, want the file untouched", got)
	}
}
