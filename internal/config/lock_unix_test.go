//go:build !windows

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/config"
)

// A config reached through a symlinked directory is the same file as
// through the real one: one lock, whether the file exists yet or not.
func TestLockPathIsTheSameThroughASymlinkedDirectory(t *testing.T) {
	cache, real := t.TempDir(), t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	for _, exists := range []bool{false, true} {
		if exists {
			if err := os.WriteFile(filepath.Join(real, "config.json"), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
		}

		viaReal := config.LockPath(cache, filepath.Join(real, "config.json"), "linux")
		viaLink := config.LockPath(cache, filepath.Join(link, "config.json"), "linux")

		if viaReal != viaLink {
			t.Errorf("exists %v: lock paths %q and %q differ, want one", exists, viaReal, viaLink)
		}
	}
}
