package uninstall_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/executable"
	"github.com/mrtheroi/memry-cli/internal/prompt"
	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// reply is a fake server answer; a nil *reply fails the connection.
type reply struct {
	status   int
	body     string
	location string
}

// request is a request the fake server received.
type request struct {
	Method, Path string
	Header       http.Header
	Body         string
}

// server fakes DELETE /api/auth/token (revoke) and DELETE /api/account,
// like the PHP tests' fakeServer() and fakeAccountDeletion(). Any other
// path answers 200, as a login page a redirect would lead to.
type server struct {
	*httptest.Server
	revoke, account *reply

	mu       sync.Mutex
	requests []request
}

func newServer(t *testing.T, configure ...func(*server)) *server {
	t.Helper()
	s := &server{revoke: &reply{status: 204}, account: &reply{status: 204}}
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
	s.requests = append(s.requests, request{r.Method, r.URL.Path, r.Header.Clone(), string(body)})
	s.mu.Unlock()
	var answer *reply
	switch r.URL.Path {
	case "/api/auth/token":
		answer = s.revoke
	case "/api/account":
		answer = s.account
	default:
		answer = &reply{status: 200, body: "<html>Log in</html>"}
	}
	if answer == nil {
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			_ = conn.Close()
		}
		return
	}
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

// fakeClaude fakes the claude CLI for the real agents, like the PHP tests'
// fakeClaude(). No real command runs.
type fakeClaude struct {
	missing bool
	results map[string]agents.RunResult
	ran     [][]string
}

func (f *fakeClaude) Run(argv []string) agents.RunResult {
	f.ran = append(f.ran, argv)
	if result, ok := f.results[argv[2]]; ok {
		return result
	}
	return agents.RunResult{Started: true}
}

func (f *fakeClaude) lookPath(name string) (string, error) {
	if name == "claude" && !f.missing {
		return "/fake/bin/claude", nil
	}
	return "", exec.ErrNotFound
}

// fakeAgent records the calls it gets and succeeds, like the PHP tests'
// FakeAgent.
type fakeAgent struct {
	key, name string
	fails     bool
	calls     []string
}

func (f *fakeAgent) Key() string       { return f.key }
func (f *fakeAgent) Name() string      { return f.name }
func (f *fakeAgent) IsInstalled() bool { return true }
func (f *fakeAgent) Install(url string) agents.Result {
	f.calls = append(f.calls, "install "+url)
	return agents.Result{Successful: true}
}
func (f *fakeAgent) Uninstall() agents.Result {
	f.calls = append(f.calls, "uninstall")
	if f.fails {
		return agents.Result{Lines: []agents.Line{{Style: "warn", Text: "Could not unwire " + f.name + "."}}}
	}
	return agents.Result{Successful: true, Lines: []agents.Line{{Style: "info", Text: "Unwired " + f.name + "."}}}
}

// harness runs the commands with a temporary home, config file and
// Claude Code settings, the real agents and a fake claude CLI.
type harness struct {
	t            *testing.T
	dir          string
	configPath   string
	settingsPath string
	env          map[string]string
	claude       *fakeClaude
	agents       agents.Registry
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	h := &harness{
		t:            t,
		dir:          dir,
		configPath:   filepath.Join(dir, "config.json"),
		settingsPath: filepath.Join(dir, "claude", "settings.json"),
		claude:       &fakeClaude{results: map[string]agents.RunResult{}},
	}
	h.env = map[string]string{"HOME": dir, "MEMRY_CONFIG": h.configPath, "CLAUDE_CONFIG_DIR": filepath.Dir(h.settingsPath)}
	getenv := func(name string) string { return h.env[name] }
	h.agents = agents.New(agents.Env{
		Getenv:     getenv,
		LookPath:   h.claude.lookPath,
		Runner:     h.claude,
		Executable: executable.Executable{Getenv: getenv, Self: "/usr/local/bin/memry"},
	})
	return h
}

// useAgents replaces the supported agents with the given fakes.
func (h *harness) useAgents(fakes ...*fakeAgent) {
	list := make([]agents.Agent, len(fakes))
	for i, fake := range fakes {
		list[i] = fake
	}
	h.agents = agents.NewSupported(list...)
}

// answer is one scripted prompt answer, like Pest's expectsQuestion and
// expectsConfirmation.
type answer struct {
	label, value string
	aborted      bool
}

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

func (p *prompter) Confirm(label string) (bool, error) {
	a := p.next(label)
	if a.aborted {
		return false, prompt.ErrAborted
	}
	return a.value == "yes", nil
}

// run runs command with argv and the scripted answers, returning the
// output and the exit code. Every scripted answer must be asked.
func (h *harness) run(command func(uninstall.Env) int, argv []string, answers ...answer) (string, int) {
	h.t.Helper()
	var out bytes.Buffer
	p := &prompter{t: h.t, answers: answers}
	code := command(uninstall.Env{
		Args: argv,
		LookupEnv: func(key string) (string, bool) {
			value, ok := h.env[key]
			return value, ok
		},
		Out:      &out,
		Prompter: p,
		Agents:   h.agents,
		HTTP:     client.New("test", 5*time.Second),
	})
	if len(p.answers) > 0 {
		h.t.Errorf("questions never asked: %+v", p.answers)
	}
	return out.String(), code
}

// previousConfig writes the config file as a previous setup would have.
func (h *harness) previousConfig(values map[string]any) {
	h.t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(h.configPath, data, 0o600); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) configExists() bool {
	_, err := os.Stat(h.configPath)
	return err == nil
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

var force = []string{"--force"}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertRan(t *testing.T, claude *fakeClaude, want []string) {
	t.Helper()
	for _, argv := range claude.ran {
		if reflect.DeepEqual(argv, want) {
			return
		}
	}
	t.Errorf("ran %q, want %q among them", claude.ran, want)
}

func assertCalls(t *testing.T, agent *fakeAgent, want ...string) {
	t.Helper()
	if len(agent.calls) != len(want) || (len(want) > 0 && !reflect.DeepEqual(agent.calls, want)) {
		t.Errorf("%s calls = %q, want %q", agent.key, agent.calls, want)
	}
}

// config returns the decoded config file.
func (h *harness) config() map[string]any {
	h.t.Helper()
	var values map[string]any
	if err := json.Unmarshal([]byte(readText(h.t, h.configPath)), &values); err != nil {
		h.t.Fatal(err)
	}
	return values
}
