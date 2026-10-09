//go:build !windows

package fsx

// retryableError is false: a rename over a file in use succeeds on Unix.
func retryableError(error) bool { return false }
