//go:build !windows

package usage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// Something at the lock path that is not a regular file was not made by
// memry: it is refused, never followed or removed.
func TestASymlinkAtTheLockPathIsRefused(t *testing.T) {
	h := newHarness(t)
	t.Cleanup(usage.SetLockWait(50 * time.Millisecond))
	victim := filepath.Join(t.TempDir(), "victim")
	writeRaw(t, victim, "keep me")
	lock := h.lockPath()
	if err := os.MkdirAll(filepath.Dir(lock), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, lock); err != nil {
		t.Fatal(err)
	}

	output, code := h.run(usage.Status)

	assertExit(t, code, 1, output)
	assertContains(t, output, lock+" is not a regular file; memry does not touch it. Remove it, then try again.")
	if info, err := os.Lstat(lock); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("lock path = %v, %v; want the symlink left in place", info, err)
	}
	if got := readText(t, victim); got != "keep me" {
		t.Errorf("victim = %q, want it untouched", got)
	}
}

// A lock directory others can write to could hold their lock or link:
// memry refuses it.
func TestAWritableByOthersLockDirectoryIsRefused(t *testing.T) {
	for _, mode := range []os.FileMode{0o770, 0o707} {
		h := newHarness(t)
		dir := filepath.Dir(h.lockPath())
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}

		output, code := h.run(usage.Status)

		assertExit(t, code, 1, output)
		assertContains(t, output, dir+" is writable by other users or not yours; memry does not keep its lock there. Make it yours with mode 0700, then try again.")
	}
}
