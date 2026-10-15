package agents_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

func TestClaudeSettingsPath(t *testing.T) {
	sep := string(filepath.Separator)
	tests := map[string]struct {
		goos string
		vars map[string]string
		want string
	}{
		// The directory is used as given, joined with the OS separator.
		"config dir":          {"linux", map[string]string{"CLAUDE_CONFIG_DIR": "/c", "HOME": "/h"}, "/c" + sep + "settings.json"},
		"home":                {"linux", map[string]string{"HOME": "/h"}, "/h" + sep + filepath.Join(".claude", "settings.json")},
		"windows userprofile": {"windows", map[string]string{"HOME": "/decoy", "USERPROFILE": `C:\Users\ana`}, `C:\Users\ana` + sep + filepath.Join(".claude", "settings.json")},
		"windows config dir":  {"windows", map[string]string{"CLAUDE_CONFIG_DIR": `C:\Users\ana\work`}, `C:\Users\ana\work` + sep + "settings.json"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := agents.ClaudeSettingsPath(tt.goos, func(key string) string { return tt.vars[key] })
			if got != tt.want {
				t.Errorf("ClaudeSettingsPath = %q, want %q", got, tt.want)
			}
		})
	}
}

// A relative CLAUDE_CONFIG_DIR is resolved against the working directory,
// so the path memry records means the same file from any other one.
func TestClaudeSettingsPathResolvesARelativeConfigDir(t *testing.T) {
	work := filepath.Join(t.TempDir(), "work")
	t.Cleanup(agents.SetGetwd(func() (string, error) { return work, nil }))

	got := agents.ClaudeSettingsPath("linux", func(key string) string {
		return map[string]string{"CLAUDE_CONFIG_DIR": "profile"}[key]
	})

	if want := filepath.Join(work, "profile") + string(filepath.Separator) + "settings.json"; got != want {
		t.Errorf("ClaudeSettingsPath = %q, want %q", got, want)
	}
}
