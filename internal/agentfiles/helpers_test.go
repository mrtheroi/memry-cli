package agentfiles_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// writeFile writes contents to path, creating its directories.
func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readFile returns the contents of path, failing the test when missing.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// exists reports whether path exists.
func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

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
