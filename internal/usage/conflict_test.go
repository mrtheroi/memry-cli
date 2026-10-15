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

// Each of memry's variables set to a value memry did not write is in the
// way.
func TestEnableStopsOnMemrysVariablesSetToOtherValues(t *testing.T) {
	for key, value := range map[string]string{
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "https://collector.example.com/v1/metrics",
		"OTEL_EXPORTER_OTLP_METRICS_HEADERS":  "x-api-key=secret",
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

// --force replaces the values of memry's own variables in the settings.
func TestEnableWithForceReplacesMemrysVariables(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"env":{"OTEL_METRICS_EXPORTER":"prometheus","OTEL_EXPORTER_OTLP_METRICS_PROTOCOL":"grpc"}}`)

	output, code := h.run(usage.Enable, "--force")

	assertExit(t, code, 0, output)
	env := h.settingsEnv()
	if env["OTEL_METRICS_EXPORTER"] != "otlp" || env["OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"] != "http/json" {
		t.Errorf("env = %v, want memry's values", env)
	}
}

// The generic OTLP settings would send the user's headers (credentials of
// another collector) to memry, or redirect other telemetry: --force never
// overrides them.
func TestEnableWithForceStillStopsOnGenericOTLPSettings(t *testing.T) {
	for _, key := range []string{"OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_PROTOCOL"} {
		t.Run(key, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)
			settings := `{"env":{"` + key + `":"x-api-key=secret"}}`
			writeRaw(t, h.settingsPath(), settings)

			output, code := h.run(usage.Enable, "--force")

			assertExit(t, code, 1, output)
			assertContains(t, output, key+" is set in "+h.settingsPath()+".")
			assertContains(t, output, "--force does not override it")
			if got := readText(t, h.settingsPath()); got != settings {
				t.Errorf("settings = %s, want them untouched", got)
			}
			if got := s.received(); len(got) != 0 {
				t.Errorf("requests = %+v, want none", got)
			}
		})
	}
}

// Claude Code's launch environment wins over its settings: a variable
// exported there with another value would override memry's, for example
// sending the usage token to another collector. --force cannot fix that.
func TestEnableStopsOnLaunchEnvironmentOverrides(t *testing.T) {
	for key, value := range map[string]string{
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "https://collector.example.com/v1/metrics",
		"OTEL_EXPORTER_OTLP_METRICS_HEADERS":  "Authorization=Bearer theirs",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL": "grpc",
		"OTEL_METRICS_EXPORTER":               "none",
		"CLAUDE_CODE_ENABLE_TELEMETRY":        "0",
		"OTEL_EXPORTER_OTLP_HEADERS":          "x-api-key=secret",
	} {
		t.Run(key, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)
			h.env[key] = value

			output, code := h.run(usage.Enable, "--force")

			assertExit(t, code, 1, output)
			assertContains(t, output, key+" is set in the environment.")
			assertContains(t, output, "--force does not override it")
			assertNotContains(t, output, value)
			if got := s.received(); len(got) != 0 {
				t.Errorf("requests = %+v, want none", got)
			}
		})
	}
}

// A launch environment value equal to memry's changes nothing.
func TestEnableAcceptsLaunchEnvironmentValuesEqualToMemrys(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.env["OTEL_METRICS_EXPORTER"] = "otlp"
	h.env["OTEL_LOGS_EXPORTER"] = ""

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
}

// CLAUDE_CODE_ENABLE_TELEMETRY turns on every signal: setting it would
// start a logs or traces export the user configured (possibly with prompt
// text) but never turned on. --force cannot fix that.
func TestEnableStopsWhenTheSwitchWouldActivateAnotherSignal(t *testing.T) {
	for _, key := range []string{"OTEL_LOGS_EXPORTER", "OTEL_TRACES_EXPORTER"} {
		for _, where := range []string{"settings", "environment"} {
			t.Run(key+" in "+where, func(t *testing.T) {
				s := newServer(t)
				h := newHarness(t)
				h.login(s.URL)
				place := h.settingsPath()
				if where == "settings" {
					writeRaw(t, h.settingsPath(), `{"env":{"`+key+`":"otlp"}}`)
				} else {
					h.env[key] = "otlp"
					place = "the environment"
				}

				output, code := h.run(usage.Enable, "--force")

				assertExit(t, code, 1, output)
				assertContains(t, output, key+" is set in "+place+"; turning on telemetry would start that export too.")
				if got := s.received(); len(got) != 0 {
					t.Errorf("requests = %+v, want none", got)
				}
			})
		}
	}
}

// Other signals already on by the user's own switch, or exporters set to
// none, are the user's choice and not in the way.
func TestEnableAcceptsOtherSignalsTheUserAlreadyTurnedOn(t *testing.T) {
	for name, settings := range map[string]string{
		"switch in settings": `{"env":{"CLAUDE_CODE_ENABLE_TELEMETRY":"1","OTEL_LOGS_EXPORTER":"otlp"}}`,
		"exporter none":      `{"env":{"OTEL_LOGS_EXPORTER":"none","OTEL_TRACES_EXPORTER":"none"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)
			writeRaw(t, h.settingsPath(), settings)

			output, code := h.run(usage.Enable)

			assertExit(t, code, 0, output)
		})
	}
	t.Run("switch in environment", func(t *testing.T) {
		s := newServer(t)
		h := newHarness(t)
		h.login(s.URL)
		h.env["CLAUDE_CODE_ENABLE_TELEMETRY"] = "1"
		h.env["OTEL_TRACES_EXPORTER"] = "otlp"

		output, code := h.run(usage.Enable)

		assertExit(t, code, 0, output)
	})
}

// The switch memry wrote is not the user's: a logs export added after
// enable is in the way of enabling again.
func TestEnableAgainStopsOnALogsExportAddedAfterMemrysSwitch(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	h.env["OTEL_LOGS_EXPORTER"] = "otlp"

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "OTEL_LOGS_EXPORTER is set in the environment; turning on telemetry would start that export too.")
}
