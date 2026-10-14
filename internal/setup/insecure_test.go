package setup_test

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// offlineTransport records every request and fails it, so a test can see
// that setup reached the request phase without touching any real host.
type offlineTransport struct {
	mu   sync.Mutex
	urls []string
}

func (o *offlineTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.urls = append(o.urls, r.URL.String())
	return nil, errors.New("offline")
}

func (o *offlineTransport) requested() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.urls...)
}

func offlineHarness(t *testing.T) (*harness, *offlineTransport) {
	t.Helper()
	h, transport := newHarness(t), &offlineTransport{}
	h.http = &http.Client{Transport: transport}
	return h, transport
}

// Setup sends the token or the login code, and later every MCP request
// carries the saved token, so a plain http:// server that is not this
// machine is refused before asking or sending anything.
func TestRefusesPlainHTTPToANonLoopbackServer(t *testing.T) {
	for _, argv := range [][]string{
		{"--url", "http://example.com", "--email", "ana@example.com"},
		{"--url=http://192.168.1.10:8000/", "--token=admin-token"},
	} {
		h, transport := offlineHarness(t)

		output, code := h.run(argv)

		assertExit(t, code, 1, output)
		if !strings.Contains(output, "Refusing to send credentials over plain HTTP to ") ||
			!strings.Contains(output, "Use an https:// URL, or pass --insecure to allow it (for example on a trusted LAN).") {
			t.Errorf("setup %q output = %q, want the plain HTTP refusal", argv, output)
		}
		if sent := transport.requested(); len(sent) != 0 {
			t.Errorf("setup %q sent %d requests, want none", argv, len(sent))
		}
		h.assertNoConfig()
	}
}

// Loopback servers never leave this machine, so plain http:// stays valid
// for them without --insecure.
func TestAcceptsPlainHTTPToALoopbackServer(t *testing.T) {
	port := newServer(t).Listener.Addr().(*net.TCPAddr).Port
	for _, host := range []string{"localhost", "LOCALHOST", "127.0.0.1", "127.1.2.3", "[::1]"} {
		h, transport := offlineHarness(t)

		output, code := h.run([]string{"--url", fmt.Sprintf("http://%s:%d", host, port), "--email", "ana@example.com"})

		assertExit(t, code, 1, output)
		if strings.Contains(output, "Refusing to send credentials") {
			t.Errorf("host %q was refused: %q", host, output)
		}
		if sent := transport.requested(); len(sent) != 1 {
			t.Errorf("host %q sent %d requests, want the login code request", host, len(sent))
		}
	}
}

// Only the whole host counts: names that merely start with, or contain, a
// loopback address or name still point somewhere else.
func TestTreatsLookalikeHostsAsNonLoopback(t *testing.T) {
	for _, host := range []string{"localhost.evil.com", "127.0.0.1.nip.io", "evil-localhost", "128.0.0.1", "[::2]", "0.0.0.0"} {
		h, transport := offlineHarness(t)

		output, code := h.run([]string{"--url", "http://" + host, "--email", "ana@example.com"})

		assertExit(t, code, 1, output)
		assertContains(t, output, "Refusing to send credentials over plain HTTP to "+host+".")
		if sent := transport.requested(); len(sent) != 0 {
			t.Errorf("host %q sent %d requests, want none", host, len(sent))
		}
	}
}

// --insecure lets a plain http:// server that is not this machine through:
// setup goes on to the request phase (which fails here, offline).
func TestInsecureAllowsPlainHTTPToANonLoopbackServer(t *testing.T) {
	for _, argv := range [][]string{
		{"--url", "http://example.com", "--email", "ana@example.com", "--insecure"},
		{"--insecure", "--url=http://example.com", "--token=admin-token"},
	} {
		h, transport := offlineHarness(t)

		output, code := h.run(argv)

		assertExit(t, code, 1, output)
		if want := "Could not reach the memry server at http://example.com.\n"; output != want {
			t.Errorf("setup %q output = %q, want %q", argv, output, want)
		}
		if sent := transport.requested(); len(sent) != 1 {
			t.Errorf("setup %q sent %d requests, want 1", argv, len(sent))
		}
	}
}

// --insecure is a flag, not an option with a value.
func TestInsecureDoesNotAcceptAValue(t *testing.T) {
	h, transport := offlineHarness(t)

	output, code := h.run([]string{"--url", "http://example.com", "--email", "ana@example.com", "--insecure=1"})

	assertExit(t, code, 1, output)
	if output != errorBlock(`The "--insecure" option does not accept a value.`) {
		t.Errorf("output = %q", output)
	}
	if sent := transport.requested(); len(sent) != 0 {
		t.Errorf("sent %d requests, want none", len(sent))
	}
}

// The URL setup ends up using is checked, whether it comes from --url or
// from MEMRY_URL.
func TestRefusesPlainHTTPFromMemryURL(t *testing.T) {
	h, transport := offlineHarness(t)
	h.env["MEMRY_URL"] = "http://example.com"

	output, code := h.run([]string{"--email", "ana@example.com"})

	assertExit(t, code, 1, output)
	assertContains(t, output, "Refusing to send credentials over plain HTTP to example.com.")
	if sent := transport.requested(); len(sent) != 0 {
		t.Errorf("sent %d requests, want none", len(sent))
	}

	h, transport = offlineHarness(t)
	h.env["MEMRY_URL"] = "http://example.com"
	output, code = h.run([]string{"--email", "ana@example.com", "--insecure"})
	assertExit(t, code, 1, output)
	if sent := transport.requested(); len(sent) != 1 {
		t.Errorf("with --insecure sent %d requests, want 1", len(sent))
	}
}
