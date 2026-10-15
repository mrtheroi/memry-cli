package usage_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/config"
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

// A record that lost the headers it expects could not tell what to check
// or remove: it fails closed.
func TestARecordThatLostItsHeadersFailsClosed(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	var cfg map[string]any
	if err := json.Unmarshal([]byte(readText(t, h.configPath)), &cfg); err != nil {
		t.Fatal(err)
	}
	current := cfg["usage_metrics"].(map[string]any)["current"].(map[string]any)
	delete(current["values"].(map[string]any), "OTEL_EXPORTER_OTLP_METRICS_HEADERS")
	delete(current["expected"].(map[string]any), "OTEL_EXPORTER_OTLP_METRICS_HEADERS")
	h.writeConfig(cfg)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "cannot tell whether usage metrics are on")
	assertContains(t, readText(t, h.settingsPath()), "Authorization=Bearer")
}

// The endpoint memry expects is recorded exactly as it matched, so a kept
// endpoint spelled with an upper-case host is still memry's.
func TestStatusAcceptsAKeptEndpointSpelledWithAnUpperCaseHost(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	url := strings.Replace(s.URL, "127.0.0.1", "LOCALHOST", 1)
	h.login(url)
	writeRaw(t, h.settingsPath(), `{"env":{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT":"`+url+`/api/otlp/v1/metrics"}}`)
	if output, code := h.run(usage.Enable); code != 0 {
		t.Fatalf("enable failed:\n%s", output)
	}

	output, code := h.run(usage.Status)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are on, as far as memry can see: Claude Code sends its metrics to "+url+"/api/otlp/v1/metrics.")
}

// The variables enable writes are exactly the ones a record expects.
func TestEnableWritesTheVariablesARecordExpects(t *testing.T) {
	if got := usage.WrittenKeys(); !reflect.DeepEqual(got, config.UsageKeys) {
		t.Errorf("enable writes %v, records expect %v", got, config.UsageKeys)
	}
}
