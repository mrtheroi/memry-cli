package main

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain runs the real memry CLI when a test re-executes this test
// binary with MEMRY_TEST_MAIN=1, so tests see its real stdout, stderr and
// exit code.
func TestMain(m *testing.M) {
	if os.Getenv("MEMRY_TEST_MAIN") == "1" {
		os.Args = append([]string{"memry"}, os.Args[1:]...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// memry is the memry CLI as a process, with a temporary HOME and config
// path (no file there unless the test writes one).
func memry(t testing.TB, configPath string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = []string{"MEMRY_TEST_MAIN=1", "HOME=" + filepath.Dir(configPath), "MEMRY_CONFIG=" + configPath, "PATH=" + os.Getenv("PATH")}
	return cmd
}

// runMemry runs the CLI with stdin and returns its exit code, stdout and
// stderr.
func runMemry(t *testing.T, configPath, stdin string, args ...string) (int, string, string) {
	t.Helper()
	cmd := memry(t, configPath, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running memry: %v", err)
	}
	return cmd.ProcessState.ExitCode(), out.String(), errOut.String()
}

// Ported from "writes nothing but JSON-RPC lines to the real stdout".
func TestMcpWritesOnlyJSONRPCLinesToStdout(t *testing.T) {
	stdin := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" + `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" + "garbage\n"

	code, out, _ := runMemry(t, filepath.Join(t.TempDir(), "config.json"), stdin, "mcp")

	want := `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Not logged in to memry. Run ` + "`memry setup`" + `."}}` + "\n" +
		`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}` + "\n"
	if code != 0 || out != want {
		t.Errorf("memry mcp = %d, %q; want 0, %q", code, out, want)
	}
}

// Ported from the mcp-headers tests, which run it as a real process the
// way Claude Code runs a headersHelper, to tell stdout from stderr.
func TestMcpHeadersAsAProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	code, out, errOut := runMemry(t, path, "", "mcp-headers")
	if code != 1 || out != "" || !strings.Contains(errOut, "memry is not logged in. Run memry setup.") {
		t.Errorf("logged out: %d, %q, %q; want 1, nothing on stdout, the error on stderr", code, out, errOut)
	}

	if err := os.WriteFile(path, []byte(`{"url":"https://memry.test","token":"secret-token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ = runMemry(t, path, "", "mcp-headers")
	if want := `{"Authorization":"Bearer secret-token"}` + "\n"; code != 0 || out != want {
		t.Errorf("logged in: %d, %q; want 0, %q", code, out, want)
	}
}

// The hook always exits 0, printing nothing when logged out.
func TestSessionStartHookAsAProcess(t *testing.T) {
	code, out, errOut := runMemry(t, filepath.Join(t.TempDir(), "config.json"), `{"cwd":"/"}`, "hook:session-start")

	if code != 0 || out != "" || errOut != "" {
		t.Errorf("hook:session-start = %d, %q, %q; want 0 and nothing", code, out, errOut)
	}
}

// BenchmarkMcpStartup measures `memry mcp` from process start to its
// first reply: it starts, reads a line and answers it (logged out, so
// without any network). The process is this test binary re-executed.
func BenchmarkMcpStartup(b *testing.B) {
	path := filepath.Join(b.TempDir(), "config.json")
	line := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n")
	for b.Loop() {
		cmd := memry(b, path, "mcp")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			b.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			b.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			b.Fatal(err)
		}
		if _, err := stdin.Write(line); err != nil {
			b.Fatal(err)
		}
		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			b.Fatal(err)
		}
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			b.Fatal(err)
		}
	}
}
