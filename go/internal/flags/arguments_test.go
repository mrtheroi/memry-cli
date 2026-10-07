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
