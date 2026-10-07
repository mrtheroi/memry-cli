package flags_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/flags"
)

// Laravel Zero adds the global --env[=ENV] option: it is accepted with or
// without a value, and like any optional value it takes the next argument
// when that is empty or does not start with "-".
func TestUnexpectedArgumentAcceptsTheEnvOption(t *testing.T) {
	for _, rest := range [][]string{{"--env"}, {"--env=x"}, {"--env", "x"}, {"--env", ""}, {"--env", "-q"}} {
		if message, ok := flags.UnexpectedArgument("mcp", rest); ok {
			t.Errorf("UnexpectedArgument(%q) = %q, want none", rest, message)
		}
	}
	message, ok := flags.UnexpectedArgument("mcp", []string{"--env", "x", "foo"})
	if want := `No arguments expected for "mcp" command, got "foo".`; !ok || message != want {
		t.Errorf("UnexpectedArgument(--env x foo) = %q, %v; want %q", message, ok, want)
	}
}

// A command's own value-less options, like uninstall's --force, are
// accepted, but not with a value; the first wrong token wins.
func TestUnexpectedArgumentAcceptsTheCommandsValuelessOptions(t *testing.T) {
	if message, ok := flags.UnexpectedArgument("uninstall", []string{"-q", "--force", "--force"}, "--force"); ok {
		t.Errorf("UnexpectedArgument(--force) = %q, want none", message)
	}
	tests := []struct {
		rest []string
		want string
	}{
		{[]string{"--force=yes"}, `The "--force" option does not accept a value.`},
		{[]string{"--bogus", "--force=yes"}, `The "--bogus" option does not exist.`},
		{[]string{"--force", "x"}, `No arguments expected for "uninstall" command, got "x".`},
	}
	for _, tt := range tests {
		if message, ok := flags.UnexpectedArgument("uninstall", tt.rest, "--force"); !ok || message != tt.want {
			t.Errorf("UnexpectedArgument(%q) = %q, %v; want %q", tt.rest, message, ok, tt.want)
		}
	}
}
