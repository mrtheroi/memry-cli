package uninstall_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// Tests ported from tests/Feature/DeleteAccountCommandTest.php.

const typeYourEmail = "Type your account email to confirm"

const warning = "This permanently deletes your memry account and ALL its memories on the server. It cannot be undone.\n"

func TestDeleteAccountFailsWhenNotLoggedIn(t *testing.T) {
	for name, config := range map[string]map[string]any{"no config file": nil, "config without a token": {"url": "https://memry.test"}} {
		t.Run(name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			if config != nil {
				h.previousConfig(config)
			}

			output, code := h.run(uninstall.DeleteAccount, nil)

			assertExit(t, code, 1, output)
			if output != "You are not logged in to memry.\n" {
				t.Errorf("output = %q", output)
			}
			if len(s.received()) != 0 || len(h.claude.ran) != 0 {
				t.Error("sent or ran something")
			}
		})
	}
}

func TestDeleteAccountWarnsAndDeletesNothingWhenTheConfirmationIsLeftEmpty(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: ""})

	assertExit(t, code, 1, output)
	if want := warning + "Aborted; nothing was deleted.\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if len(s.received()) != 0 || len(h.claude.ran) != 0 || !h.configExists() {
		t.Error("deleted something")
	}
}

func TestDeleteAccountAsksAgainWhenTheConfirmationIsNotAValidEmailAddress(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.DeleteAccount, nil,
		answer{label: typeYourEmail, value: "not-an-email"},
		answer{label: typeYourEmail, value: ""})

	assertExit(t, code, 1, output)
	if want := warning + "Enter a valid email address.\nAborted; nothing was deleted.\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if len(s.received()) != 0 {
		t.Error("sent a request")
	}
}

func TestDeleteAccountDeletesTheAccountOnTheServerItIsLoggedInTo(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "  Ana@Example.com "})

	assertExit(t, code, 0, output)
	assertContains(t, output, "Deleted your memry account and all its memories.\n")
	got := s.received()
	if len(got) != 1 {
		t.Fatalf("requests = %+v, want one", got)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(got[0].Body), &body)
	if got[0].Method != http.MethodDelete || got[0].Path != "/api/account" || got[0].Header.Get("Authorization") != "Bearer old-token" ||
		got[0].Header.Get("Accept") != "application/json" || got[0].Header.Get("Content-Type") != "application/json" ||
		len(body) != 1 || body["email"] != "ana@example.com" {
		t.Errorf("request = %+v, want DELETE /api/account with Bearer old-token and {\"email\":\"ana@example.com\"}", got[0])
	}
}

func TestDeleteAccountFailsAndRemovesNothingLocallyWhenTheServerRefuses(t *testing.T) {
	tests := map[string]struct {
		account *reply
		want    string
	}{
		"email does not match":  {&reply{status: 422, body: `{"message":"The email does not match this account."}`}, "The email does not match your memry account.\n"},
		"login no longer valid": {&reply{status: 401, body: `{"message":"Unauthenticated."}`}, "Your memry login is no longer valid. Run `memry setup` and try again.\n"},
		"server error":          {&reply{status: 500, body: "Server Error"}, "Could not delete your memry account. Try again later.\n"},
		"unreachable":           {nil, "Could not delete your memry account. Try again later.\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t, func(s *server) { s.account = tt.account })
			h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

			output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "bob@example.com"})

			assertExit(t, code, 1, output)
			if want := warning + tt.want; output != want {
				t.Errorf("output = %q, want %q", output, want)
			}
			if len(h.claude.ran) != 0 || !h.configExists() {
				t.Error("removed something locally")
			}
		})
	}
}

func TestDeleteAccountDoesNotFollowARedirectFromTheDeletionToAPageThatAnswers200(t *testing.T) {
	h := newHarness(t)
	s := newServer(t, func(s *server) { s.account = &reply{status: 302} })
	s.account.location = s.URL + "/login"
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "ana@example.com"})

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not delete your memry account. Try again later.\n")
	assertNotContains(t, output, "Deleted your memry account")
	for _, r := range s.received() {
		if r.Path == "/login" {
			t.Error("followed the redirect to /login")
		}
	}
	if len(h.claude.ran) != 0 || !h.configExists() {
		t.Error("removed something locally")
	}
}

