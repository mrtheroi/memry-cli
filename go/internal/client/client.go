// Package client builds the one HTTP client memry talks to its server with.
package client

import (
	"net/http"
	"time"
)

// New returns the HTTP client. It never follows a redirect: the redirect
// itself is the response, so a login page answering 200 never makes a
// request look successful. Every request gets the memry/<version>
// User-Agent and gives up after timeout.
func New(version string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: userAgent{value: "memry/" + version, next: http.DefaultTransport},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Succeeded reports whether the response is a success: only a 2xx status
// is, never a redirect.
func Succeeded(resp *http.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

type userAgent struct {
	value string
	next  http.RoundTripper
}

func (t userAgent) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.value)
	return t.next.RoundTrip(req)
}
