package usage

import "time"

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
	beforeWait, beforeStale := lockWait, lockStale
	lockWait, lockStale = wait, stale
	return func() { lockWait, lockStale = beforeWait, beforeStale }
}

// LockPath is the usage lock env takes.
func LockPath(env Env) string {
	return env.lockPath()
}
