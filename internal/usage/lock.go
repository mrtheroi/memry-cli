package usage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Timing of the usage lock, replaced in the tests.
var (
	// lockWait is how long a usage command waits for another one.
	lockWait = 15 * time.Second
	// lockStale is how old a lock is when its command must have died: a
	// usage command takes seconds, its two requests 10 at most each.
	lockStale = 2 * time.Minute
	lockPoll  = 50 * time.Millisecond
)

// errLocked means another usage command held the lock for too long.
var errLocked = errors.New("locked")

// errNotPrivate means the lock directory is not this user's own: others
// could plant a lock or a link in it.
var errNotPrivate = errors.New("not a private directory")

// errNotRegular means something that is not a regular file, which memry
// never makes, is at the lock path.
var errNotRegular = errors.New("not a regular file")

// lock takes the usage lock at path, an exclusive lock between memry
// processes, in a directory only this user can write to: a file created only when there is none (O_EXCL), holding the
// pid and the time of the command that holds it. A lock older than
// lockStale is taken over. It waits up to lockWait for another command,
// then returns errLocked. The returned function releases it.
func lock(path string) (func(), error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(dir); err != nil || !private(info) {
		return nil, errNotPrivate
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
			return nil, errNotRegular
		}
		if err == nil && time.Since(info.ModTime()) > lockStale {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, errLocked
		}
		time.Sleep(lockPoll)
	}
}

// locked runs command holding the usage lock of the memry config, so
// two usage commands (or uninstalls) never read, issue, record, write or
// revoke at the same time.
func (env Env) locked(command func() int) int {
	if env.CacheDir == "" {
		env.line("Could not find your cache directory for the usage lock; set HOME (or XDG_CACHE_HOME), then try again.")
		return failure
	}
	path := env.lockPath()
	unlock, err := lock(path)
	switch {
	case errors.Is(err, errLocked):
		env.line("Another memry usage command is running; try again when it is done. If none is, delete " + path + ".")
		return failure
	case errors.Is(err, errNotPrivate):
		env.line(filepath.Dir(path) + " is writable by other users or not yours; memry does not keep its lock there. Make it yours with mode 0700, then try again.")
		return failure
	case errors.Is(err, errNotRegular):
		env.line(path + " is not a regular file; memry does not touch it. Remove it, then try again.")
		return failure
	case err != nil:
		env.line("Could not lock " + path + ": " + err.Error() + ".")
		return failure
	}
	defer unlock()
	return command()
}
