package setup_test

import (
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// Tests ported from tests/Feature/SetupTokenTest.php.

func tokenArgs(url string, token ...string) []string {
	argv := []string{"--url", url, "--token"}
	return append(append(argv, token...), "--agents", "claude-code")
}

// Ported from "skips the email login when a --token option is given",
// "checks the token against the server before saving it" and "saves the
// url and token and wires the agents without printing the token".
func TestChecksTheTokenAgainstTheServerAndSavesIt(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run(tokenArgs(s.URL, "admin-token"))

	assertExit(t, code, 0, output)
	checks := s.sent(http.MethodGet, "/api/context")
	if len(checks) != 1 || checks[0].Query != "project=memry" ||
		checks[0].Header.Get("Authorization") != "Bearer admin-token" || checks[0].Header.Get("Accept") != "application/json" {
		t.Errorf("token checks = %+v, want one GET /api/context?project=memry with Bearer admin-token accepting JSON", checks)
	}
	for _, r := range s.received() {
		if strings.HasPrefix(r.Path, "/api/auth/") {
			t.Errorf("sent %s %s, want no email login", r.Method, r.Path)
		}
	}
	assertContains(t, output, "Connected to "+s.URL+". Credentials saved to "+h.configPath+".\n")
	assertNotContains(t, output, "admin-token")
	want := map[string]any{"url": s.URL, "token": "admin-token", "agents": []any{"claude-code"}}
	if got := h.config(); !reflect.DeepEqual(got, want) {
		t.Errorf("config = %v, want %v", got, want)
	}
}

// Ported from "fails without saving anything when the server rejects the
// token", "... cannot be reached", "... cannot check the token", "... answers
// the token check with a redirect" and "does not follow a redirect from the
// token check to a page that answers 200".
func TestFailsWithoutSavingAnythingWhenTheTokenCheckFails(t *testing.T) {
	login := newServer(t, func(s *server) { s.context = &reply{status: 200, body: `<html>Log in</html>`} })
	tests := []struct {
		name    string
		context *reply
		want    string
	}{
		{"rejected", &reply{status: 401, body: `{"message":"Unauthenticated."}`}, "The token was rejected by %s."},
		{"server error", &reply{status: 500, body: `Server Error`}, "The memry server returned an unexpected error (HTTP 500)."},
		{"redirect", &reply{status: 302, location: login.URL + "/api/context"}, "The memry server returned an unexpected error (HTTP 302)."},
		{"unreachable", nil, "Could not reach the memry server at %s."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			s := newServer(t, func(s *server) { s.context = tt.context })

			output, code := h.run(tokenArgs(s.URL, "admin-token"))

			assertExit(t, code, 1, output)
			if want := strings.ReplaceAll(tt.want, "%s", s.URL) + "\n"; output != want {
				t.Errorf("output = %q, want %q", output, want)
			}
			h.assertNoConfig()
		})
	}
	if len(login.received()) != 0 {
		t.Error("followed the redirect")
	}
}

// Ported from "asks for the token when --token is given without a value".
func TestAsksForTheTokenWhenTokenHasNoValue(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run(tokenArgs(s.URL), answer{label: "Token", value: "admin-token"})

	assertExit(t, code, 0, output)
	if checks := s.sent(http.MethodGet, "/api/context"); len(checks) != 1 || checks[0].Header.Get("Authorization") != "Bearer admin-token" {
		t.Errorf("token checks = %+v, want one with Bearer admin-token", checks)
	}
	if got := h.config()["token"]; got != "admin-token" {
		t.Errorf("saved token = %v, want admin-token", got)
	}
	assertNotContains(t, output, "admin-token")
}

// Ported from "fails without sending anything when the entered token is
// empty".
func TestFailsWithoutSendingAnythingWhenTheEnteredTokenIsEmpty(t *testing.T) {
	for _, entered := range []string{"", "   "} {
		h, s := newHarness(t), newServer(t)

		output, code := h.run(tokenArgs(s.URL), answer{label: "Token", value: entered})

		assertExit(t, code, 1, output)
		if want := "The token is empty.\n"; output != want {
			t.Errorf("output = %q, want %q", output, want)
		}
		if len(s.received()) != 0 {
			t.Error("sent requests, want none")
		}
		h.assertNoConfig()
	}
}

