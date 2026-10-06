// Package flags resolves the options of `memry setup` in one place.
package flags

import "strings"

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
	NoInteraction             bool
	Rest                      []string
}

// Scan reads the setup options from argv the way the PHP CLI (Symfony
// Console) does: --name=value, or --name followed by its value when the
// next argument is empty or does not start with "-". The last occurrence
// wins. Any other argument, and everything from "--" on, is kept in Rest.
func Scan(argv []string) Args {
	var args Args
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			args.Rest = append(args.Rest, argv[i:]...)
			break
		}
		if arg == "--no-interaction" || arg == "-n" {
			args.NoInteraction = true
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
	return args
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
