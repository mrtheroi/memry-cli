package usage_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// A telemetry setting the user chose is never replaced silently: enable
// stops before asking for a token, and says which settings are in the way.
func TestEnableStopsOnAUserMetricsExporter(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	settings := `{"env":{"OTEL_METRICS_EXPORTER":"prometheus"}}`
	writeRaw(t, h.settingsPath(), settings)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "OTEL_METRICS_EXPORTER is already set to another value in "+h.settingsPath()+".")
	assertContains(t, output, "Pass --force to replace it")
	if got := readText(t, h.settingsPath()); got != settings {
		t.Errorf("settings = %s, want them untouched", got)
	}
	if got := s.received(); len(got) != 0 {
		t.Errorf("requests = %+v, want none", got)
	}
}

// Settings memry does not write but that would change where the user's
// own telemetry goes, or send its headers to memry, are in the way too.
func TestEnableStopsOnOtherOTLPSettings(t *testing.T) {
	for key, value := range map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT":         "https://collector.example.com",
		"OTEL_EXPORTER_OTLP_HEADERS":          "x-api-key=secret",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "https://collector.example.com/v1/metrics",
		"OTEL_EXPORTER_OTLP_METRICS_HEADERS":  "x-api-key=secret",
		"OTEL_EXPORTER_OTLP_PROTOCOL":         "grpc",
		"CLAUDE_CODE_ENABLE_TELEMETRY":        "0",
	} {
		t.Run(key, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)
			writeRaw(t, h.settingsPath(), `{"env":{"`+key+`":"`+value+`"}}`)

			output, code := h.run(usage.Enable)

			assertExit(t, code, 1, output)
			assertContains(t, output, key+" is already set to another value")
			if got := s.received(); len(got) != 0 {
				t.Errorf("requests = %+v, want none", got)
			}
		})
	}
}

// --force replaces memry's variables, and leaves the generic OTLP settings
// it does not write as they are.
func TestEnableWithForceReplacesTheSettingsInTheWay(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"env":{"OTEL_METRICS_EXPORTER":"prometheus","OTEL_EXPORTER_OTLP_ENDPOINT":"https://collector.example.com"}}`)

	output, code := h.run(usage.Enable, "--force")

	assertExit(t, code, 0, output)
	env := h.settingsEnv()
	if env["OTEL_METRICS_EXPORTER"] != "otlp" || env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "https://collector.example.com" {
		t.Errorf("env = %v, want the exporter replaced and the generic endpoint kept", env)
	}
}
