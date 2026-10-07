package setup_test

import (
	"bytes"
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

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/executable"
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

// fakeAgent is an agent that records the calls it gets and succeeds
// unless told to fail, like the PHP tests' FakeAgent.
type fakeAgent struct {
	key, name    string
	notInstalled bool
	fails        bool
	calls        []string
}

func (f *fakeAgent) Key() string       { return f.key }
func (f *fakeAgent) Name() string      { return f.name }
func (f *fakeAgent) IsInstalled() bool { return !f.notInstalled }

func (f *fakeAgent) Install(url string) agents.Result {
	f.calls = append(f.calls, "install "+url)
	if f.fails {
		return agents.Result{Lines: []agents.Line{{Style: "error", Text: "Could not wire " + f.name + "."}}}
	}
	return agents.Result{Successful: true, Lines: []agents.Line{{Style: "info", Text: "Wired " + f.name + "."}}}
}

func (f *fakeAgent) Uninstall() agents.Result {
	f.calls = append(f.calls, "uninstall")
	return agents.Result{Successful: !f.fails, Lines: []agents.Line{{Style: "info", Text: "Unwired " + f.name + "."}}}
}

// defaultAgents are fake agents with the keys and names of the supported
// ones, all installed.
func defaultAgents() *agents.Supported {
	return agents.NewSupported(
		&fakeAgent{key: "claude-code", name: "Claude Code"},
		&fakeAgent{key: "codex", name: "Codex"},
		&fakeAgent{key: "opencode", name: "OpenCode"},
		&fakeAgent{key: "antigravity", name: "Antigravity"},
		&fakeAgent{key: "windsurf", name: "Windsurf"},
	)
}

// harness runs setup with a temporary config file.
type harness struct {
	t          *testing.T
	dir        string
	configPath string
	env        map[string]string
	agents     agents.Registry
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
	return &harness{t: t, dir: dir, configPath: path, env: map[string]string{"MEMRY_CONFIG": path, "HOME": dir}, agents: defaultAgents()}
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

// fakeClaude fakes the claude CLI for the real agents, like the PHP tests'
// fakeClaude(): `claude` is on the PATH unless missing, and each `claude
// mcp <subcommand>` succeeds unless results says otherwise. No real
// command runs.
type fakeClaude struct {
	missing bool
	results map[string]agents.RunResult
	// ran are the commands looked up ("lookpath <name>") and run, in order.
	ran [][]string
}

func (f *fakeClaude) Run(argv []string) agents.RunResult {
	f.ran = append(f.ran, argv)
	if result, ok := f.results[argv[2]]; ok {
		return result
	}
	return agents.RunResult{Started: true}
}

func (f *fakeClaude) lookPath(name string) (string, error) {
	f.ran = append(f.ran, []string{"lookpath", name})
	if name == "claude" && !f.missing {
		return "/fake/bin/claude", nil
	}
	return "", exec.ErrNotFound
}

// claudeRan returns the claude commands run, without the lookups.
func (f *fakeClaude) claudeRan() [][]string {
	var ran [][]string
	for _, argv := range f.ran {
		if argv[0] == "claude" {
			ran = append(ran, argv)
		}
	}
	return ran
}

// useRealAgents makes setup wire the supported agents, reading the
// harness environment, with claude faked. The running binary is
// /usr/local/bin/memry.
func (h *harness) useRealAgents() *fakeClaude {
	claude := &fakeClaude{results: map[string]agents.RunResult{}}
	getenv := func(name string) string { return h.env[name] }
	h.agents = agents.New(agents.Env{
		Getenv:     getenv,
		LookPath:   claude.lookPath,
		Runner:     claude,
		Executable: executable.Executable{Getenv: getenv, Self: "/usr/local/bin/memry"},
	})
	return claude
}