func TestDeleteAccountRemovesMemryFromThisMachineAfterDeletingTheAccount(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})
	writeRaw(t, h.settingsPath, `{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"'/opt/memry/memry' hook:session-start","timeout":10}]}]}}`)

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "ana@example.com"})

	assertExit(t, code, 0, output)
	want := warning +
		"Deleted your memry account and all its memories.\n" +
		"Removed the memry MCP server from Claude Code.\n" +
		"Removed the memry SessionStart hook from " + h.settingsPath + ".\n" +
		"Deleted " + h.configPath + ".\n" +
		"Run `brew uninstall memry` to remove the CLI.\n"
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	assertRan(t, h.claude, []string{"claude", "mcp", "remove", "--scope", "user", "memry"})
	assertRan(t, h.claude, []string{"claude", "mcp", "remove", "--scope", "user", "db-memory"})
	if h.configExists() {
		t.Error("the config file still exists")
	}
	if got := readText(t, h.settingsPath); got != "[]" && got != "{}" && got != "[]\n" && got != "{}\n" {
		t.Errorf("settings = %q, want them empty", got)
	}
	if got := s.received(); len(got) != 1 {
		t.Errorf("requests = %+v, want only the deletion (no revoke)", got)
	}
}

func TestDeleteAccountKeepsCleaningUpAndFailsWhenALocalStepFails(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.claude.missing = true
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "ana@example.com"})

	assertExit(t, code, 1, output)
	assertContains(t, output, "Deleted your memry account and all its memories.\n")
	assertContains(t, output, "Claude Code CLI not found; skipped removing the memry MCP server.\n")
	// Unlike the PHP CLI, which deletes the config anyway, the agent is
	// kept for `memry uninstall` to retry (see retry_test.go).
	assertContains(t, output, "No memry SessionStart hook to remove.\n")
	assertContains(t, output, retryHint)
}

func TestDeleteAccountRemovesMemryFromTheAgentsSavedBySetup(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	claude, codex := &fakeAgent{key: "claude-code", name: "Claude Code"}, &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(claude, codex)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"codex"}})

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "ana@example.com"})

	assertExit(t, code, 0, output)
	assertContains(t, output, "Unwired Codex.\n")
	assertCalls(t, codex, "uninstall")
	assertCalls(t, claude)
}

// Like Symfony, a question without interaction has no answer: empty.
func TestDeleteAccountDeletesNothingWithoutInteraction(t *testing.T) {
	for _, option := range []string{"-n", "-q"} {
		h, s := newHarness(t), newServer(t)
		h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

		output, code := h.run(uninstall.DeleteAccount, []string{option})

		assertExit(t, code, 1, output)
		if option == "-n" && output != warning+"Aborted; nothing was deleted.\n" {
			t.Errorf("output = %q", output)
		}
		if len(s.received()) != 0 || !h.configExists() {
			t.Errorf("%s: deleted something", option)
		}
	}
}

func TestDeleteAccountAbortsWhenTheInputEndsBeforeTheEmail(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, aborted: true})

	assertExit(t, code, 1, output)
	if want := warning + "\n            \n  Aborted.  \n            \n\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if len(s.received()) != 0 {
		t.Error("sent a request")
	}
}

// delete-account has no --force: it takes only the global options.
func TestDeleteAccountRejectsArgumentsLikeSymfony(t *testing.T) {
	for arg, message := range map[string]string{
		"--force": `The "--force" option does not exist.`,
		"extra":   `No arguments expected for "delete-account" command, got "extra".`,
	} {
		h := newHarness(t)

		output, code := h.run(uninstall.DeleteAccount, []string{arg})

		assertExit(t, code, 1, output)
		assertContains(t, output, "  "+message+"  \n")
	}
}
