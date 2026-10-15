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

// SetLockWait sets how long a usage command waits for the lock, until
// the test ends.
func SetLockWait(wait time.Duration) (restore func()) {
	return config.SetLockWait(wait)
}

// LockPath is the usage lock env takes.
func LockPath(env Env) string {
	path, _ := config.Path(env.GOOS, env.getenv)
	return config.LockPath(env.CacheDir, path, env.GOOS)
}

// WrittenKeys are the variables enable writes, in order.
func WrittenKeys() []string {
	var keys []string
	for _, v := range append(wanted("https://memry.test/api/otlp/v1/metrics"), variable{headersKey, ""}) {
		keys = append(keys, v.key)
	}
	return keys
}
