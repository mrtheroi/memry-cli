package usage_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// enableBlocked runs enable with --force and checks it stops before asking
// for a token, saying want.
func enableBlocked(t *testing.T, h *harness, s *server, want string) {
	t.Helper()
	output, code := h.run(usage.Enable, "--force")
	assertExit(t, code, 1, output)
	assertContains(t, output, want)
	if got := s.received(); len(got) != 0 {
		t.Errorf("requests = %+v, want none", got)
	}
}

// Claude Code adds the headers of an otelHeadersHelper (another
// collector's credentials) to the ones it sends to memry.
func TestEnableStopsOnAnOTLPHeadersHelper(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"otelHeadersHelper":"/bin/headers.sh"}`)

	enableBlocked(t, h, s, "otelHeadersHelper is set in "+h.settingsPath()+".")
}

// OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES=false only keeps custom
// resource attributes off the data points: Claude Code still sends them in
// the OTLP resource block.
func TestEnableStopsOnCustomResourceAttributes(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		s := newServer(t)
		h := newHarness(t)
		h.login(s.URL)
		h.env["OTEL_RESOURCE_ATTRIBUTES"] = "department=engineering"

		enableBlocked(t, h, s, "OTEL_RESOURCE_ATTRIBUTES is set in the environment.")
	})
	t.Run("settings", func(t *testing.T) {
		s := newServer(t)
		h := newHarness(t)
		h.login(s.URL)
		writeRaw(t, h.settingsPath(), `{"env":{"OTEL_RESOURCE_ATTRIBUTES":"cost_center=eng-123"}}`)

		enableBlocked(t, h, s, "OTEL_RESOURCE_ATTRIBUTES is set in "+h.settingsPath()+".")
	})
}

// Managed settings win over the user's, and a managed OTLP setting removes
// memry's endpoint at startup: memry cannot promise where metrics go.
func TestEnableStopsOnManagedTelemetrySettings(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	managed := filepath.Join(h.managedDir, "managed-settings.json")
	writeRaw(t, managed, `{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"https://collector.example.com"}}`)

	enableBlocked(t, h, s, "OTEL_EXPORTER_OTLP_ENDPOINT is set in the managed settings "+managed+".")
}

// Drop-in files of managed-settings.d count too, as does a managed
// otelHeadersHelper or telemetry switch.
func TestEnableStopsOnManagedDropInSettings(t *testing.T) {
	for name, contents := range map[string]string{
		"otelHeadersHelper":            `{"otelHeadersHelper":"/bin/headers.sh"}`,
		"CLAUDE_CODE_ENABLE_TELEMETRY": `{"env":{"CLAUDE_CODE_ENABLE_TELEMETRY":"0"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)
			dropIn := filepath.Join(h.managedDir, "managed-settings.d", "10-telemetry.json")
			writeRaw(t, dropIn, contents)

			enableBlocked(t, h, s, name+" is set in the managed settings "+dropIn+".")
		})
	}
}

// A managed settings file memry cannot parse may lock the destination:
// where metrics would go cannot be determined.
func TestEnableStopsWhenManagedSettingsCannotBeParsed(t *testing.T) {
	for name, contents := range map[string]string{"not JSON": `{"env":`, "not an object": `[]`, "env not an object": `{"env":"x"}`} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			h.login(s.URL)
			managed := filepath.Join(h.managedDir, "managed-settings.json")
			writeRaw(t, managed, contents)

			enableBlocked(t, h, s, "Could not read the managed settings "+managed+", so memry cannot determine where Claude Code sends metrics.")
		})
	}
}

// A managed policyHelper computes the managed settings at startup: what
// they set cannot be determined, so enable stops. memry never runs it.
func TestEnableStopsOnAManagedPolicyHelper(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	managed := filepath.Join(h.managedDir, "managed-settings.json")
	writeRaw(t, managed, `{"policyHelper":{"path":"/usr/local/bin/policy"}}`)

	enableBlocked(t, h, s, "policyHelper is set in the managed settings "+managed+", so memry cannot determine where Claude Code sends metrics.")
}
