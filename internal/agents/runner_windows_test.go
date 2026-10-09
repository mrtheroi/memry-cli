//go:build windows

package agents_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// cmdScript writes a .cmd with body to a temp dir whose name has a space
// and puts that dir on the PATH, so the runner finds it as `fake`.
func cmdScript(t *testing.T, body string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "with space")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake.cmd"), []byte("@echo off\r\n"+body+"\r\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// S2.2.d
func TestRunner_CmdEchoesArgvIntactWithSpaces(t *testing.T) {
	cmdScript(t, "echo %1\r\necho %2\r\necho %3")
	runner := agents.ExecRunner{Timeout: 30 * time.Second}
	spaced := filepath.Join(t.TempDir(), "my dir", "memry.exe")

	got := runner.Run([]string{"fake", spaced, "a & b", `end\`})

	if !got.Succeeded() {
		t.Fatalf("Run = %+v, want success", got)
	}
	// %N keeps the quotes, so the & stays text; the trailing backslash
	// arrives doubled.
	want := "\"" + spaced + "\"\r\n\"a & b\"\r\n\"end\\\\\"\r\n"
	if got.Output != want {
		t.Errorf("output = %q, want %q", got.Output, want)
	}
}

func TestRunner_CmdRefusesAnArgumentCmdWouldExpand(t *testing.T) {
	cmdScript(t, "echo ran")
	runner := agents.ExecRunner{Timeout: 30 * time.Second}

	got := runner.Run([]string{"fake", "%COMSPEC%"})

	if got.Started || got.Output == "" || strings.Contains(got.Output, "ran") {
		t.Errorf("Run = %+v, want not started with a reason", got)
	}
}

func TestRunner_CmdReportsExitCode(t *testing.T) {
	cmdScript(t, "exit /b 3")
	runner := agents.ExecRunner{Timeout: 30 * time.Second}

	got := runner.Run([]string{"fake"})

	if !got.Started || got.ExitCode != 3 {
		t.Errorf("Run = %+v, want started with exit code 3", got)
	}
}
