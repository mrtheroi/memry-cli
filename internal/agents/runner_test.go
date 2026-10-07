package agents_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// script writes an executable shell script with body to a temp dir.
func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExecRunnerReportsWhetherTheCommandExitedSuccessfully(t *testing.T) {
	runner := agents.ExecRunner{Timeout: 10 * time.Second}
	marker := filepath.Join(t.TempDir(), "ran")

	if !runner.Run([]string{script(t, `touch "$1"; echo noise; echo noise >&2`), marker}).Succeeded() {
		t.Error("Run = false for a command exiting with 0")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("the command did not get its arguments: %v", err)
	}
	if runner.Run([]string{script(t, "exit 3")}).Succeeded() {
		t.Error("Run = true for a command exiting with 3")
	}
	if runner.Run([]string{filepath.Join(t.TempDir(), "missing")}).Succeeded() {
		t.Error("Run = true for a missing command")
	}
}

func TestExecRunnerStopsACommandAfterItsTimeout(t *testing.T) {
	runner := agents.ExecRunner{Timeout: 100 * time.Millisecond}
	start := time.Now()

	if runner.Run([]string{script(t, "exec sleep 10")}).Succeeded() {
		t.Error("Run = true for a command that timed out")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Run took %v, want it stopped after its timeout", elapsed)
	}
}

// Codex review of PR #34: the result tells a command that ran and failed
// from one that could not start or timed out, and keeps its output.
func TestExecRunnerResultTellsHowTheCommandWent(t *testing.T) {
	runner := agents.ExecRunner{Timeout: 10 * time.Second}
	tests := map[string]struct {
		argv []string
		want agents.RunResult
	}{
		"exits with 0": {[]string{script(t, "echo out; echo err >&2")}, agents.RunResult{Started: true, ExitCode: 0, Output: "out\nerr\n"}},
		"exits with 3": {[]string{script(t, "echo No MCP server found >&2; exit 3")}, agents.RunResult{Started: true, ExitCode: 3, Output: "No MCP server found\n"}},
		"cannot start": {[]string{filepath.Join(t.TempDir(), "missing")}, agents.RunResult{Started: false, ExitCode: -1}},
		"times out":    {[]string{script(t, "exec sleep 10")}, agents.RunResult{Started: true, ExitCode: -1, TimedOut: true}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := runner
			if name == "times out" {
				r.Timeout = 100 * time.Millisecond
			}
			if got := r.Run(tt.argv); got != tt.want {
				t.Errorf("Run = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExecRunnerCapsTheOutputItKeeps(t *testing.T) {
	runner := agents.ExecRunner{Timeout: 10 * time.Second}

	got := runner.Run([]string{script(t, "head -c 100000 /dev/zero | tr '\\0' x")})

	if !got.Succeeded() || len(got.Output) != 64*1024 {
		t.Errorf("Run = started %v, exit %d, %d bytes of output, want success with 65536", got.Started, got.ExitCode, len(got.Output))
	}
}
