package setup_test

import "testing"

// failure is a failed email login: the server's reply to one endpoint.
type failure struct {
	name     string
	code     *reply // the reply to POST /api/auth/code, if not the default
	token    *reply // the reply to POST /api/auth/token, if not the default
	answers  []answer
	want     string
	requests int
}

func runFailures(t *testing.T, failures []failure) {
	t.Helper()
	for _, f := range failures {
		t.Run(f.name, func(t *testing.T) {
			h := newHarness(t)
			s := newServer(t, func(s *server) {
				if f.code != nil {
					s.code = f.code
				}
				if f.token != nil {
					s.token = f.token
				}
			})

			output, code := h.run(emailArgs(s.URL), f.answers...)

			assertExit(t, code, 1, output)
			assertContains(t, output, f.want+"\n")
			assertNotContains(t, output, "Logged in")
			h.assertNoConfig()
			if got := len(s.received()); got != f.requests {
				t.Errorf("sent %d requests, want %d", got, f.requests)
			}
		})
	}
}

// Ported from "fails without writing the config when the email is
// rejected" and "... when the code is invalid or expired".
func TestFailsWithTheServerMessageWhenTheLoginIsRejected(t *testing.T) {
	runFailures(t, []failure{
		{
			name:     "email rejected",
			code:     &reply{status: 422, body: `{"message":"The email field must be a valid email address.","errors":{"email":["The email field must be a valid email address."]}}`},
			want:     "The email field must be a valid email address.",
			requests: 1,
		},
		{
			name:     "invalid code",
			token:    &reply{status: 422, body: `{"message":"Invalid or expired code."}`},
			answers:  []answer{{label: "Login code", value: "000000"}},
			want:     "Invalid or expired code.",
			requests: 2,
		},
	})
}

// Ported from "fails without writing the config when rate limited".
func TestFailsWhenRateLimited(t *testing.T) {
	tooMany := &reply{status: 429, body: `{"message":"Too Many Attempts."}`}
	runFailures(t, []failure{
		{name: "code", code: tooMany, want: "Too many attempts, try again later.", requests: 1},
		{name: "token", token: tooMany, answers: []answer{code123456}, want: "Too many attempts, try again later.", requests: 2},
	})
}

// Ported from "fails with a generic message when the server errors without
// a message", for both requests.
func TestFailsWithAGenericMessageWhenTheServerErrorsWithoutAMessage(t *testing.T) {
	serverError := &reply{status: 500, body: `Server Error`}
	want := "The memry server returned an unexpected error (HTTP 500)."
	runFailures(t, []failure{
		{name: "code", code: serverError, want: want, requests: 1},
		{name: "token", token: serverError, answers: []answer{code123456}, want: want, requests: 2},
		{name: "a message that is not a string", code: &reply{status: 500, body: `{"message":["a","b"]}`}, want: want, requests: 1},
		{name: "a JSON list", code: &reply{status: 500, body: `[{"message":"nested"}]`}, want: want, requests: 1},
	})
}

// Ported from "fails without writing the config when the server answers
// without a token".
func TestFailsWhenTheServerAnswersWithoutAToken(t *testing.T) {
	var failures []failure
	for _, body := range []string{`[]`, `{}`, `{"token":""}`, `{"token":5}`, `not json`} {
		failures = append(failures, failure{
			name: body, token: &reply{status: 200, body: body}, answers: []answer{code123456},
			want: "The memry server did not return a token.", requests: 2,
		})
	}
	runFailures(t, failures)
}

// Ported from "fails without writing the config when the server cannot be
// reached", for both requests.
func TestFailsWhenTheServerCannotBeReached(t *testing.T) {
	t.Run("code", func(t *testing.T) {
		h := newHarness(t)
		url := unreachableURL(t)

		output, code := h.run(emailArgs(url))

		assertExit(t, code, 1, output)
		if want := "Could not reach the memry server at " + url + ".\n"; output != want {
			t.Errorf("output = %q, want %q", output, want)
		}
		h.assertNoConfig()
	})
	t.Run("token", func(t *testing.T) {
		h := newHarness(t)
		s := newServer(t, func(s *server) { s.token = nil })

		output, code := h.run(emailArgs(s.URL), code123456)

		assertExit(t, code, 1, output)
		assertContains(t, output, "Could not reach the memry server at "+s.URL+".\n")
		h.assertNoConfig()
	})
}

// Ported from "does not follow a redirect from the login code request to a
// page that answers 200" and "... from the token request to a page that
// answers with a token". The no-redirect client (internal/client) does it;
// this guards that setup reports the redirect as an error.
func TestDoesNotFollowARedirect(t *testing.T) {
	elsewhere := newServer(t, func(s *server) {
		s.code, s.token = &reply{status: 200, body: `{"message":"OK"}`}, &reply{status: 200, body: `{"token":"other-token"}`}
	})
	redirect := &reply{status: 302, location: elsewhere.URL + "/api/auth/code"}
	runFailures(t, []failure{
		{name: "code", code: redirect, want: "The memry server returned an unexpected error (HTTP 302).", requests: 1},
		{name: "token", token: redirect, answers: []answer{code123456}, want: "The memry server returned an unexpected error (HTTP 302).", requests: 2},
	})
	if len(elsewhere.received()) != 0 {
		t.Error("followed the redirect")
	}
}
