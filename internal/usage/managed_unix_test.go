//go:build !windows

package usage_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A managed-settings.d directory memry cannot list may hold the policy.
func TestEnableStopsWhenTheManagedDropInDirectoryCannotBeRead(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	dropIns := filepath.Join(h.managedDir, "managed-settings.d")
	if err := os.MkdirAll(dropIns, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dropIns, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dropIns, 0o700) })

	enableBlocked(t, h, s, "Could not read the managed settings "+dropIns+", so memry cannot determine where Claude Code sends metrics.")
}
