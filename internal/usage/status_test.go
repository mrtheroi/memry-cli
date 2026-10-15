package usage_test

import (
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
	assertContains(t, output, "Usage metrics are on: Claude Code sends its token counts and cost per model to "+s.URL+metricsPath+".")
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
