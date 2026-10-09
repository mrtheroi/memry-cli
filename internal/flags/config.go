package flags

import (
	"errors"
	"path/filepath"
	"strings"
)

// ExtractConfig takes the global --config option out of argv, as
// --config=<path> or --config <path>; the last one wins and "--" ends the
// options. The path is made absolute. A missing or empty value is an error.
func ExtractConfig(argv []string) (path string, given bool, rest []string, err error) {
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			rest = append(rest, argv[i:]...)
			break
		}
		value, found := strings.CutPrefix(arg, "--config=")
		if arg == "--config" {
			found, value = true, ""
			if i+1 < len(argv) && (argv[i+1] == "" || argv[i+1][0] != '-') {
				i++
				value = argv[i]
			}
		}
		if !found {
			rest = append(rest, arg)
			continue
		}
		if value == "" {
			return "", false, nil, errors.New("The --config option needs a value.")
		}
		path, given = value, true
	}
	if given {
		if path, err = filepath.Abs(path); err != nil {
			return "", false, nil, err
		}
	}
	return path, given, rest, nil
}
