//go:build !windows

package agentfiles_test

import (
	"os"
	"syscall"
	"testing"
)

func inode(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Sys().(*syscall.Stat_t).Ino
}

// assertReplacedKeepingMode checks that path is a new file, not the one
// with inode before, and still has mode 0640.
func assertReplacedKeepingMode(t *testing.T, path string, before uint64) {
	t.Helper()
	if inode(t, path) == before {
		t.Error("the file was written in place, want a new file renamed over it")
	}
	info, _ := os.Stat(path)
	if mode := info.Mode().Perm(); mode != 0o640 {
		t.Errorf("mode = %o, want 0640", mode)
	}
}

// chmod0640 sets path to mode 0640 and returns its inode.
func chmod0640(t *testing.T, path string) uint64 {
	t.Helper()
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	return inode(t, path)
}
