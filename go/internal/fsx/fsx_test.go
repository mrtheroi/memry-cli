package fsx_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/fsx"
)

func inode(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Sys().(*syscall.Stat_t).Ino
}

// Ported from tests/Unit/AtomicFileTest.php.
func TestWriteAtomicReplacesTheFileKeepingItsMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	before := inode(t, path)

	if err := fsx.WriteAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}
	if inode(t, path) == before {
		t.Error("the file was written in place, want a new file renamed over it")
	}
	info, _ := os.Stat(path)
	if mode := info.Mode().Perm(); mode != 0o640 {
		t.Errorf("mode = %o, want 0640", mode)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "file.json" {
		t.Errorf("dir entries = %v, want only file.json", entries)
	}
}

func TestWriteAtomicCreatesANewFileWithTheModeInMissingOwnerOnlyDirectories(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "memry")
	path := filepath.Join(dir, "config.json")

	if err := fsx.WriteAtomic(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}
	dirInfo, _ := os.Stat(dir)
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Errorf("dir mode = %o, want 0700", mode)
	}
}
