package setup_test

import (
	"strings"
	"testing"
)

// A misbehaving server must not make setup buffer an unbounded body: any
// response over 1 MiB fails the login and nothing is saved.
func TestRejectsAResponseLargerThanOneMebibyte(t *testing.T) {
	h := newHarness(t)
	s := newServer(t, func(s *server) {
		s.token = &reply{status: 200, body: `{"token":"secret-token","padding":"` + strings.Repeat("x", 1<<20) + `"}`}
	})

	output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com"}, code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "The memry server at "+s.URL+" sent a response that is too large.")
	h.assertNoConfig()
}

// Like PHP's json_decode, a body that is not valid UTF-8 is not JSON, so a
// token with a malformed byte is never saved altered.
func TestRejectsATokenResponseThatIsNotValidUTF8(t *testing.T) {
	h := newHarness(t)
	s := newServer(t, func(s *server) {
		s.token = &reply{status: 200, body: "{\"token\":\"secret-\xff-token\"}"}
	})

	output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com"}, code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "The memry server did not return a token.")
	h.assertNoConfig()
}
