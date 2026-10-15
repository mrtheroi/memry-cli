package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/config"
)

// A lock is never taken over for its age: a command that is paused (or a
// machine that slept) still holds it.
func TestLockNeverTakesOverAnOldLock(t *testing.T) {
	cache, configPath := t.TempDir(), filepath.Join(t.TempDir(), "config.json")
	t.Cleanup(config.SetLockWait(50 * time.Millisecond))
	path := config.LockPath(cache, configPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("their-token 1 2000-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	_, err := config.Lock(cache, configPath)

	if !errors.Is(err, config.ErrLocked) {
		t.Errorf("Lock = %v, want ErrLocked", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "their-token 1 2000-01-01T00:00:00Z\n" {
		t.Errorf("lock = %q, want the other command's kept", data)
	}
}

// Releasing a lock removes it only while it still holds this command's
// token: one someone else holds now (after deleting a lock they thought
// a crashed command's) stays.
func TestUnlockLeavesALockWithAnotherToken(t *testing.T) {
	cache, configPath := t.TempDir(), filepath.Join(t.TempDir(), "config.json")
	unlock, err := config.Lock(cache, configPath)
	if err != nil {
		t.Fatal(err)
	}
	path := config.LockPath(cache, configPath)
	if err := os.WriteFile(path, []byte("their-token 2 2026-10-15T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	unlock()

	if data, err := os.ReadFile(path); err != nil || string(data) != "their-token 2 2026-10-15T00:00:00Z\n" {
		t.Errorf("lock = %q, %v; want theirs kept", data, err)
	}
}

// Releasing its own lock removes it.
func TestUnlockRemovesItsOwnLock(t *testing.T) {
	cache, configPath := t.TempDir(), filepath.Join(t.TempDir(), "config.json")
	unlock, err := config.Lock(cache, configPath)
	if err != nil {
		t.Fatal(err)
	}

	unlock()

	if _, err := os.Lstat(config.LockPath(cache, configPath)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("lock stat = %v, want it removed", err)
	}
}

// The message for a held lock names it, for a lock a crash left behind.
func TestLockFailureNamesTheLock(t *testing.T) {
	cache, configPath := t.TempDir(), filepath.Join(t.TempDir(), "config.json")

	got := config.LockFailure(config.ErrLocked, cache, configPath)

	if want := "Another memry command is changing the memry config. If none is running (for example after a crash), delete " + config.LockPath(cache, configPath) + " and try again."; got != want {
		t.Errorf("LockFailure = %q, want %q", got, want)
	}
}
