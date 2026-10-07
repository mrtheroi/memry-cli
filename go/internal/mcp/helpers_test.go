package mcp_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/mcp"
)

// request is a request the fake server received.
type request struct {
	Method, Path string
	Header       http.Header
	Body         string
}

// server fakes the memry MCP endpoint with handle, recording every request.
type server struct {
	*httptest.Server
	mu       sync.Mutex
	requests []request
}

func newServer(t *testing.T, handle func(w http.ResponseWriter, r request)) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		req := request{r.Method, r.URL.Path, r.Header.Clone(), string(body)}
		s.mu.Lock()
		s.requests = append(s.requests, req)
		s.mu.Unlock()
		handle(w, req)
	}))
	t.Cleanup(s.Close)
	return s
}

// respond answers every request with status and body.
func respond(status int, body string) func(http.ResponseWriter, request) {
	return func(w http.ResponseWriter, _ request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// harness runs the proxy with a temporary config file, logged in to url.
type harness struct {
	t          *testing.T
	url        string
	configPath string
}

func newHarness(t *testing.T, url string) *harness {
	t.Helper()
	h := &harness{t: t, url: url, configPath: filepath.Join(t.TempDir(), "config.json")}
	h.writeConfig(`{"url":"` + url + `","token":"secret-token"}`)
	return h
}

func (h *harness) writeConfig(contents string) {
	h.t.Helper()
	if err := os.WriteFile(h.configPath, []byte(contents), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

// run runs the proxy with lines on stdin, each ended by a newline, and
// returns its exit code and exact output.
func (h *harness) run(lines ...string) (int, string) {
	h.t.Helper()
	var out bytes.Buffer
	code := mcp.Proxy(h.env(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out))
	return code, out.String()
}

func assertOutput(t *testing.T, code int, output, want string) {
	t.Helper()
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if output != want {
		t.Errorf("output =\n%s\nwant\n%s", abbreviate(output), abbreviate(want))
	}
}

// sequence answers the requests in order, one handler each.
func sequence(handlers ...func(http.ResponseWriter, request)) func(http.ResponseWriter, request) {
	var mu sync.Mutex
	return func(w http.ResponseWriter, r request) {
		mu.Lock()
		handle := handlers[0]
		handlers = handlers[1:]
		mu.Unlock()
		handle(w, r)
	}
}

// dropConnection fails the request: the connection closes without an
// answer.
func dropConnection(w http.ResponseWriter, _ request) {
	if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
		_ = conn.Close()
	}
}

// abbreviate keeps a huge output readable in a failure message.
func abbreviate(s string) string {
	if len(s) <= 400 {
		return s
	}
	return fmt.Sprintf("%s…(%d bytes)…%s", s[:200], len(s), s[len(s)-200:])
}

// assertHeader checks that header has name once with value, or not at all
// when value is empty.
func assertHeader(t *testing.T, header http.Header, name, value string) {
	t.Helper()
	got := header.Values(name)
	if value == "" && len(got) != 0 {
		t.Errorf("%s = %q, want no header", name, got)
	}
	if value != "" && (len(got) != 1 || got[0] != value) {
		t.Errorf("%s = %q, want %q", name, got, value)
	}
}

// env is the proxy's environment with the temporary config.
func (h *harness) env(in io.Reader, out io.Writer) mcp.Env {
	vars := map[string]string{"MEMRY_CONFIG": h.configPath, "HOME": filepath.Dir(h.configPath)}
	return mcp.Env{
		LookupEnv: func(key string) (string, bool) {
			value, ok := vars[key]
			return value, ok
		},
		In:   in,
		Out:  out,
		HTTP: client.New("test", 5*time.Second),
	}
}

// unreachableURL is a server address nothing listens on.
const unreachableURL = "http://127.0.0.1:1"

// failingWriter fails every write, like a closed stdout.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
