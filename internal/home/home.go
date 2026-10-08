// Package home resolves the user's home directory from an injected getenv.
package home

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoHome means no home directory could be derived from the environment.
var ErrNoHome = errors.New("home directory not found")

// NotFoundMessage is what commands tell the user when Dir fails.
const NotFoundMessage = "Could not find your home directory; set USERPROFILE or HOME."

// Dir returns the home directory for goos (the zero value means Unix): HOME,
// then USERPROFILE on Unix and the reverse on Windows, then LOCALAPPDATA two
// levels up. With none of them set it returns ErrNoHome.
func Dir(goos string, getenv func(string) string) (string, error) {
	keys := []string{"HOME", "USERPROFILE"}
	if goos == "windows" {
		keys = []string{"USERPROFILE", "HOME"}
	}
	for _, key := range keys {
		if dir := getenv(key); dir != "" {
			return dir, nil
		}
	}
	if dir, ok := up(up(strings.TrimRight(getenv("LOCALAPPDATA"), `\/`), true)); ok {
		return dir, nil
	}
	return "", ErrNoHome
}

// up drops the last path element, splitting on either separator so a Windows
// path is handled the same on every OS. It reports false when path has no
// parent to step up to, or when the previous step already failed.
func up(path string, ok bool) (string, bool) {
	i := strings.LastIndexAny(path, `\/`)
	if !ok || i <= 0 {
		return "", false
	}
	return path[:i], true
}

// Join appends elems to base. Unlike filepath.Join it does not clean base, so
// the bytes of a path built from $HOME stay what they were before.
func Join(base string, elems ...string) string {
	return base + string(os.PathSeparator) + filepath.Join(elems...)
}
