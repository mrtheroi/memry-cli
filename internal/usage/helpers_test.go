package usage_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/usage"
)

// request is a request the fake server received.
type request struct {
	Method, Path, Query, Authorization string
}

// server fakes the usage token endpoints of the memry server, keeping the
// usage tokens it issued: POST /api/auth/usage-token issues one (ids from
// 7) without revoking any, DELETE /api/auth/usage-tokens revokes them all,
// or with ?keep=<id> only the ones issued before it (422 when it is not one
// of them).
type server struct {
	*httptest.Server
	// issueStatus and issueBody, when issueStatus is set, are the answer
	// to POST instead.
	issueStatus int
	issueBody   string
	// revokeStatus, when set, is the answer to DELETE instead.
	revokeStatus int

	mu       sync.Mutex
	requests []request
	nextID   int
	active   map[int]bool
}

func newServer(t *testing.T) *server {
	t.Helper()
	s := &server{nextID: 7, active: map[int]bool{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")})
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/auth/usage-token" && s.issueStatus != 0:
		w.WriteHeader(s.issueStatus)
		_, _ = io.WriteString(w, s.issueBody)
	case r.Method == http.MethodPost && r.URL.Path == "/api/auth/usage-token":
		id := s.nextID
		s.nextID++
		s.active[id] = true
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"id":%d,"token":%q}`, id, usageToken(id))
	case r.Method == http.MethodDelete && r.URL.Path == "/api/auth/usage-tokens" && s.revokeStatus != 0:
		w.WriteHeader(s.revokeStatus)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/auth/usage-tokens":
		keep, hasKeep := 0, r.URL.Query().Has("keep")
		if hasKeep {
			keep, _ = strconv.Atoi(r.URL.Query().Get("keep"))
			if !s.active[keep] {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
		}
		for id := range s.active {
			if !hasKeep || id < keep {
				delete(s.active, id)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// usageToken is the plain text of the usage token id.
func usageToken(id int) string {
	return fmt.Sprintf("%d|usage-secret-%d", id, id)
}

func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// activeTokens are the ids of the usage tokens not revoked, in order.
func (s *server) activeTokens() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []int{}
	for id := range s.active {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// answer sets what the server answers to POST from now on.
func (s *server) answer(status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issueStatus, s.issueBody = status, body
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
	h := &harness{t: t, home: dir, configPath: filepath.Join(dir, "memry", "config.json")}
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

// writeSettingsEnv replaces the settings with just env.
func (h *harness) writeSettingsEnv(env map[string]any) {
	h.t.Helper()
	data, err := json.Marshal(map[string]any{"env": env})
	if err != nil {
		h.t.Fatal(err)
	}
	writeRaw(h.t, h.settingsPath(), string(data))
}

// answerRevoke sets what the server answers to DELETE from now on.
func (s *server) answerRevoke(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokeStatus = status
}
