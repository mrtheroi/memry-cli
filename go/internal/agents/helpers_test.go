package agents_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/executable"
)

// exe is the MEMRY_EXECUTABLE the adapter tests run with, as the PHP
// tests configure memry.executable.
const exe = "/opt/homebrew/opt/memry/bin/memry"

// testEnv is a temporary home with its environment variables, the
// commands on a fake PATH and a runner that records what it runs. Nothing
// reads or writes the real home, and no real command runs.
type testEnv struct {
	home   string
	vars   map[string]string
	onPath []string
	runner *fakeRunner
	// looked are the commands looked up on the PATH, in order.
	looked []string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	home := t.TempDir()
	return &testEnv{
		home:   home,
		vars:   map[string]string{"HOME": home, "MEMRY_EXECUTABLE": exe},
		runner: &fakeRunner{},
	}
}

func (e *testEnv) env() agents.Env {
	getenv := func(name string) string { return e.vars[name] }
	return agents.Env{
		Getenv: getenv,
		LookPath: func(name string) (string, error) {
			e.looked = append(e.looked, name)
			e.runner.ran = append(e.runner.ran, []string{"lookpath", name})
			if slices.Contains(e.onPath, name) {
				return "/fake/bin/" + name, nil
			}
			return "", exec.ErrNotFound
		},
		Runner:     e.runner,
		Executable: executable.Executable{Getenv: getenv, Self: "/usr/local/bin/memry"},
	}
}

// agent returns the agent with key, as the registry builds it.
func (e *testEnv) agent(t *testing.T, key string) agents.Agent {
	t.Helper()
	found := agents.New(e.env()).Only([]string{key})
	if len(found) != 1 {
		t.Fatalf("no agent %q", key)
	}
	return found[0]
}

// path returns the path under the temporary home.
func (e *testEnv) path(parts ...string) string {
	return filepath.Join(append([]string{e.home}, parts...)...)
}

// fakeRunner records the commands it is asked to run, answering with the
// result set for their subcommand (argv[2], like `claude mcp <sub>`),
// success by default.
type fakeRunner struct {
	ran     [][]string
	results map[string]bool
}

func (r *fakeRunner) Run(argv []string) bool {
	r.ran = append(r.ran, argv)
	if ok, set := r.results[argv[2]]; set {
		return ok
	}
	return true
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func assertContents(t *testing.T, path, want string) {
	t.Helper()
	if got := readFile(t, path); got != want {
		t.Errorf("%s = %q, want %q", filepath.Base(path), got, want)
	}
}

func assertSuccessful(t *testing.T, result agents.Result, want bool) {
	t.Helper()
	if result.Successful != want {
		t.Errorf("Successful = %v, want %v (lines: %q)", result.Successful, want, result.Lines)
	}
}

// assertHasLine checks that result shows the line text in style.
func assertHasLine(t *testing.T, result agents.Result, style, text string) {
	t.Helper()
	if !slices.Contains(result.Lines, agents.Line{Style: style, Text: text}) {
		t.Errorf("lines %q do not contain [%s %q]", result.Lines, style, text)
	}
}

// assertLines checks that result shows exactly lines, each a style and a text.
func assertLines(t *testing.T, result agents.Result, lines ...[2]string) {
	t.Helper()
	want := []agents.Line{}
	for _, line := range lines {
		want = append(want, agents.Line{Style: line[0], Text: line[1]})
	}
	got := append([]agents.Line{}, result.Lines...)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}
