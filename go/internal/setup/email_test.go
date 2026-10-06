package setup_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// Tests ported from tests/Feature/SetupCommandTest.php (the login part).

func emailArgs(url string) []string {
	return []string{"--url", url, "--email", "ana@example.com", "--agents", "claude-code"}
}

func TestWritesTheURLAndTokenToTheConfigFileOnSuccess(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	want := map[string]any{"url": s.URL, "token": "secret-token", "agents": []any{"claude-code"}}
	if got := h.config(); !reflect.DeepEqual(got, want) {
		t.Errorf("config = %v, want %v", got, want)
	}
}

// Ported from "tells the user where the login code was sent" and "confirms
// the login without ever printing the token".
func TestConfirmsTheLoginWithoutEverPrintingTheToken(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "We sent a login code to ana@example.com.\n")
	assertContains(t, output, "Logged in as ana@example.com. Credentials saved to "+h.configPath+".\n")
	assertNotContains(t, output, "secret-token")
}

// Ported from "sends JSON requests that accept JSON responses".
func TestSendsJSONRequestsThatAcceptJSONResponses(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	for path, want := range map[string]map[string]any{
		"/api/auth/code":  {"email": "ana@example.com"},
		"/api/auth/token": {"email": "ana@example.com", "code": "123456"},
	} {
		sent := s.sent(http.MethodPost, path)
		if len(sent) != 1 {
			t.Fatalf("POST %s sent %d times, want once", path, len(sent))
		}
		r := sent[0]
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("POST %s Accept, Content-Type = %q, %q; want application/json", path, r.Header.Get("Accept"), r.Header.Get("Content-Type"))
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(r.Body), &got); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("POST %s body = %s, want %v", path, r.Body, want)
		}
	}
}

// Ported from "fails without sending anything when the --email option is
// not valid UTF-8" and "... is not an email address".
func TestFailsWithoutSendingAnythingWhenTheEmailOptionIsInvalid(t *testing.T) {
	for _, address := range []string{"ana@example.co\xc3m", "not-an-email"} {
		h, s := newHarness(t), newServer(t)

		output, code := h.run([]string{"--url", s.URL, "--email", address, "--agents", "claude-code"})

		assertExit(t, code, 1, output)
		if want := "Invalid email address given with --email.\n"; output != want {
			t.Errorf("output = %q, want %q", output, want)
		}
		if len(s.received()) != 0 {
			t.Errorf("sent requests for %q, want none", address)
		}
		h.assertNoConfig()
	}
}

// Ported from "asks for the email when no --email option is given", "asks
// for the email again until it is valid" and "trims and lowercases the
// email before sending it".
func TestAsksForTheEmailUntilItIsValid(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run([]string{"--url", s.URL, "--agents", "claude-code"},
		answer{label: "Email", value: "ana@example.co\xc3m"},
		answer{label: "Email", value: ""},
		answer{label: "Email", value: "  Ana@Example.COM "},
		code123456)

	assertExit(t, code, 0, output)
	if want := "Enter a valid email address.\nEnter a valid email address.\nWe sent a login code to ana@example.com.\n"; !strings.HasPrefix(output, want) {
		t.Errorf("output = %q, want it to start with %q", output, want)
	}
	if sent := s.received(); len(sent) != 2 || sent[0].Body != `{"email":"ana@example.com"}` ||
		sent[1].Body != `{"email":"ana@example.com","code":"123456"}` {
		t.Errorf("sent %+v, want the code and token requests for ana@example.com", sent)
	}
}

// Ported from "fails without sending anything when not interactive and no
// --email option is given".
func TestFailsWithoutAnEmailWhenNotInteractive(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run([]string{"--url", s.URL, "--agents", "claude-code", "--no-interaction"})

	assertExit(t, code, 1, output)
	if want := "Pass --email when running without interaction.\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if len(s.received()) != 0 {
		t.Error("sent requests, want none")
	}
	h.assertNoConfig()
}

// Ported from "fails without asking again when no login code can be read":
// without interaction there is no answer, as with an empty one.
func TestFailsWithoutAskingAgainWhenNoLoginCodeCanBeRead(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		answers []answer
	}{
		{"no interaction", []string{"--no-interaction"}, nil},
		{"an empty answer", nil, []answer{{label: "Login code", value: ""}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)

			output, code := h.run(append(emailArgs(s.URL), tt.argv...), tt.answers...)

			assertExit(t, code, 1, output)
			want := "We sent a login code to ana@example.com.\nNo login code given. Run `memry setup` interactively to enter the code from the email.\n"
			if output != want {
				t.Errorf("output = %q, want %q", output, want)
			}
			if sent := s.sent(http.MethodPost, "/api/auth/token"); len(sent) != 0 {
				t.Error("sent the token request, want none")
			}
			h.assertNoConfig()
		})
	}
}

// Ported from "asks for the login code again until it has 6 digits".
func TestAsksForTheLoginCodeAgainUntilItHasSixDigits(t *testing.T) {
	h, s := newHarness(t), newServer(t)

	output, code := h.run(emailArgs(s.URL),
		answer{label: "Login code", value: "12345\xc3"},
		answer{label: "Login code", value: "１２３４５６"},
		answer{label: "Login code", value: "1234567"},
		answer{label: "Login code", value: "123456"})

	assertExit(t, code, 0, output)
	if got := strings.Count(output, "Enter the 6-digit code from the email.\n"); got != 3 {
		t.Errorf("asked again %d times, want 3:\n%s", got, output)
	}
	if sent := s.sent(http.MethodPost, "/api/auth/token"); len(sent) != 1 || sent[0].Body != `{"email":"ana@example.com","code":"123456"}` {
		t.Errorf("token requests = %+v, want one with the code 123456", sent)
	}
}

// The PHP CLI aborts with "Aborted." (exit 1) when the input ends before
// an answer, as with `memry setup < /dev/null`; it never asks forever.
func TestAbortsWhenTheInputEndsBeforeAnAnswer(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		answers []answer
		want    string
	}{
		{"email", []string{"--agents", "claude-code"}, []answer{{label: "Email", aborted: true}}, "Aborted.\n"},
		{"login code", []string{"--email", "ana@example.com"}, []answer{{label: "Login code", aborted: true}},
			"We sent a login code to ana@example.com.\nAborted.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)

			output, code := h.run(append([]string{"--url", s.URL}, tt.argv...), tt.answers...)

			assertExit(t, code, 1, output)
			if output != tt.want {
				t.Errorf("output = %q, want %q", output, tt.want)
			}
			if sent := s.sent(http.MethodPost, "/api/auth/token"); len(sent) != 0 {
				t.Error("sent the token request, want none")
			}
			h.assertNoConfig()
		})
	}
}
