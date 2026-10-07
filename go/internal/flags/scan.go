// Package flags resolves the options of `memry setup` in one place.
package flags

import (
	"slices"
	"strings"
)

// Option is the state of one option on the command line: absent, present
// without a value, or present with a value (which may be empty).
type Option struct {
	Present  bool
	HasValue bool
	Value    string
}

// Args are the setup options found on the command line.
type Args struct {
	URL, Email, Token, Agents Option
	// NoInteraction is set by -n or --no-interaction.
	NoInteraction bool
	// Quiet is set by -q, --quiet or --silent.
	Quiet bool
	// Verbose is set by -v, -vv, -vvv or --verbose.
	Verbose bool
	Rest    []string
}

// Scan reads the setup options from argv the way the PHP CLI (Symfony
// Console) does: --name=value, or --name followed by its value when the
// next argument is empty or does not start with "-". The last occurrence
// wins. The interaction and verbosity options are found like Symfony does
// (see HasParameterOption). Any other argument, and everything from "--"
// on, is kept in Rest.
func Scan(argv []string) Args {
	var args Args
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			args.Rest = append(args.Rest, argv[i:]...)
			break
		}
		if IsGlobalOption(arg) {
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		option := args.option(name)
		if option == nil || !strings.HasPrefix(arg, "--") {
			args.Rest = append(args.Rest, arg)
			continue
		}
		if !hasValue && i+1 < len(argv) && (argv[i+1] == "" || argv[i+1][0] != '-') {
			i++
			value, hasValue = argv[i], true
		}
		*option = Option{Present: true, HasValue: hasValue, Value: value}
	}
	args.NoInteraction = HasParameterOption(argv, "--no-interaction", "-n")
	args.Quiet = HasParameterOption(argv, "--silent", "--quiet", "-q")
	args.Verbose = HasParameterOption(argv, "--verbose", "-v")
	return args
}

// GlobalOptions are the value-less Symfony global options setup accepts
// (--help and --version are handled before setup runs).
var GlobalOptions = []string{"--no-interaction", "--silent", "--quiet", "--verbose", "--ansi", "--no-ansi"}

// GlobalShortcuts are the shortcuts of those options: -n, -q and -v, which
// can be combined in one token ("-qn", "-vvv").
const GlobalShortcuts = "nqv"

// IsGlobalOption reports whether the whole token is a valid global option
// or set of global shortcuts. A partly valid token ("-qfoo", "--quiet=x")
// is not: setup rejects it like Symfony does.
func IsGlobalOption(token string) bool {
	if slices.Contains(GlobalOptions, token) {
		return true
	}
	if len(token) < 2 || token[0] != '-' || token[1] == '-' {
		return false
	}
	return strings.Trim(token[1:], GlobalShortcuts) == ""
}

// HasParameterOption reports whether argv has one of the options, like
// Symfony's ArgvInput::HasParameterOption with $onlyParams: a token that is
// the option, or starts with it ("-vvv" for -v, "--verbose=2" for
// --verbose), before any "--".
func HasParameterOption(argv []string, options ...string) bool {
	for _, token := range argv {
		if token == "--" {
			return false
		}
		for _, option := range options {
			leading := option
			if strings.HasPrefix(option, "--") {
				leading += "="
			}
			if token == option || strings.HasPrefix(token, leading) {
				return true
			}
		}
	}
	return false
}

func (a *Args) option(name string) *Option {
	switch name {
	case "url":
		return &a.URL
	case "email":
		return &a.Email
	case "token":
		return &a.Token
	case "agents":
		return &a.Agents
	}
	return nil
}
