package agents_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

func TestClaudeSettingsPath(t *testing.T) {
	tests := map[string]struct {
		goos string
		vars map[string]string
		want string
	}{
		"config dir":          {"linux", map[string]string{"CLAUDE_CONFIG_DIR": "/c", "HOME": "/h"}, filepath.Join("/c", "settings.json")},
		"home":                {"linux", map[string]string{"HOME": "/h"}, filepath.Join("/h", ".claude", "settings.json")},
		"windows userprofile": {"windows", map[string]string{"HOME": "/decoy", "USERPROFILE": `C:\Users\ana`}, `C:\Users\ana` + string(filepath.Separator) + filepath.Join(".claude", "settings.json")},
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
