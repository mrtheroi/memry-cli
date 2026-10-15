package uninstall_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// Uninstall turns off the usage metrics memry set up, before the login it
// needs to recognize them is deleted, and keeps the user's own variables.
func TestUninstallTurnsOffUsageMetrics(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.previousConfig(map[string]any{"url": s.URL, "token": "1|abc", "agents": []string{"claude-code"}})
	writeRaw(t, h.settingsPath, `{"env":{"FOO":"bar","CLAUDE_CODE_ENABLE_TELEMETRY":"1","OTEL_METRICS_EXPORTER":"otlp","OTEL_EXPORTER_OTLP_METRICS_PROTOCOL":"http/json","OTEL_EXPORTER_OTLP_METRICS_ENDPOINT":"`+s.URL+`/api/otlp/v1/metrics","OTEL_EXPORTER_OTLP_METRICS_HEADERS":"Authorization=Bearer 7|usage"}}`)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are off. Removed them from "+h.settingsPath+"; restart Claude Code to apply it.")
	var settings struct{ Env map[string]any }
	if err := json.Unmarshal([]byte(readText(t, h.settingsPath)), &settings); err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"FOO": "bar"}; !reflect.DeepEqual(settings.Env, want) {
		t.Errorf("env = %v, want %v", settings.Env, want)
	}
}

// Claude Code settings memry cannot read do not fail an uninstall that
// never turned on usage metrics.
func TestUninstallIgnoresUnreadableSettingsWithoutUsageMetrics(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	codex := &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(codex)
	h.previousConfig(map[string]any{"url": s.URL, "token": "1|abc", "agents": []string{"codex"}})
	writeRaw(t, h.settingsPath, `{"env":`)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 0, output)
	assertNotContains(t, output, "usage metrics")
}

// When memry turned usage metrics on and cannot turn them off, the login
// they are recognized by is kept for a retry.
func TestUninstallKeepsTheLoginWhenUsageMetricsCannotBeTurnedOff(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	codex := &fakeAgent{key: "codex", name: "Codex"}
	h.useAgents(codex)
	h.previousConfig(map[string]any{"url": s.URL, "token": "1|abc", "agents": []string{"codex"},
		"usage_metrics": map[string]any{"endpoint": s.URL + "/api/otlp/v1/metrics", "kept": []string{}}})
	writeRaw(t, h.settingsPath, `{"env":`)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not turn off usage metrics: "+h.settingsPath+" is not valid JSON")
	if cfg := h.config(); cfg["token"] != "1|abc" {
		t.Errorf("config = %v, want the login kept", cfg)
	}
}
