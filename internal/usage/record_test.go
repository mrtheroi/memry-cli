package usage_test

import (
	"encoding/json"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// A usage_metrics record that is there but not a valid record is unknown
// state, never "no record": every usage command fails closed.
func TestUsageCommandsFailClosedOnAMalformedRecord(t *testing.T) {
	for name, record := range map[string]string{
		"null":            `null`,
		"empty":           `{}`,
		"empty current":   `{"current":{}}`,
		"values a string": `{"current":{"server_url":"https://x","settings_path":"/s","values":"x","token_id":7}}`,
		"no token id":     `{"current":{"server_url":"https://x","settings_path":"/s","values":{"A":"1"}}}`,
	} {
		for command, run := range map[string]func(usage.Env) int{"enable": usage.Enable, "disable": usage.Disable, "status": usage.Status} {
			t.Run(name+" "+command, func(t *testing.T) {
				s := newServer(t)
				h := newHarness(t)
				var raw json.RawMessage = []byte(record)
				h.writeConfig(map[string]any{"url": s.URL, "token": "1|login-token", "usage_metrics": raw})

				output, code := h.run(run)

				assertExit(t, code, 1, output)
				assertContains(t, output, "Could not read the memry config "+h.configPath+", so memry cannot tell whether usage metrics are on. Fix it, then try again.")
			})
		}
	}
}