// Ported from "uses the MEMRY_TOKEN environment variable when --token has
// no value and there is no interaction", "ignores a blank MEMRY_TOKEN
// environment variable", "uses the trimmed MEMRY_TOKEN instead of asking",
// "prefers a --token value over the MEMRY_TOKEN environment variable", "uses
// a MEMRY_TOKEN of "0" instead of treating it as unset" and "trims the
// token before checking and saving it".
func TestUsesTheTokenFromTheOptionTheEnvironmentOrThePrompt(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		argv    []string
		answers []answer
		want    string
	}{
		{"MEMRY_TOKEN without interaction", "env-token", []string{"--no-interaction"}, nil, "env-token"},
		{"a trimmed MEMRY_TOKEN", "  env-token \n", nil, nil, "env-token"},
		{"a MEMRY_TOKEN of 0", "0", []string{"--no-interaction"}, nil, "0"},
		{"a blank MEMRY_TOKEN", "  \n", nil, []answer{{label: "Token", value: "admin-token"}}, "admin-token"},
		{"--token over MEMRY_TOKEN", "env-token", []string{"--token=admin-token"}, nil, "admin-token"},
		{"a trimmed --token", "", []string{"--token=  admin-token \n"}, nil, "admin-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			h.env["MEMRY_TOKEN"] = tt.env

			output, code := h.run(append(tokenArgs(s.URL), tt.argv...), tt.answers...)

			assertExit(t, code, 0, output)
			if checks := s.sent(http.MethodGet, "/api/context"); len(checks) != 1 || checks[0].Header.Get("Authorization") != "Bearer "+tt.want {
				t.Errorf("token checks = %+v, want one with Bearer %s", checks, tt.want)
			}
			if got := h.config()["token"]; got != tt.want {
				t.Errorf("saved token = %v, want %s", got, tt.want)
			}
			if secret := strings.TrimSpace(tt.env); secret != "" && secret != "0" {
				assertNotContains(t, output, secret)
			}
		})
	}
}

// Ported from "ignores the MEMRY_TOKEN environment variable when --token is
// not given".
func TestIgnoresMemryTokenWithoutTheTokenOption(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.env["MEMRY_TOKEN"] = "env-token"

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	if got := h.config()["token"]; got != "secret-token" {
		t.Errorf("saved token = %v, want secret-token", got)
	}
	for _, r := range s.received() {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("sent %s %s with %q, want no token", r.Method, r.Path, r.Header.Get("Authorization"))
		}
	}
}

// Ported from "accepts a plain http server url for self-hosted servers"
// (a path).
func TestAcceptsAServerURLWithAPath(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run(tokenArgs(s.URL+"/memry/", "admin-token"))

	assertExit(t, code, 0, output)
	if len(s.sent(http.MethodGet, "/memry/api/context")) != 1 {
		t.Errorf("sent %+v, want GET /memry/api/context", s.received())
	}
	if got := h.config()["url"]; got != s.URL+"/memry" {
		t.Errorf("saved url = %v, want %s/memry", got, s.URL)
	}
}

// Ported from "revokes the previous token only when it differs from the
// given one".
func TestRevokesThePreviousTokenOnlyWhenItDiffersFromTheGivenOne(t *testing.T) {
	for previous, revoked := range map[string]bool{"old-token": true, "admin-token": false} {
		h, s := newHarness(t), newServer(t)
		h.writeConfig(map[string]any{"url": s.URL, "token": previous})

		output, code := h.run(tokenArgs(s.URL, "admin-token"))

		assertExit(t, code, 0, output)
		revokes := s.sent(http.MethodDelete, "/api/auth/token")
		if got := len(revokes) == 1 && revokes[0].Header.Get("Authorization") == "Bearer "+previous; got != revoked {
			t.Errorf("previous %s: revoked = %v, want %v", previous, got, revoked)
		}
	}
}

// Ported from "removes MEMRY_TOKEN from the environment subprocesses
// inherit once --token is given": a real child process must not see it,
// even when the login then fails.
func TestRemovesMemryTokenFromTheEnvironmentOnceTokenIsGiven(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		exit int
	}{
		{"token from MEMRY_TOKEN", []string{"--no-interaction"}, 0},
		{"token from --token", []string{"--token=admin-token"}, 0},
		{"an empty --token", []string{"--token="}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			t.Setenv("MEMRY_TOKEN", "env-token")
			h.useProcessEnv(t)

			output, code := h.run(append(tokenArgs(s.URL), tt.argv...))

			assertExit(t, code, tt.exit, output)
			if _, ok := os.LookupEnv("MEMRY_TOKEN"); ok {
				t.Error("MEMRY_TOKEN is still set")
			}
			child, err := exec.Command("sh", "-c", `printf %s "${MEMRY_TOKEN-unset}"`).Output()
			if err != nil {
				t.Fatal(err)
			}
			if string(child) != "unset" {
				t.Errorf("a child process sees MEMRY_TOKEN=%q, want it unset", child)
			}
		})
	}
}

// The email login leaves MEMRY_TOKEN alone, like PHP.
func TestKeepsMemryTokenWithoutTheTokenOption(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	t.Setenv("MEMRY_TOKEN", "env-token")
	h.useProcessEnv(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	if value := os.Getenv("MEMRY_TOKEN"); value != "env-token" {
		t.Errorf("MEMRY_TOKEN = %q, want env-token", value)
	}
}
