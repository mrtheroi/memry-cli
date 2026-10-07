package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/hook"
)

// request is a request the fake server received.
type request struct {
	Method, Path, RawQuery string
	Header                 http.Header
}

// server fakes GET /api/context with handle, recording every request.
type server struct {
	*httptest.Server
	mu       sync.Mutex
	requests []request
}

func newServer(t *testing.T, handle func(http.ResponseWriter, *http.Request)) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, request{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone()})
		s.mu.Unlock()
		handle(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

// respond answers every request with status and body.
func respond(status int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// harness runs the hook with a temporary directory and config file.
type harness struct {
	t          *testing.T
	dir        string
	configPath string
	getwd      func() (string, error)
	git        func(context.Context, string) (string, bool)
	gitTimeout time.Duration
}

func newHarness(t *testing.T, url string) *harness {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, dir: dir, configPath: filepath.Join(dir, "config.json"), getwd: os.Getwd, git: hook.GitTopLevel, gitTimeout: 5 * time.Second}
	h.writeConfig(`{"url":"` + url + `","token":"secret-token"}`)
	return h
}

func (h *harness) writeConfig(contents string) {
	h.t.Helper()
	writeFile(h.t, h.configPath, contents)
}

// run runs the hook with the input JSON on stdin and returns its exit
// code and exact output.
func (h *harness) run(input map[string]any) (int, string) {
	h.t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		h.t.Fatal(err)
	}
	return h.runWith(bytes.NewReader(data))
}

func (h *harness) runWith(stdin io.Reader) (int, string) {
	h.t.Helper()
	var out bytes.Buffer
	vars := map[string]string{"MEMRY_CONFIG": h.configPath, "HOME": h.dir}
	code := hook.SessionStart(hook.Env{
		LookupEnv: func(key string) (string, bool) {
			value, ok := vars[key]
			return value, ok
		},
		In:          stdin,
		Out:         &out,
		HTTP:        client.New("test", 5*time.Second),
		Getwd:       h.getwd,
		GitTopLevel: h.git,
		GitTimeout:  h.gitTimeout,
	})
	return code, out.String()
}

// mkdir creates the directory path, with its parents.
func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// gitRepo creates a git repository at path.
func gitRepo(t *testing.T, path string) string {
	t.Helper()
	mkdir(t, path)
	if out, err := exec.Command("git", "-C", path, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return path
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertOutput(t *testing.T, code int, output, want string) {
	t.Helper()
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if output != want {
		t.Errorf("output =\n%s\nwant\n%s", output, want)
	}
}

func assertExit(t *testing.T, code int) {
	t.Helper()
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func assertPrefix(t *testing.T, output, prefix string) {
	t.Helper()
	if !strings.HasPrefix(output, prefix) {
		t.Errorf("output does not start with %q:\n%s", prefix, output)
	}
}

// dropConnection fails the request: the connection closes without an
// answer.
func dropConnection(w http.ResponseWriter, _ *http.Request) {
	if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
		_ = conn.Close()
	}
}
