package usage_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

func TestStatusShowsTheEndpointButNeverTheToken(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)

	output, code := h.run(usage.Status)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are on, as far as memry can see: Claude Code sends its metrics to "+s.URL+metricsPath+".")
	assertContains(t, output, "Set in "+h.settingsPath()+".")
	assertNotContains(t, output, "usage-secret")
	assertNotContains(t, output, "login-token")
	assertNotContains(t, output, "Bearer")
}

func TestStatusOff(t *testing.T) {
	h := newHarness(t)
	h.login("https://memry.example.com")
	writeRaw(t, h.settingsPath(), `{"env":{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT":"https://collector.example.com/v1/metrics","OTEL_EXPORTER_OTLP_METRICS_HEADERS":"Authorization=Bearer theirs"}}`)

	output, code := h.run(usage.Status)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are off.")
	assertNotContains(t, output, "theirs")
}

// Status checks what Claude Code would run with, the launch environment
// first, not only what memry wrote.
func TestStatusReportsAnOverrideInTheEnvironment(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	h.env["CLAUDE_CODE_ENABLE_TELEMETRY"] = "0"

	output, code := h.run(usage.Status)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are not active:\n- CLAUDE_CODE_ENABLE_TELEMETRY is overridden in the environment.\n")
	assertNotContains(t, output, "are on")
}

func TestStatusReportsWhatIsMissingOrChangedInTheSettings(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	env := h.settingsEnv()
	delete(env, "OTEL_EXPORTER_OTLP_METRICS_HEADERS")
	env["OTEL_METRICS_EXPORTER"] = "none"
	h.writeSettingsEnv(env)

	output, code := h.run(usage.Status)

	assertExit(t, code, 0, output)
	assertContains(t, output, "- OTEL_METRICS_EXPORTER is not memry's value in "+h.settingsPath()+".\n")
	assertContains(t, output, "- OTEL_EXPORTER_OTLP_METRICS_HEADERS is missing from "+h.settingsPath()+".\n")
	assertNotContains(t, output, "are on")
}

// Headers without a bearer token cannot authenticate; their value is
// never printed.
func TestStatusReportsHeadersWithoutAToken(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	h.env["OTEL_EXPORTER_OTLP_METRICS_HEADERS"] = "x-api-key=secret"

	output, code := h.run(usage.Status)

	assertContains(t, output, "- OTEL_EXPORTER_OTLP_METRICS_HEADERS is overridden in the environment.\n")
	assertNotContains(t, output, "secret")
	assertExit(t, code, 0, output)
}

// What would stop enable now also keeps status from claiming the metrics
// reach memry, such as a managed endpoint or a headers helper.
func TestStatusReportsManagedSettingsAndHelpersInTheWay(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	managed := filepath.Join(h.managedDir, "managed-settings.json")
	writeRaw(t, managed, `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"https://collector.example.com"}}`)

	output, code := h.run(usage.Status)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are not active:\n- OTEL_EXPORTER_OTLP_ENDPOINT is set in the managed settings "+managed+".\n")
}
