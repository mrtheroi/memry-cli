package usage_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// Like the other commands, an option a command does not take is refused
// before anything is done.
func TestCommandsRefuseOptionsTheyDoNotTake(t *testing.T) {
	tests := map[string]struct {
		command func(usage.Env) int
		argv    []string
		message string
	}{
		"enable":  {usage.Enable, []string{"--bogus"}, `The "--bogus" option does not exist.`},
		"disable": {usage.Disable, []string{"--force"}, `The "--force" option does not exist.`},
		"status":  {usage.Status, []string{"extra"}, `No arguments expected for "usage status" command, got "extra".`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)

			output, code := h.run(tt.command, tt.argv...)

			assertExit(t, code, 1, output)
			assertContains(t, output, tt.message)
			if got := s.received(); len(got) != 0 {
				t.Errorf("requests = %+v, want none", got)
			}
		})
	}
}
