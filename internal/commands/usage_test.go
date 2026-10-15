package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUsageCommandsRun(t *testing.T) {
	dir := isolate(t)

	out, _, err := executeIn(t, "", "usage", "status")
	if err != nil || out != "Usage metrics are off.\n" {
		t.Errorf("usage status = %q, %v; want off", out, err)
	}

	out, _, err = executeIn(t, "", "usage", "disable")
	if err != nil || out != "Usage metrics are not on; nothing to remove.\n" {
		t.Errorf("usage disable = %q, %v; want nothing to remove", out, err)
	}

	out, _, err = executeIn(t, "", "usage", "enable", "--force", "--insecure")
	if err == nil || !strings.Contains(out, "You are not logged in to memry.") {
		t.Errorf("usage enable = %q, %v; want a failure for the missing login", out, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "claude", "settings.json")); err == nil {
		t.Error("usage enable wrote the Claude Code settings without a login")
	}
}
