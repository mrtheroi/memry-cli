package flags

import (
	"slices"
	"strings"
)

// UnexpectedArgument returns the error Symfony gives for the first option
// in rest the command does not know or its first argument (memry's
// commands take none), if any. --ansi, --no-ansi and --env are global
// options the PHP CLI accepts; options are the command's own value-less
// options, such as uninstall's --force.
func UnexpectedArgument(command string, rest []string, options ...string) (string, bool) {
	for i := 0; i < len(rest); i++ {
		switch token := rest[i]; {
		case token == "--":
			if i+1 < len(rest) {
				return noArguments(command, rest[i+1]), true
			}
			return "", false
		case IsGlobalOption(token), strings.HasPrefix(token, "--env="), slices.Contains(options, token):
			continue
		case token == "--env":
			// Laravel Zero's --env[=ENV], whose optional value may be the
			// next argument.
			if i+1 < len(rest) && (rest[i+1] == "" || rest[i+1][0] != '-') {
				i++
			}
		case strings.HasPrefix(token, "--"):
			name, _, _ := strings.Cut(token, "=")
			if slices.Contains(GlobalOptions, name) || slices.Contains(options, name) {
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
