//go:build !windows

package fsx_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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

// defaultMode is the mode a new file gets from the umask, like PHP's
// 0666 & ~umask().
func defaultMode(t *testing.T) os.FileMode {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	info, _ := os.Stat(path)
	return info.Mode().Perm()
}

// Ported from app/Support/AtomicFile.php: a new file gets the default
// permissions, 0666 minus the umask.
func TestReplaceCreatesANewFileWithTheDefaultMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "AGENTS.md")

	if err := fsx.Replace(path, []byte("rules")); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), defaultMode(t); got != want {
		t.Errorf("mode = %o, want %o", got, want)
	}
	if got, _ := os.ReadFile(path); string(got) != "rules" {
		t.Errorf("contents = %q, want %q", got, "rules")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("dir entries = %v, want only AGENTS.md", entries)
	}
}

// Ported from tests/Unit/AtomicFileTest.php, for Replace.
func TestReplaceRenamesOverAnExistingFileKeepingItsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	before := inode(t, path)

	if err := fsx.Replace(path, []byte("new")); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	info, _ := os.Stat(path)
	if mode := info.Mode().Perm(); mode != 0o640 {
		t.Errorf("mode = %o, want 0640", mode)
	}
	if inode(t, path) == before {
		t.Error("the file was written in place")
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}
}

// S1.5.d (pin, green on arrival: Unix behavior is unchanged): no
// owner-only restriction is applied, the 0600 mode is what protects the file.
func TestWriteAtomicOnUnixAppliesNoRestrictionAndKeepsMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if fsx.HasRestrictToOwner() {
		t.Error("restrictToOwner is set, want none on Unix")
	}
	if err := fsx.WriteAtomic(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	info, _ := os.Stat(path)
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 0600", mode)
	}
}

// S1.6.d (pin, green on arrival): Unix tries the rename once, even for
// the errno a locked file gives on Windows.
func TestWriteAtomicOnUnixRenamesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	attempts := 0
	t.Cleanup(fsx.SetRename(func(string, string) error {
		attempts++
		return syscall.Errno(32)
	}))
	t.Cleanup(fsx.SetSleep(func(time.Duration) { t.Error("slept, want a single attempt") }))

	if err := fsx.WriteAtomic(path, []byte("{}"), 0o600); err == nil {
		t.Fatal("WriteAtomic succeeded, want the rename error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}
