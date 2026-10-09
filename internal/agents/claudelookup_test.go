package agents_test

import (
	"os/exec"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// Pins (already green): on Windows exec.LookPath("claude") applies PATHEXT,
// so claude.exe and claude.cmd are both found through the one lookup.
func TestClaudeCodeIsInstalled_FoundThroughPATHEXT(t *testing.T) {
	tests := map[string]struct {
		found string
		want  bool
	}{
		"only claude.exe": {`C:\bin\claude.exe`, true},
		"only claude.cmd": {`C:\bin\claude.cmd`, true},
		"neither":         {"", false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t)
			env := e.env()
			env.LookPath = func(command string) (string, error) {
				if command != "claude" || tt.found == "" {
					return "", exec.ErrNotFound
				}
				return tt.found, nil
			}

			got := agents.New(env).Only([]string{"claude-code"})[0].IsInstalled()

			if got != tt.want {
				t.Errorf("IsInstalled = %v, want %v", got, tt.want)
			}
		})
	}
}
