package fsx

import "time"

// Each Set replaces a seam and returns what restores it.

func SetRestrictToOwner(f func(path string) error) (restore func()) {
	previous := restrictToOwner
	restrictToOwner = f
	return func() { restrictToOwner = previous }
}

func SetRename(f func(oldpath, newpath string) error) (restore func()) {
	previous := rename
	rename = f
	return func() { rename = previous }
}

func SetSleep(f func(time.Duration)) (restore func()) {
	previous := sleep
	sleep = f
	return func() { sleep = previous }
}

func SetRetryable(f func(error) bool) (restore func()) {
	previous := retryable
	retryable = f
	return func() { retryable = previous }
}

// HasRestrictToOwner reports whether writes restrict the temp file.
func HasRestrictToOwner() bool { return restrictToOwner != nil }

// Retryable is retryable.
func Retryable(err error) bool { return retryable(err) }
