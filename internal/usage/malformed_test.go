package usage_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// A memry config that cannot be read as a JSON object may hold the usage
// metrics record: no usage command takes it for "no record".
func TestUsageCommandsFailClosedOnAMalformedConfig(t *testing.T) {
	for _, contents := range []string{`{"url":`, `null`, `[]`} {
		for name, command := range map[string]func(usage.Env) int{"enable": usage.Enable, "disable": usage.Disable, "status": usage.Status} {
			t.Run(name+" "+contents, func(t *testing.T) {
				s := newServer(t)
				h := newHarness(t)
				writeRaw(t, h.configPath, contents)

				output, code := h.run(command)

				assertExit(t, code, 1, output)
				assertContains(t, output, "Could not read the memry config "+h.configPath+", so memry cannot tell whether usage metrics are on. Fix it, then try again.")
				if got := s.received(); len(got) != 0 {
					t.Errorf("requests = %+v, want none", got)
				}
			})
		}
	}
}
