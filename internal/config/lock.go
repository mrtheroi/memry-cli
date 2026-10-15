package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Timing of the config lock, replaced in the tests (SetLockTiming).
var (
	// lockWait is how long a command waits for another one.
	lockWait = 15 * time.Second
	// lockStale is how old a lock is when its command must have died: a
	// command holds it for seconds, its requests 10 at most each.
	lockStale = 2 * time.Minute
	lockPoll  = 50 * time.Millisecond
)

// Errors of Lock.
var (
	// ErrLocked means another command held the lock for too long.
	ErrLocked = errors.New("locked")
	// ErrNotPrivate means the lock directory is not this user's own:
	// others could plant a lock or a link in it.
	ErrNotPrivate = errors.New("not a private directory")
	// ErrNotRegular means something that is not a regular file, which
	// memry never makes, is at the lock path.
	ErrNotRegular = errors.New("not a regular file")
)

// LockPath is the lock of the memry config at configPath: in memry's
// directory of the user's cache directory, which other users cannot write
// to, named after the config's absolute path, so it works even where the
// config's own directory is read-only, and two configs never share one.
func LockPath(cacheDir, configPath string) string {
	if abs, err := filepath.Abs(configPath); err == nil {
		configPath = abs
	}
	sum := sha256.Sum256([]byte(configPath))
	return filepath.Join(cacheDir, "memry", "memry-usage-"+hex.EncodeToString(sum[:8])+".lock")
}

// Lock takes the lock of the memry config at configPath, the exclusive
// lock every command that reads, changes and saves the config holds
// (setup and the usage commands), so none overwrites another's changes.
// It is a file created only when there is none (O_EXCL), holding the pid
// and the time of its command, in a directory only this user can write
// to. A regular lock older than lockStale is taken over; anything else at
// the lock path is refused, never followed or removed. It waits up to
// lockWait for another command, then returns ErrLocked. The returned
// function releases it.
func Lock(cacheDir, configPath string) (func(), error) {
	path := LockPath(cacheDir, configPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(dir); err != nil || !private(info) {
		return nil, ErrNotPrivate
	}
	deadline := time.Now().Add(lockWait)
	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, err = fmt.Fprintf(file, "%s %s\n", strconv.Itoa(os.Getpid()), time.Now().UTC().Format(time.RFC3339))
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				_ = os.Remove(path)
				return nil, err
			}
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		// Lstat: a symlink is never followed, and anything but a regular
		// file was not made by memry, so it is never removed.
		info, err := os.Lstat(path)
		if err == nil && !info.Mode().IsRegular() {
			return nil, ErrNotRegular
		}
		if err == nil && time.Since(info.ModTime()) > lockStale {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, ErrLocked
		}
		time.Sleep(lockPoll)
	}
}

// LockFailure says why Lock failed with err for the config at configPath.
func LockFailure(err error, cacheDir, configPath string) string {
	path := LockPath(cacheDir, configPath)
	switch {
	case errors.Is(err, ErrLocked):
		return "Another memry command is changing the memry config; try again when it is done. If none is, delete " + path + "."
	case errors.Is(err, ErrNotPrivate):
		return filepath.Dir(path) + " is writable by other users or not yours; memry does not keep its lock there. Make it yours with mode 0700, then try again."
	case errors.Is(err, ErrNotRegular):
		return path + " is not a regular file; memry does not touch it. Remove it, then try again."
	}
	return "Could not lock " + path + ": " + err.Error() + "."
}

// SetLockTiming sets how long Lock waits for another command and when a
// lock is stale, until restore; for the tests.
func SetLockTiming(wait, stale time.Duration) (restore func()) {
	beforeWait, beforeStale := lockWait, lockStale
	lockWait, lockStale = wait, stale
	return func() { lockWait, lockStale = beforeWait, beforeStale }
}
