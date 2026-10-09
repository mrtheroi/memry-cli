package fsx_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/fsx"
)

// S1.5.a: the owner-only restriction is applied to the temp file before
// any data is written, on every write.
func TestWriteAtomicRestrictsTheTempFileBeforeWritingEveryTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var sizes []int64
	t.Cleanup(fsx.SetRestrictToOwner(func(temp string) error {
		info, err := os.Stat(temp)
		if err != nil {
			return err
		}
		sizes = append(sizes, info.Size())
		return nil
	}))

	for _, data := range []string{"first", "second"} {
		if err := fsx.WriteAtomic(path, []byte(data), 0o600); err != nil {
			t.Fatalf("WriteAtomic: %v", err)
		}
	}

	if len(sizes) != 2 || sizes[0] != 0 || sizes[1] != 0 {
		t.Errorf("temp sizes when restricted = %v, want [0 0]", sizes)
	}
}

// The files of other programs (Claude settings, AGENTS.md, ...) keep the
// access they inherit: only memry's own config is restricted to its owner.
func TestReplaceDoesNotRestrictTheFilesOfOtherPrograms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	called := false
	t.Cleanup(fsx.SetRestrictToOwner(func(string) error {
		called = true
		return nil
	}))

	if err := fsx.Replace(path, []byte("{}")); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	if called {
		t.Error("Replace restricted the file to its owner, want its inherited access kept")
	}
}

// S1.5.b: failing closed, nothing is written and the original is intact.
func TestWriteAtomicFailsClosedWhenTheRestrictionFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	denied := errors.New("denied")
	t.Cleanup(fsx.SetRestrictToOwner(func(string) error { return denied }))

	err := fsx.WriteAtomic(path, []byte("token"), 0o600)

	if !errors.Is(err, denied) {
		t.Fatalf("err = %v, want it to wrap %v", err, denied)
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "nothing was written") {
		t.Errorf("err = %q, want it to name %s and say nothing was written", err, path)
	}
	if got, _ := os.ReadFile(path); string(got) != "original" {
		t.Errorf("contents = %q, want the original", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("dir entries = %v, want only config.json", entries)
	}
}

var errBusy = errors.New("busy")

// fakeRename fails with errBusy `failures` times (forever when negative),
// then renames for real; it counts the attempts and the total sleep.
type fakeRename struct {
	failures int
	attempts int
	slept    time.Duration
}

func (f *fakeRename) install(t *testing.T, retry func(error) bool) {
	t.Helper()
	t.Cleanup(fsx.SetRename(func(oldpath, newpath string) error {
		f.attempts++
		if f.failures < 0 || f.attempts <= f.failures {
			return errBusy
		}
		return os.Rename(oldpath, newpath)
	}))
	t.Cleanup(fsx.SetSleep(func(d time.Duration) { f.slept += d }))
	t.Cleanup(fsx.SetRetryable(retry))
}

func isBusy(err error) bool { return errors.Is(err, errBusy) }

// S1.6.a
func TestWriteAtomicRetriesTheRenameUntilItSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	f := &fakeRename{failures: 2}
	f.install(t, isBusy)

	if err := fsx.WriteAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	if f.attempts != 3 {
		t.Errorf("attempts = %d, want 3", f.attempts)
	}
	if f.slept != 30*time.Millisecond {
		t.Errorf("slept = %v, want 30ms (10ms + 20ms)", f.slept)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("contents = %q, want new", got)
	}
}

// S1.6.b
func TestWriteAtomicGivesUpAfterTenAttemptsWithAnInUseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	f := &fakeRename{failures: -1}
	f.install(t, isBusy)

	err := fsx.WriteAtomic(path, []byte("new"), 0o600)

	if !errors.Is(err, errBusy) {
		t.Fatalf("err = %v, want it to wrap %v", err, errBusy)
	}
	want := path + " is in use by another program; close it and try again"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("err = %q, want it to contain %q", err, want)
	}
	if f.attempts != 10 {
		t.Errorf("attempts = %d, want 10", f.attempts)
	}
	if f.slept != 450*time.Millisecond {
		t.Errorf("slept = %v, want 450ms", f.slept)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("dir entries = %v, want the temp file removed", entries)
	}
}

// S1.6.c
func TestWriteAtomicDoesNotRetryAnErrorThatIsNotTransient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	f := &fakeRename{failures: -1}
	f.install(t, func(error) bool { return false })

	err := fsx.WriteAtomic(path, []byte("new"), 0o600)

	if !errors.Is(err, errBusy) {
		t.Fatalf("err = %v, want it to wrap %v", err, errBusy)
	}
	if f.attempts != 1 || f.slept != 0 {
		t.Errorf("attempts = %d, slept = %v, want 1 and 0", f.attempts, f.slept)
	}
}
