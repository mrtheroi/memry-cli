package flags

import (
	"slices"
	"strings"
)

// UnexpectedArgument returns the error Symfony gives for the first option
// in rest the command does not know or its first argument (memry's
// commands take none), if any. --ansi and --no-ansi are global options the
// PHP CLI accepts.
func UnexpectedArgument(command string, rest []string) (string, bool) {
	for i, token := range rest {
		switch {
		case token == "--":
			if i+1 < len(rest) {
				return noArguments(command, rest[i+1]), true
			}
			return "", false
		case IsGlobalOption(token):
			continue
		case strings.HasPrefix(token, "--"):
			name, _, _ := strings.Cut(token, "=")
			if slices.Contains(GlobalOptions, name) {
				return `The "` + name + `" option does not accept a value.`, true
			}
			return `The "` + name + `" option does not exist.`, true
		case len(token) > 1 && token[0] == '-':
			// Like Symfony, the first character of the set that is not a
			// global shortcut.
			unknown := strings.TrimLeft(token[1:], GlobalShortcuts)
			return `The "-` + unknown[:1] + `" option does not exist.`, true
		default:
			return noArguments(command, token), true
		}
	}
	return "", false
}

func noArguments(command, argument string) string {
	return `No arguments expected for "` + command + `" command, got "` + argument + `".`
}
