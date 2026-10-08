package setup_test

import (
	"os"
	"path/filepath"
	"testing"
)

const noHome = "Could not find your home directory; set USERPROFILE or HOME.\n"

func TestSetup_NoHomeExitsNonZeroWritesNothing(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(newClaude(), newCodex())
	delete(h.env, "MEMRY_CONFIG")
	delete(h.env, "HOME")

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "--agents=claude-code"})

	assertExit(t, code, 1, output)
	assertContains(t, output, noHome)
	if entries, err := os.ReadDir(h.dir); err != nil || len(entries) != 0 {
		t.Errorf("home dir has %v (%v), want nothing written", entries, err)
	}
}

func TestSetup_UserprofileAloneIsAHome(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	h.useAgents(newClaude(), newCodex())
	delete(h.env, "MEMRY_CONFIG")
	delete(h.env, "HOME")
	h.env["USERPROFILE"] = h.dir

	output, code := h.run([]string{"--url", s.URL, "--token=admin-token", "--agents=claude-code"})

	assertExit(t, code, 0, output)
	if _, err := os.Stat(filepath.Join(h.dir, ".config", "memry", "config.json")); err != nil {
		t.Errorf("config was not saved under USERPROFILE: %v", err)
	}
}
