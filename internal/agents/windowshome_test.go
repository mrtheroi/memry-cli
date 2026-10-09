package agents_test

import (
	"os"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// On Windows the home is USERPROFILE, not the HOME Git Bash and MSYS set to
// a POSIX-style path.
func TestPaths_WindowsHomeIsUserprofile(t *testing.T) {
	tests := map[string]struct {
		key  string
		want []string
	}{
		"codex":       {"codex", []string{".codex", "config.toml"}},
		"opencode":    {"opencode", []string{".config", "opencode", "opencode.json"}},
		"antigravity": {"antigravity", []string{".gemini", "config", "mcp_config.json"}},
		"windsurf":    {"windsurf", []string{".codeium", "windsurf", "mcp_config.json"}},
		"claude-code": {"claude-code", []string{".claude", "settings.json"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t)
			decoy := t.TempDir()
			e.vars["HOME"], e.vars["USERPROFILE"] = decoy, e.home
			env := e.env()
			env.GOOS = "windows"

			agentsFor(env, tt.key).Install("https://memry.test")

			if !exists(t, e.path(tt.want...)) {
				t.Errorf("%s was not written under USERPROFILE", e.path(tt.want...))
			}
			if entries := readDir(t, decoy); len(entries) != 0 {
				t.Errorf("HOME holds %v, want nothing", entries)
			}
		})
	}
}

func agentsFor(env agents.Env, key string) agents.Agent {
	return agents.New(env).Only([]string{key})[0]
}

func readDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
