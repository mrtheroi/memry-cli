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

	if !runner.Run([]string{script(t, `touch "$1"; echo noise; echo noise >&2`), marker}) {
		t.Error("Run = false for a command exiting with 0")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("the command did not get its arguments: %v", err)
	}
	if runner.Run([]string{script(t, "exit 3")}) {
		t.Error("Run = true for a command exiting with 3")
	}
	if runner.Run([]string{filepath.Join(t.TempDir(), "missing")}) {
		t.Error("Run = true for a missing command")
	}
}

func TestExecRunnerStopsACommandAfterItsTimeout(t *testing.T) {
	runner := agents.ExecRunner{Timeout: 100 * time.Millisecond}
	start := time.Now()

	if runner.Run([]string{script(t, "exec sleep 10")}) {
		t.Error("Run = true for a command that timed out")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Run took %v, want it stopped after its timeout", elapsed)
	}
}
