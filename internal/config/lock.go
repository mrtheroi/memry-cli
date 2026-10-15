package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Timing of the config lock, replaced in the tests (SetLockWait).
var (
	// lockWait is how long a command waits for another one.
	lockWait = 15 * time.Second
	lockPoll = 50 * time.Millisecond
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
// to, named after the config's canonical path (see canonicalPath), so it
// works even where the config's own directory is read-only, every alias
// of one config shares one lock, and two configs never do.
func LockPath(cacheDir, configPath, goos string) string {
	sum := sha256.Sum256([]byte(canonicalPath(configPath, goos)))
	return filepath.Join(cacheDir, "memry", "memry-usage-"+hex.EncodeToString(sum[:8])+".lock")
}

// canonicalPath is the one name of the file at path whatever alias it is
// reached by: absolute, with its symlinks resolved (its directory's,
// while the file does not exist yet), and, on Windows, where paths are
// case-insensitive, in lower case.
func canonicalPath(path, goos string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	} else if dir, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		path = filepath.Join(dir, filepath.Base(path))
	}
	if goos == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

// Lock takes the lock of the memry config at configPath, the exclusive
// lock every command that reads, changes and saves the config holds
// (setup, uninstall and the usage commands), so none overwrites another's
// changes. It is a file created only when there is none (O_EXCL), in a
// directory only this user can write to, holding a random token that is
// its owner's, and the pid and time for people. It is never taken over,
// whatever its age, as its command may only be paused: after lockWait it
// returns ErrLocked. Anything at the lock path that is not a regular file
// is refused, never followed or removed. The returned function releases
// the lock, removing the file only while it still holds this token.
func Lock(cacheDir, configPath, goos string) (func(), error) {
	path := LockPath(cacheDir, configPath, goos)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(dir); err != nil || !private(info) {
		return nil, ErrNotPrivate
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(random)
	deadline := time.Now().Add(lockWait)
	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, err = fmt.Fprintf(file, "%s %s %s\n", token, strconv.Itoa(os.Getpid()), time.Now().UTC().Format(time.RFC3339))
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				_ = os.Remove(path)
				return nil, err
			}
			return func() { unlock(path, token) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		// Lstat: a symlink is never followed, and anything but a regular
		// file was not made by memry.
		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return nil, ErrNotRegular
		}
		if time.Now().After(deadline) {
			return nil, ErrLocked
		}
		time.Sleep(lockPoll)
	}
}

// unlock removes the lock at path while it still holds token: if someone
// replaced it (after deleting a lock they took for a crashed command's),
// it is theirs.
func unlock(path, token string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if owner, _, _ := strings.Cut(string(data), " "); owner == token {
		_ = os.Remove(path)
	}
}

// LockFailure says why Lock failed with err for the config at configPath.
func LockFailure(err error, cacheDir, configPath, goos string) string {
	path := LockPath(cacheDir, configPath, goos)
	switch {
	case errors.Is(err, ErrLocked):
		return "Another memry command is changing the memry config. If none is running (for example after a crash), delete " + path + " and try again."
	case errors.Is(err, ErrNotPrivate):
		return filepath.Dir(path) + " is writable by other users or not yours; memry does not keep its lock there. Make it yours with mode 0700, then try again."
	case errors.Is(err, ErrNotRegular):
		return path + " is not a regular file; memry does not touch it. Remove it, then try again."
	}
	return "Could not lock " + path + ": " + err.Error() + "."
}

// SetLockWait sets how long Lock waits for another command, until
// restore; for the tests.
func SetLockWait(wait time.Duration) (restore func()) {
	before := lockWait
	lockWait = wait
	return func() { lockWait = before }
}
