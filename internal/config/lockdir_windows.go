//go:build windows

package config

import "io/fs"

// private reports whether the lock directory, as Lstat describes it, is a
// real directory. On Windows the user's cache directory (LocalAppData) is
// already private to the user.
func private(info fs.FileInfo) bool {
	return info.IsDir()
}
