package setup_test

import "testing"

// errorBlock is how the PHP CLI renders an error, without its colors.
func errorBlock(message string) string {
	pad := "  "
	blank := ""
	for range len(message) + 4 {
		blank += " "
	}
	return "\n" + blank + "\n" + pad + message + pad + "\n" + blank + "\n\n"
}

// Like the PHP CLI (Symfony), setup rejects options it does not know and
// any argument, before asking anything or contacting the server.
func TestRejectsUnknownOptionsAndArgumentsBeforeLoggingIn(t *testing.T) {
	tests := []struct {
		argv []string
		want string
	}{
		{[]string{"--foo"}, `The "--foo" option does not exist.`},
		{[]string{"--foo=bar"}, `The "--foo" option does not exist.`},
		{[]string{"-x"}, `The "-x" option does not exist.`},
		{[]string{"extra"}, `No arguments expected for "setup" command, got "extra".`},
		{[]string{"--", "extra"}, `No arguments expected for "setup" command, got "extra".`},
		// Symfony checks every character of a shortcut set and rejects a
		// value on the value-less global options, even quiet ones.
		{[]string{"-qfoo"}, `The "-f" option does not exist.`},
		{[]string{"-nfoo"}, `The "-f" option does not exist.`},
		{[]string{"--quiet=foo"}, `The "--quiet" option does not accept a value.`},
		{[]string{"--no-interaction=1"}, `The "--no-interaction" option does not accept a value.`},
		{[]string{"--verbose=2"}, `The "--verbose" option does not accept a value.`},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			argv := append([]string{"--url", s.URL, "--email", "ana@example.com"}, tt.argv...)

			output, code := h.run(argv)

			assertExit(t, code, 1, output)
			if output != errorBlock(tt.want) {
				t.Errorf("output = %q, want %q", output, errorBlock(tt.want))
			}
			if got := s.received(); len(got) != 0 {
				t.Errorf("sent %d requests, want none", len(got))
			}
			h.assertNoConfig()
		})
	}
}

// Shortcut sets of global options stay valid, like in the PHP CLI.
func TestAcceptsGlobalShortcutSets(t *testing.T) {
	// -n turns interaction off, so the token login (which asks nothing) is used.
	for _, option := range []string{"-vvv", "-qn", "-nq"} {
		h, s := newHarness(t), newServer(t)

		output, code := h.run([]string{"--url", s.URL, "--token=admin-token", option})

		assertExit(t, code, 0, output)
	}
}

// --ansi and --no-ansi are Symfony global options the PHP CLI accepts.
func TestAcceptsTheAnsiGlobalOptions(t *testing.T) {
	for _, option := range []string{"--ansi", "--no-ansi"} {
		h, s := newHarness(t), newServer(t)

		output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com", option}, code123456)

		assertExit(t, code, 0, output)
	}
}

// Like Symfony, --quiet hides messages but not errors; only --silent (or
// SHELL_VERBOSITY=-2) hides the argument error too. A malformed silent
// token is not silent.
func TestShowsArgumentErrorsUnlessSilent(t *testing.T) {
	bogus := errorBlock(`The "--bogus" option does not exist.`)
	tests := []struct {
		name string
		argv []string
		env  map[string]string
		want string
	}{
		{"--silent", []string{"--silent", "--bogus"}, nil, ""},
		{"SHELL_VERBOSITY=-2", []string{"--bogus"}, map[string]string{"SHELL_VERBOSITY": "-2"}, ""},
		{"--quiet", []string{"--quiet", "--bogus"}, nil, bogus},
		{"-q", []string{"-q", "--bogus"}, nil, bogus},
		{"SHELL_VERBOSITY=-1", []string{"--bogus"}, map[string]string{"SHELL_VERBOSITY": "-1"}, bogus},
		{"--silent=x", []string{"--silent=x"}, nil, errorBlock(`The "--silent" option does not accept a value.`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s := newHarness(t), newServer(t)
			for key, value := range tt.env {
				h.env[key] = value
			}

			output, code := h.run(append([]string{"--url", s.URL, "--email", "ana@example.com"}, tt.argv...))

			assertExit(t, code, 1, output)
			if output != tt.want {
				t.Errorf("output = %q, want %q", output, tt.want)
			}
		})
	}
}

// Like the PHP CLI (Laravel Zero), setup accepts the global --env option,
// whose optional value may be the next argument.
func TestAcceptsTheEnvGlobalOption(t *testing.T) {
	for _, env := range [][]string{{"--env"}, {"--env=x"}, {"--env", "x"}} {
		h := newHarness(t)

		output, code := h.run(append([]string{"--url=ftp://memry.test", "--email", "ana@example.com"}, env...))

		assertExit(t, code, 1, output)
		assertContains(t, output, "Invalid server address given with --url.")
	}
}
