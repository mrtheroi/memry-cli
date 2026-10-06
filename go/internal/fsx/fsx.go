// Package fsx holds the file system helpers memry writes its files with.
package fsx

import (
	"os"
	"path/filepath"
)

// WriteAtomic writes data to a temporary file in the same directory and
// renames it over path, so a crash mid-write never leaves a truncated file.
// An existing file keeps its permissions; a new one gets mode. Missing
// parent directories are created readable by the owner only.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".")
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
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
