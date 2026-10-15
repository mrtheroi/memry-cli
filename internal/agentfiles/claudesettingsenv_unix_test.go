//go:build !windows

package agentfiles_test

import (
	"os"
	"path/filepath"
	"testing"
)

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// The env may hold a bearer token: a settings file SetEnv creates is
// readable by its owner only.
func TestClaudeSettingsSetEnvCreatesTheFileOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude", "settings.json")

	setEnv(t, path, "TOKEN", "secret")

	if got := mode(t, path); got != 0o600 {
		t.Errorf("mode = %o, want 0600", got)
	}
}

// An existing file others can read is tightened to its owner's bits, so
// the token never stays readable by them; a stricter mode is kept.
func TestClaudeSettingsSetEnvTightensButNeverLoosensAnExistingMode(t *testing.T) {
	for before, want := range map[os.FileMode]os.FileMode{0o644: 0o600, 0o640: 0o600, 0o600: 0o600, 0o400: 0o400} {
		path := settingsPath(t)
		writeFile(t, path, `{}`)
		if err := os.Chmod(path, before); err != nil {
			t.Fatal(err)
		}

		setEnv(t, path, "TOKEN", "secret")

		if got := mode(t, path); got != want {
			t.Errorf("mode %o became %o, want %o", before, got, want)
		}
	}
}
