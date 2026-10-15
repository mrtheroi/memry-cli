package usage_test

import (
	"path/filepath"
	"testing"
)

// Detailed beta tracing exports logs and traces, which can hold prompts and
// code, once telemetry is on: memry never turns it on.
func TestEnableStopsOnDetailedBetaTracingInTheEnvironment(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.env["ENABLE_BETA_TRACING_DETAILED"] = "1"
	h.env["BETA_TRACING_ENDPOINT"] = "https://collector.example.com"

	enableBlocked(t, h, s, "ENABLE_BETA_TRACING_DETAILED is set in the environment.")
	enableBlocked(t, h, s, "BETA_TRACING_ENDPOINT is set in the environment.")
}

// Any telemetry variable that is not memry's export is in the way, by
// namespace, so variables a later Claude Code adds are covered too.
func TestEnableStopsOnAnyOtherTelemetryVariable(t *testing.T) {
	for _, key := range []string{
		"OTEL_LOG_USER_PROMPTS", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "OTEL_SOMETHING_NEW",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA", "ENABLE_ENHANCED_TELEMETRY_BETA", "CLAUDE_CODE_ENABLE_TELEMETRY_FOR_X",
		"CLAUDE_CODE_OTEL_HEADERS_HELPER_DEBOUNCE_MS", "ENABLE_BETA_TRACING_DETAILED", "BETA_TRACING_ENDPOINT",
	} {
		t.Run(key, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)
			writeRaw(t, h.settingsPath(), `{"env":{"`+key+`":"1"}}`)

			enableBlocked(t, h, s, key+" is set in "+h.settingsPath()+".")
		})
	}
}

// Managed settings follow the same rule: beta tracing set there exports
// logs and traces too.
func TestEnableStopsOnManagedBetaTracing(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	managed := filepath.Join(h.managedDir, "managed-settings.json")
	writeRaw(t, managed, `{"env":{"BETA_TRACING_ENDPOINT":"https://collector.example.com","ENABLE_BETA_TRACING_DETAILED":"1"}}`)

	enableBlocked(t, h, s, "BETA_TRACING_ENDPOINT is set in the managed settings "+managed+".")
}

// Repository identity is a gate memry keeps off: turned on in the launch
// environment, it would override memry's value.
func TestEnableStopsOnRepositoryAttributesInTheEnvironment(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.env["OTEL_METRICS_INCLUDE_REPOSITORY"] = "true"

	enableBlocked(t, h, s, "OTEL_METRICS_INCLUDE_REPOSITORY is set in the environment.")
}
