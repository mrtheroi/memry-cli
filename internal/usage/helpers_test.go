package usage_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/usage"
)

// request is a request the fake server received.
type request struct {
	Method, Path, Authorization string
}

// server fakes POST /api/auth/usage-token, answering status with body.
type server struct {
	*httptest.Server
	status int
	body   string

	mu       sync.Mutex
	requests []request
}

func newServer(t *testing.T) *server {
	t.Helper()
	s := &server{status: 201, body: `{"token":"7|usage-secret"}`}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, request{r.Method, r.URL.Path, r.Header.Get("Authorization")})
		status, body := s.status, s.body
		s.mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// answer sets what the server answers from now on.
func (s *server) answer(status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.body = status, body
}

// harness runs the commands with a temporary home and config file; the
// Claude Code settings are under that home. Nothing real is touched.
type harness struct {
	t          *testing.T
	home       string
	configPath string
	env        map[string]string
	goos       string
	// via, when set, receives every request whatever its host, so a
	// remote server URL never reaches the network.
	via *server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	h := &harness{t: t, home: dir, configPath: filepath.Join(dir, "config.json")}
	h.env = map[string]string{"HOME": dir, "USERPROFILE": dir, "MEMRY_CONFIG": h.configPath}
	return h
}

// settingsPath is Claude Code's user settings under the temporary home.
func (h *harness) settingsPath() string {
	return filepath.Join(h.home, ".claude", "settings.json")
}

// login writes the config file of a login to url.
func (h *harness) login(url string) {
	h.t.Helper()
	h.writeConfig(map[string]any{"url": url, "token": "1|login-token", "agents": []string{"claude-code"}})
}

func (h *harness) writeConfig(values map[string]any) {
	h.t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		h.t.Fatal(err)
	}
	writeRaw(h.t, h.configPath, string(data))
}

func (h *harness) run(command func(usage.Env) int, argv ...string) (string, int) {
	h.t.Helper()
	var out bytes.Buffer
	code := command(usage.Env{
		GOOS: h.goos,
		Args: argv,
		LookupEnv: func(key string) (string, bool) {
			value, ok := h.env[key]
			return value, ok
		},
		Out:  &out,
		HTTP: h.client(),
	})
	return out.String(), code
}

// settingsEnv returns the decoded env object of the settings.
func (h *harness) settingsEnv() map[string]any {
	h.t.Helper()
	var settings map[string]any
	if err := json.Unmarshal([]byte(readText(h.t, h.settingsPath())), &settings); err != nil {
		h.t.Fatal(err)
	}
	env, _ := settings["env"].(map[string]any)
	return env
}

func writeRaw(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("output does not contain %q:\n%s", want, output)
	}
}

func assertNotContains(t *testing.T, output, unwanted string) {
	t.Helper()
	if strings.Contains(output, unwanted) {
		t.Errorf("output contains %q:\n%s", unwanted, output)
	}
}

func assertExit(t *testing.T, got, want int, output string) {
	t.Helper()
	if got != want {
		t.Errorf("exit code = %d, want %d; output:\n%s", got, want, output)
	}
}

// client is the HTTP client the commands use: the real one, sending every
// request to via when set.
func (h *harness) client() *http.Client {
	c := client.New("test", 5*time.Second)
	if h.via != nil {
		target, _ := url.Parse(h.via.URL)
		next := c.Transport
		c.Transport = roundTripper(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
			return next.RoundTrip(r)
		})
	}
	return c
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
