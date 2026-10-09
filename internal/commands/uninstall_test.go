package commands_test

import (
	"os"
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
	t.Setenv("USERPROFILE", dir)
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

	// Without the claude CLI the MCP server removal is skipped, which is
	// not a failure (the PHP CLI exits with 1).
	want := "Not logged in; no token to revoke.\n" +
		"Claude Code CLI not found; skipped removing the memry MCP server.\n" +
		"No memry SessionStart hook to remove.\n" +
		"No config file to delete.\n" +
		"Run `brew uninstall memry` to remove the CLI.\n"
	if err != nil || out != want {
		t.Errorf("uninstall --force = %q, %v; want %q and exit code 0", out, err, want)
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

func TestDeleteAccountRequiresALogin(t *testing.T) {
	isolate(t)

	out, _, err := executeIn(t, "", "delete-account")

	if err == nil || out != "You are not logged in to memry.\n" {
		t.Errorf("delete-account = %q, %v; want the login error and exit code 1", out, err)
	}
}

func TestDeleteAccountAsksForTheEmailAndIsListed(t *testing.T) {
	dir := isolate(t)
	writeConfig(t, filepath.Join(dir, "config.json"), `{"url":"http://127.0.0.1:1","token":"secret-token"}`)

	out, _, err := executeIn(t, "\n", "delete-account")

	want := "This permanently deletes your memry account and ALL its memories on the server. It cannot be undone.\n" +
		"\n Type your account email to confirm:\n > \nAborted; nothing was deleted.\n"
	if err == nil || out != want {
		t.Errorf("delete-account = %q, %v; want %q and exit code 1", out, err, want)
	}
	out, _, _ = executeIn(t, "", "--help")
	if !strings.Contains(out, "delete-account") || !strings.Contains(out, "Permanently delete your memry account and all its memories") {
		t.Errorf("the command list does not show delete-account:\n%s", out)
	}
}

func writeConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
