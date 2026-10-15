package usage_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// The system directories of Claude Code's managed settings files, from
// https://code.claude.com/docs/en/managed-settings.
func TestManagedSettingsDir(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin":  "/Library/Application Support/ClaudeCode",
		"linux":   "/etc/claude-code",
		"windows": `C:\Program Files\ClaudeCode`,
	} {
		if got := usage.ManagedSettingsDir(goos); got != want {
			t.Errorf("ManagedSettingsDir(%q) = %q, want %q", goos, got, want)
		}
	}
}
