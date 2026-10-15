package usage_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

func TestDisableRemovesOnlyWhatEnableWrote(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"model":"opus","env":{"FOO":"bar","OTEL_SERVICE_NAME":"mine"}}`)
	h.run(usage.Enable)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are off. Removed them from "+h.settingsPath()+"; restart Claude Code to apply it.")
	want := map[string]any{"FOO": "bar", "OTEL_SERVICE_NAME": "mine"}
	if got := h.settingsEnv(); !reflect.DeepEqual(got, want) {
		t.Errorf("env = %v, want %v", got, want)
	}
	assertContains(t, readText(t, h.settingsPath()), `"model": "opus"`)
}

// Telemetry the user set up themselves is not memry's, even with the
// values memry would write: it stays.
func TestDisableLeavesTelemetryMemryDidNotSetUp(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	settings := `{"env":{"CLAUDE_CODE_ENABLE_TELEMETRY":"1","OTEL_METRICS_EXPORTER":"otlp","OTEL_EXPORTER_OTLP_METRICS_ENDPOINT":"https://collector.example.com/v1/metrics"}}`
	writeRaw(t, h.settingsPath(), settings)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are not on; nothing to remove.")
	if got := readText(t, h.settingsPath()); got != settings {
		t.Errorf("settings = %s, want them untouched", got)
	}
}

// A variable that already had memry's value before enable, as for a user
// exporting logs elsewhere, was not written by memry: disable keeps it.
func TestDisableKeepsWhatTheUserHadSetBeforeEnable(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"env":{"CLAUDE_CODE_ENABLE_TELEMETRY":"1","OTEL_LOGS_EXPORTER":"console"}}`)
	h.run(usage.Enable)
	h.run(usage.Enable) // Enabling again does not take it for memry's.

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	want := map[string]any{"CLAUDE_CODE_ENABLE_TELEMETRY": "1", "OTEL_LOGS_EXPORTER": "console"}
	if got := h.settingsEnv(); !reflect.DeepEqual(got, want) {
		t.Errorf("env = %v, want %v", got, want)
	}
}

func TestDisableForgetsWhatEnableSavedKeepingTheLogin(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)

	h.run(usage.Disable)

	var cfg map[string]any
	if err := json.Unmarshal([]byte(readText(t, h.configPath)), &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["usage_metrics"]; ok || cfg["token"] != "1|login-token" {
		t.Errorf("config = %v, want the login without usage_metrics", cfg)
	}
}

// After a setup to another server, memry still knows the export it wrote.
func TestDisableRecognizesTheEndpointEnableWroteAfterTheServerChanged(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	var cfg map[string]any
	_ = json.Unmarshal([]byte(readText(t, h.configPath)), &cfg)
	cfg["url"] = "https://other.example.com"
	h.writeConfig(cfg)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	if got := h.settingsEnv(); len(got) != 0 {
		t.Errorf("env = %v, want memry's variables removed", got)
	}
}

// Disable revokes the usage tokens once memry's variables are gone.
func TestDisableRevokesTheUsageTokens(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	if got := s.activeTokens(); len(got) != 0 {
		t.Errorf("active tokens = %v, want none", got)
	}
	if got := s.received(); got[len(got)-1] != (request{"DELETE", "/api/auth/usage-tokens", "", "Bearer 1|login-token"}) {
		t.Errorf("last request = %+v, want DELETE /api/auth/usage-tokens without keep", got[len(got)-1])
	}
}

func TestDisableSucceedsWithAWarningWhenTheTokensCannotBeRevoked(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	s.answerRevoke(401)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Could not revoke your usage tokens; they stop working after 30 days without use.")
	assertContains(t, output, "Usage metrics are off.")
	assertNotContains(t, readText(t, h.configPath), `"usage_metrics"`)
}
