//go:build windows

package agentfiles_test

import "testing"

// Windows has no inodes or 0640 mode; the tests that need them skip.
func chmod0640(t *testing.T, _ string) uint64 {
	t.Helper()
	t.Skip("POSIX inodes and permission bits; the Windows equivalent is covered in slice 4a")
	return 0
}

func assertReplacedKeepingMode(t *testing.T, _ string, _ uint64) {
	t.Helper()
}
