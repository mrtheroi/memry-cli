package uninstall_test

import (
	"os"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// A config file that exists but cannot be read is not an empty config:
// it may hold the login and the agents. The commands fail without
// removing, revoking or sending anything. (The PHP CLI stops there too,
// with an uncaught ErrorException from file_get_contents.)

func unreadableConfig(t *testing.T, h *harness, s *server) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root can read a mode 000 file")
	}
	h.previousConfig(map[string]any{"url": s.URL, "token": "old-token", "agents": []string{"codex"}})
	if err := os.Chmod(h.configPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(h.configPath, 0o600) })
}

func TestUninstallFailsOnAConfigItCannotRead(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	codex := &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(&fakeAgent{key: "claude-code", name: "Claude Code"}, codex)
	unreadableConfig(t, h, s)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	if want := "Could not read " + h.configPath + ": permission denied.\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	assertCalls(t, codex)
	if len(s.received()) != 0 || !h.configExists() {
		t.Error("sent a request or deleted the config")
	}
}

func TestDeleteAccountFailsOnAConfigItCannotRead(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	unreadableConfig(t, h, s)

	output, code := h.run(uninstall.DeleteAccount, nil)

	assertExit(t, code, 1, output)
	if want := "Could not read " + h.configPath + ": permission denied.\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if len(s.received()) != 0 || !h.configExists() {
		t.Error("sent a request or deleted the config")
	}
}

// Like the PHP CLI (is_file), a directory at the config path is no config
// file: it is left alone, and uninstall goes on.
func TestUninstallTakesADirectoryAtTheConfigPathForNoConfig(t *testing.T) {
	h := newHarness(t)
	h.useAgents(&fakeAgent{key: "claude-code", name: "Claude Code"})
	if err := os.Mkdir(h.configPath, 0o700); err != nil {
		t.Fatal(err)
	}

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "No config file to delete.\n")
}
