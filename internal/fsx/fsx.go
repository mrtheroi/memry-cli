// Package fsx holds the file system helpers memry writes its files with.
package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Seams, replaced in the tests.
var (
	// restrictToOwner leaves path accessible to its owner only; nil
	// where the file mode already does.
	restrictToOwner func(path string) error
	rename          = os.Rename
	sleep           = time.Sleep
	// retryable reports whether a rename error is transient.
	retryable = retryableError
)

// WriteAtomic writes data to a temporary file in the same directory and
// renames it over path, so a crash mid-write never leaves a truncated file.
// An existing file keeps its permissions; a new one gets mode. Missing
// parent directories are created readable by the owner only.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	return write(path, data, &mode, true)
}

// Replace is WriteAtomic for the files of other programs, like PHP's
// AtomicFile::write: an existing file keeps its permissions and a new one
// gets the default ones, 0666 minus the umask.
func Replace(path string, data []byte) error {
	return write(path, data, nil, false)
}

// write writes data atomically to path, with the mode of an existing file,
// else mode, else (when mode is nil) the default mode. With restrict, the
// file is left accessible to its owner only where restrictToOwner is set.
func write(path string, data []byte, mode *os.FileMode, restrict bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil {
		perm := info.Mode().Perm()
		mode = &perm
	} else if mode == nil {
		perm, err := defaultMode(dir)
		if err != nil {
			return err
		}
		mode = &perm
	}

	// Created readable by the owner only, so the data is never exposed
	// before the chmod.
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	// Once renamed, there is nothing left to remove.
	defer func() { _ = os.Remove(temp.Name()) }()

	if restrict && restrictToOwner != nil {
		if err := restrictToOwner(temp.Name()); err != nil {
			_ = temp.Close()
			return fmt.Errorf("restricting %s to its owner failed, nothing was written: %w", path, err)
		}
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), *mode); err != nil {
		return err
	}
	return renameRetry(temp.Name(), path)
}

// renameAttempts is how many times a rename is tried: with the pauses of
// 10, 20 ... 90 ms in between, about 450 ms in total.
const renameAttempts = 10

// renameRetry renames oldpath over newpath, trying again while the error
// is transient, like a file another program holds open on Windows.
func renameRetry(oldpath, newpath string) error {
	for attempt := 1; ; attempt++ {
		err := rename(oldpath, newpath)
		if err == nil || !retryable(err) {
			return err
		}
		if attempt == renameAttempts {
			return fmt.Errorf("%s is in use by another program; close it and try again: %w", newpath, err)
		}
		sleep(time.Duration(attempt) * 10 * time.Millisecond)
	}
}

// defaultMode is the mode a new file in dir gets, 0666 minus the umask,
// read from an empty probe file: setting the umask to read it would
// affect the whole process.
func defaultMode(dir string) (os.FileMode, error) {
	for {
		name := filepath.Join(dir, ".memry-mode."+strconv.FormatUint(rand.Uint64(), 36))
		probe, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		info, err := probe.Stat()
		_ = probe.Close()
		_ = os.Remove(name)
		if err != nil {
			return 0, err
		}
		return info.Mode().Perm(), nil
	}
}
