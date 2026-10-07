package commands_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// isolate runs the CLI logged out in a temporary home, with an empty PATH
// so no real agent CLI (claude) can be found or run.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("MEMRY_CONFIG", filepath.Join(dir, "config.json"))
	t.Setenv("PATH", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	t.Setenv("CODEX_HOME", filepath.Join(dir, "codex"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("MEMRY_EXECUTABLE", "")
	t.Setenv("SHELL_VERBOSITY", "")
	return dir
}

func TestUninstallRemovesMemryFromThisMachine(t *testing.T) {
	isolate(t)

	out, _, err := executeIn(t, "", "uninstall", "--force")

	want := "Not logged in; no token to revoke.\n" +
		"Claude Code CLI not found; skipped removing the memry MCP server.\n" +
		"No memry SessionStart hook to remove.\n" +
		"No config file to delete.\n" +
		"Run `brew uninstall memry` to remove the CLI.\n"
	if err == nil || out != want {
		t.Errorf("uninstall --force = %q, %v; want %q and exit code 1", out, err, want)
	}
}

func TestUninstallAsksForConfirmation(t *testing.T) {
	isolate(t)

	out, _, err := executeIn(t, "no\n", "uninstall")

	want := "\n Remove memry from your agents and delete your login? (yes/no) [no]:\n > \nAborted; nothing was removed.\n"
	if err == nil || out != want {
		t.Errorf("uninstall = %q, %v; want %q and exit code 1", out, err, want)
	}
}

func TestUninstallShowsItsHelpAndIsListed(t *testing.T) {
	isolate(t)

	out, _, err := executeIn(t, "", "uninstall", "--help")
	if err != nil || !strings.Contains(out, "Remove memry from your agents and delete your login") || !strings.Contains(out, "--force") {
		t.Errorf("uninstall --help = %q, %v", out, err)
	}
	out, _, _ = executeIn(t, "", "--help")
	if !strings.Contains(out, "uninstall") {
		t.Errorf("the command list does not show uninstall:\n%s", out)
	}
}
