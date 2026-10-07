package commands_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/commands"
)

// run runs the memry CLI with args and stdin, logged out (a temporary
// config path that does not exist), and returns its stdout, stderr and
// error.
func run(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("MEMRY_CONFIG", filepath.Join(dir, "config.json"))
	t.Setenv("SHELL_VERBOSITY", "")
	var out, errOut bytes.Buffer
	root := commands.NewRoot("v0.8.0")
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errOut.String(), err
}

const parseError = `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}` + "\n"

// `memry mcp` proxies stdin to the memry server.
func TestMcpProxiesStdin(t *testing.T) {
	out, _, err := run(t, "garbage\n", "mcp")

	if err != nil || out != parseError {
		t.Errorf("mcp = %q, %v; want the parse error, nil", out, err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

// `memry mcp` exits with code 1 when its output fails.
func TestMcpFailsWhenItsOutputFails(t *testing.T) {
	t.Setenv("MEMRY_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	root := commands.NewRoot("v0.8.0")
	root.SetIn(strings.NewReader("garbage\n"))
	root.SetOut(failingWriter{})
	root.SetArgs([]string{"mcp"})

	if err := root.Execute(); err == nil {
		t.Error("Execute() = nil, want an error for exit code 1")
	}
}

// errorBlock is how the PHP CLI renders an error, without its colors.
func errorBlock(message string) string {
	blank := strings.Repeat(" ", len(message)+4)
	return "\n" + blank + "\n  " + message + "  \n" + blank + "\n\n"
}

// Like the PHP CLI, the hidden commands take no arguments and only the
// global options, checked before they run: an error exits with code 1,
// shown on stdout unless --silent.
func TestHiddenCommandsRejectArgumentsLikeThePHPCLI(t *testing.T) {
	for _, command := range []string{"mcp", "hook:session-start", "mcp-headers"} {
		tests := []struct {
			args []string
			want string
		}{
			{[]string{"foo"}, errorBlock(`No arguments expected for "` + command + `" command, got "foo".`)},
			{[]string{"--x"}, errorBlock(`The "--x" option does not exist.`)},
			{[]string{"--quiet=1"}, errorBlock(`The "--quiet" option does not accept a value.`)},
			{[]string{"-q", "foo"}, errorBlock(`No arguments expected for "` + command + `" command, got "foo".`)},
			{[]string{"--silent", "foo"}, ""},
		}
		for _, tt := range tests {
			t.Run(command+" "+strings.Join(tt.args, " "), func(t *testing.T) {
				out, _, err := run(t, "garbage\n", append([]string{command}, tt.args...)...)

				if err == nil || out != tt.want {
					t.Errorf("output = %q, %v; want %q and an error", out, err, tt.want)
				}
			})
		}
	}
}

// The global options run the command, wherever they are; quiet does not
// hide the proxy's replies, which PHP writes straight to stdout.
func TestMcpAcceptsTheGlobalOptions(t *testing.T) {
	for _, args := range [][]string{{"mcp", "-q"}, {"mcp", "--env", "x"}, {"mcp", "-vvv", "--no-ansi"}, {"-q", "mcp"}, {"mcp", "--"}} {
		out, _, err := run(t, "garbage\n", args...)

		if err != nil || out != parseError {
			t.Errorf("%q = %q, %v; want the parse error, nil", args, out, err)
		}
	}
}

// Ported from the three "is hidden from the command list" tests.
func TestHiddenCommandsAreLeftOutOfTheCommandList(t *testing.T) {
	out, _, err := run(t, "", "--help")

	if err != nil {
		t.Fatalf("--help: %v", err)
	}
	for _, hidden := range []string{"mcp-headers", "hook:session-start", "Proxy MCP messages", "headersHelper", "SessionStart"} {
		if strings.Contains(out, hidden) {
			t.Errorf("the command list shows %q:\n%s", hidden, out)
		}
	}
	if !strings.Contains(out, "setup") {
		t.Errorf("the command list does not show setup:\n%s", out)
	}
}

// Like setup (and Symfony), --help and --version, even after other
// arguments, show the help or the version instead of running; quiet hides
// them.
func TestHiddenCommandsShowTheirHelpAndVersion(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"mcp", "--help"}, "Proxy MCP messages between stdio and the memry server"},
		{[]string{"hook:session-start", "foo", "-h"}, "Print the memry context of the current project (Claude Code SessionStart hook)"},
		{[]string{"mcp-headers", "-V"}, "Memry v0.8.0\n"},
		{[]string{"mcp", "--version"}, "Memry v0.8.0\n"},
		{[]string{"mcp", "-q", "--help"}, ""},
	}
	for _, tt := range tests {
		out, _, err := run(t, "garbage\n", tt.args...)

		if err != nil || !strings.Contains(out, tt.want) || (tt.want == "" && out != "") || strings.Contains(out, "Parse error") {
			t.Errorf("%q = %q, %v; want %q", tt.args, out, err, tt.want)
		}
	}
}

// logIn saves a login to a fake memry server answering every request with
// body.
func logIn(t *testing.T, body string) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"url":"`+s.URL+`","token":"secret-token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("MEMRY_CONFIG", path)
}

// executeIn runs the memry CLI in the environment as it is.
func executeIn(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	root := commands.NewRoot("v0.8.0")
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errOut.String(), err
}

// Like the PHP CLI, quiet or silent hides what the hook prints.
func TestSessionStartHookFollowsQuietMode(t *testing.T) {
	logIn(t, "the context")
	input := `{"cwd":"` + t.TempDir() + `"}`

	out, _, err := executeIn(t, input, "hook:session-start")
	if err != nil || !strings.HasSuffix(out, "\n\nthe context\n") {
		t.Fatalf("hook:session-start = %q, %v; want the context", out, err)
	}
	for _, quiet := range []string{"-q", "--silent"} {
		if out, _, err := executeIn(t, input, "hook:session-start", quiet); err != nil || out != "" {
			t.Errorf("hook:session-start %s = %q, %v; want nothing", quiet, out, err)
		}
	}
}

// `memry mcp-headers` prints the header on stdout and its error on
// stderr, exiting 1 without a login; quiet or silent hides both but keeps
// the exit code, as in the PHP CLI.
func TestMcpHeadersFollowsQuietMode(t *testing.T) {
	out, errOut, err := run(t, "", "mcp-headers")
	if err == nil || out != "" || errOut != "memry is not logged in. Run memry setup.\n" {
		t.Errorf("mcp-headers logged out = %q, %q, %v; want the error on stderr and exit 1", out, errOut, err)
	}
	for _, quiet := range []string{"-q", "--silent"} {
		if out, errOut, err := run(t, "", "mcp-headers", quiet); err == nil || out != "" || errOut != "" {
			t.Errorf("mcp-headers %s logged out = %q, %q, %v; want nothing and exit 1", quiet, out, errOut, err)
		}
	}

	logIn(t, "")
	if out, _, err := executeIn(t, "", "mcp-headers"); err != nil || out != `{"Authorization":"Bearer secret-token"}`+"\n" {
		t.Errorf("mcp-headers = %q, %v; want the header", out, err)
	}
	if out, errOut, err := executeIn(t, "", "mcp-headers", "-q"); err != nil || out != "" || errOut != "" {
		t.Errorf("mcp-headers -q = %q, %q, %v; want nothing and exit 0", out, errOut, err)
	}
}
