// Package fsx holds the file system helpers memry writes its files with.
package fsx

import (
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
)

// WriteAtomic writes data to a temporary file in the same directory and
// renames it over path, so a crash mid-write never leaves a truncated file.
// An existing file keeps its permissions; a new one gets mode. Missing
// parent directories are created readable by the owner only.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	return write(path, data, &mode)
}

// Replace is WriteAtomic for the files of other programs, like PHP's
// AtomicFile::write: an existing file keeps its permissions and a new one
// gets the default ones, 0666 minus the umask.
func Replace(path string, data []byte) error {
	return write(path, data, nil)
}

// write writes data atomically to path, with the mode of an existing file,
// else mode, else (when mode is nil) the default mode.
func write(path string, data []byte, mode *os.FileMode) error {
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
	return os.Rename(temp.Name(), path)
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
