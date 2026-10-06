package client_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/client"
)

// Ported from "does not follow a redirect from the token check to a page
// that answers 200": a redirect to a login page must never look like a
// success.
func TestNeverFollowsARedirect(t *testing.T) {
	var stubHits atomic.Int32
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stubHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer stub.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, stub.URL+"/login", http.StatusFound)
	}))
	defer server.Close()

	resp, err := client.New("dev", 10*time.Second).Get(server.URL + "/api/context")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	if client.Succeeded(resp) {
		t.Error("Succeeded() = true for the redirect, want false")
	}
	if hits := stubHits.Load(); hits != 0 {
		t.Errorf("the redirect target was requested %d times, want 0", hits)
	}
}

func TestSucceededOnlyFor2xx(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{199, false}, {200, true}, {201, true}, {204, true}, {299, true},
		{301, false}, {302, false}, {304, false}, {401, false}, {422, false}, {500, false},
	}
	for _, tt := range tests {
		if got := client.Succeeded(&http.Response{StatusCode: tt.status}); got != tt.want {
			t.Errorf("Succeeded(%d) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestSendsTheMemryUserAgent(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.UserAgent()
	}))
	defer server.Close()

	resp, err := client.New("v1.2.3", 10*time.Second).Get(server.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = resp.Body.Close()

	if want := "memry/v1.2.3"; got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}
}

func TestGivesUpAfterTheTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	resp, err := client.New("dev", 50*time.Millisecond).Get(server.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("Get succeeded, want a timeout error")
	}
}
