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

// --ansi and --no-ansi are Symfony global options the PHP CLI accepts.
func TestAcceptsTheAnsiGlobalOptions(t *testing.T) {
	for _, option := range []string{"--ansi", "--no-ansi"} {
		h, s := newHarness(t), newServer(t)

		output, code := h.run([]string{"--url", s.URL, "--email", "ana@example.com", option}, code123456)

		assertExit(t, code, 0, output)
	}
}
