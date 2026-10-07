package setup_test

import (
	"net/http"
	"testing"
)

// Ported from "revokes the previous token after saving the new one".
func TestRevokesThePreviousTokenAfterSavingTheNewOne(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Credentials saved to "+h.configPath+".\nRevoked the previous memry token.\n")
	revokes := s.sent(http.MethodDelete, "/api/auth/token")
	if len(revokes) != 1 {
		t.Fatalf("sent %d revokes, want 1", len(revokes))
	}
	if got := revokes[0].Header; got.Get("Authorization") != "Bearer old-token" || got.Get("Accept") != "application/json" {
		t.Errorf("revoke Authorization, Accept = %q, %q; want Bearer old-token, application/json", got.Get("Authorization"), got.Get("Accept"))
	}
}

// Ported from "keeps going when the previous token cannot be revoked".
func TestKeepsGoingWhenThePreviousTokenCannotBeRevoked(t *testing.T) {
	for name, revoke := range map[string]*reply{
		"already revoked": {status: 401, body: `{"message":"Unauthenticated."}`},
		"server error":    {status: 500, body: `Server Error`},
		"redirect":        {status: 302, location: "/elsewhere"},
		"unreachable":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			s := newServer(t, func(s *server) { s.revoke = revoke })
			h.writeConfig(map[string]any{"url": s.URL, "token": "old-token"})

			output, code := h.run(emailArgs(s.URL), code123456)

			assertExit(t, code, 0, output)
			assertContains(t, output, "Credentials saved to "+h.configPath+".\nCould not revoke the previous memry token.\n")
			assertNotContains(t, output, "Revoked the previous memry token.")
			if got := h.config()["token"]; got != "secret-token" {
				t.Errorf("saved token = %v, want secret-token", got)
			}
		})
	}
}

// Ported from "does not revoke anything when there was no previous login"
// and "does not revoke the previous token when the server returns the same
// one".
func TestDoesNotRevokeWithoutADifferentPreviousToken(t *testing.T) {
	for name, previous := range map[string]map[string]any{
		"no config file":          nil,
		"config without a token":  {"project": "kept"},
		"a token that is no text": {"token": 5},
		"the same token":          {"token": "secret-token"},
	} {
		t.Run(name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			if previous != nil {
				previous["url"] = s.URL
				h.writeConfig(previous)
			}

			output, code := h.run(emailArgs(s.URL), code123456)

			assertExit(t, code, 0, output)
			assertNotContains(t, output, "Revoked")
			assertNotContains(t, output, "Could not revoke")
			if len(s.sent(http.MethodDelete, "/api/auth/token")) != 0 {
				t.Error("sent a revoke, want none")
			}
		})
	}
}

// Ported from "revokes the previous token on the server it belongs to".
func TestRevokesThePreviousTokenOnTheServerItBelongsTo(t *testing.T) {
	h, s, old := newHarness(t), newServer(t), newServer(t)
	h.writeConfig(map[string]any{"url": old.URL, "token": "old-token"})

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	if revokes := old.sent(http.MethodDelete, "/api/auth/token"); len(revokes) != 1 || revokes[0].Header.Get("Authorization") != "Bearer old-token" {
		t.Errorf("revokes on the old server = %+v, want one with Bearer old-token", revokes)
	}
	if len(s.sent(http.MethodDelete, "/api/auth/token")) != 0 {
		t.Error("sent a revoke to the new server")
	}
}

// Ported from "does not revoke the previous token when the login fails".
func TestDoesNotRevokeThePreviousTokenWhenTheLoginFails(t *testing.T) {
	h := newHarness(t)
	s := newServer(t, func(s *server) { s.token = &reply{status: 422, body: `{"message":"Invalid or expired code."}`} })
	h.writeConfig(map[string]any{"url": s.URL, "token": "old-token"})

	output, code := h.run(emailArgs(s.URL), answer{label: "Login code", value: "000000"})

	assertExit(t, code, 1, output)
	if len(s.sent(http.MethodDelete, "/api/auth/token")) != 0 {
		t.Error("sent a revoke, want none")
	}
	if got := h.config()["token"]; got != "old-token" {
		t.Errorf("token = %v, want old-token", got)
	}
}
