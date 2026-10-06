package setup_test

import (
	"bytes"
	"encoding/json"
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
	"github.com/mrtheroi/memry-cli/internal/prompt"
	"github.com/mrtheroi/memry-cli/internal/setup"
)

// reply is a fake server answer; a nil *reply fails the connection.
type reply struct {
	status   int
	body     string
	location string
}

// request is a request the fake server received.
type request struct {
	Method, Path, Query string
	Header              http.Header
	Body                string
}

// server fakes the memry endpoints like the PHP tests' fakeServer():
// POST /api/auth/code, POST /api/auth/token (login), DELETE
// /api/auth/token (revoke) and GET /api/context.
type server struct {
	*httptest.Server
	code, token, revoke, context *reply

	mu       sync.Mutex
	requests []request
}

func defaultReplies() (code, token, revoke, context *reply) {
	return &reply{status: 202, body: `{"message":"If the email is valid, a login code has been sent."}`},
		&reply{status: 200, body: `{"token":"secret-token"}`},
		&reply{status: 204, body: ``},
		&reply{status: 200, body: ``}
}

func newServer(t *testing.T, configure ...func(*server)) *server {
	t.Helper()
	s := &server{}
	s.code, s.token, s.revoke, s.context = defaultReplies()
	for _, c := range configure {
		c(s)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.requests = append(s.requests, request{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), string(body)})
	s.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/memry") // for a server URL with a path
	var answer *reply
	switch {
	case path == "/api/auth/code":
		answer = s.code
	case path == "/api/auth/token" && r.Method == http.MethodDelete:
		answer = s.revoke
	case path == "/api/auth/token":
		answer = s.token
	case path == "/api/context":
		answer = s.context
	default:
		answer = &reply{status: 404, body: `{"message":"Not Found"}`}
	}
	if answer == nil {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if answer.location != "" {
		w.Header().Set("Location", answer.location)
	}
	w.WriteHeader(answer.status)
	_, _ = io.WriteString(w, answer.body)
}

func (s *server) received() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}

// sent returns the requests matching method and path.
func (s *server) sent(method, path string) []request {
	var found []request
	for _, r := range s.received() {
		if r.Method == method && r.Path == path {
			found = append(found, r)
		}
	}
	return found
}

// unreachableURL is the address of a server that is no longer listening.
func unreachableURL(t *testing.T) string {
	t.Helper()
	s := httptest.NewServer(http.NotFoundHandler())
	s.Close()
	return s.URL
}

// answer is one scripted prompt answer, like Pest's expectsQuestion.
type answer struct {
	label, value string
	aborted      bool
}

// prompter answers the questions in order and fails the test on any
// other question.
type prompter struct {
	t       *testing.T
	answers []answer
}

func (p *prompter) next(label string) answer {
	p.t.Helper()
	if len(p.answers) == 0 {
		p.t.Fatalf("unexpected question %q", label)
	}
	a := p.answers[0]
	p.answers = p.answers[1:]
	if a.label != label {
		p.t.Fatalf("asked %q, want %q", label, a.label)
	}
	return a
}

func (p *prompter) Ask(label string) (string, error) {
	a := p.next(label)
	if a.aborted {
		return "", prompt.ErrAborted
	}
	return a.value, nil
}

func (p *prompter) Secret(label string) string {
	return p.next(label).value
}

// fakeAgents is a registry of agents, some installed.
type fakeAgents struct {
	keys      []string
	installed map[string]bool
}

func (f fakeAgents) Keys() []string              { return f.keys }
func (f fakeAgents) IsInstalled(key string) bool { return f.installed[key] }

var defaultAgents = fakeAgents{keys: []string{"claude-code", "codex", "opencode", "antigravity", "windsurf"}}

// harness runs setup with a temporary config file.
type harness struct {
	t          *testing.T
	dir        string
	configPath string
	env        map[string]string
	agents     fakeAgents
	// processEnv makes setup read and change the process environment.
	processEnv bool
}

// useProcessEnv makes setup use the process environment, with h's
// variables set for the test.
func (h *harness) useProcessEnv(t *testing.T) {
	for key, value := range h.env {
		t.Setenv(key, value)
	}
	h.processEnv = true
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	return &harness{t: t, dir: dir, configPath: path, env: map[string]string{"MEMRY_CONFIG": path, "HOME": dir}, agents: defaultAgents}
}

// run runs setup with argv and the scripted answers, returning the output
// and the exit code. Every scripted answer must be asked.
func (h *harness) run(argv []string, answers ...answer) (string, int) {
	h.t.Helper()
	var out bytes.Buffer
	p := &prompter{t: h.t, answers: answers}
	lookupEnv := func(key string) (string, bool) {
		value, ok := h.env[key]
		return value, ok
	}
	unsetenv := func(key string) error {
		delete(h.env, key)
		return nil
	}
	if h.processEnv {
		lookupEnv, unsetenv = os.LookupEnv, os.Unsetenv
	}
	code := setup.Run(setup.Env{
		Args:      argv,
		LookupEnv: lookupEnv,
		Unsetenv:  unsetenv,
		Out:       &out,
		Prompter:  p,
		Agents:    h.agents,
		HTTP:      client.New("test", 5*time.Second),
	})
	if len(p.answers) > 0 {
		h.t.Errorf("questions never asked: %+v", p.answers)
	}
	return out.String(), code
}

// writeConfig writes the config file as a previous setup would have.
func (h *harness) writeConfig(values map[string]any) {
	h.t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(h.configPath, data, 0o600); err != nil {
		h.t.Fatal(err)
	}
}

// config returns the decoded config file.
func (h *harness) config() map[string]any {
	h.t.Helper()
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		h.t.Fatalf("reading the config: %v", err)
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		h.t.Fatalf("decoding the config %q: %v", data, err)
	}
	return values
}

func (h *harness) assertNoConfig() {
	h.t.Helper()
	if _, err := os.Stat(h.configPath); !os.IsNotExist(err) {
		h.t.Errorf("config file exists (%v), want none", err)
	}
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

var code123456 = answer{label: "Login code", value: "123456"}
