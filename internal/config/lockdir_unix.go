//go:build !windows

package config

import (
	"io/fs"
	"os"
	"syscall"
)

// private reports whether the lock directory, as Lstat describes it, is a
// real directory of this user that no one else can write to.
func private(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return info.IsDir() && ok && int(stat.Uid) == os.Getuid() && info.Mode().Perm()&0o022 == 0
}
