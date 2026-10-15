package usage

import (
	"time"

	"github.com/mrtheroi/memry-cli/internal/config"
)

// SetBeforeSettingsWrite makes enable call f right before it writes the
// settings, as a crash there would stop it, until the test ends.
func SetBeforeSettingsWrite(f func()) (restore func()) {
	before := beforeSettingsWrite
	beforeSettingsWrite = f
	return func() { beforeSettingsWrite = before }
}

// SetLockTiming sets how long a usage command waits for the lock and when
// a lock is stale, until the test ends.
func SetLockTiming(wait, stale time.Duration) (restore func()) {
	return config.SetLockTiming(wait, stale)
}

// LockPath is the usage lock env takes.
func LockPath(env Env) string {
	path, _ := config.Path(env.GOOS, env.getenv)
	return config.LockPath(env.CacheDir, path)
}
