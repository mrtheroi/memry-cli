package uninstall_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// Uninstall turns off the usage metrics memry set up, before the login it
// needs to recognize them is deleted, and keeps the user's own variables.
func TestUninstallTurnsOffUsageMetrics(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	values := map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY":        "1",
		"OTEL_METRICS_EXPORTER":               "otlp",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": s.URL + "/api/otlp/v1/metrics",
		"OTEL_EXPORTER_OTLP_METRICS_HEADERS":  "Authorization=Bearer 7|usage",
	}
	h.previousConfig(map[string]any{"url": s.URL, "token": "1|abc", "agents": []string{"claude-code"},
		"usage_metrics": map[string]any{"current": map[string]any{"settings_path": h.settingsPath, "values": values, "kept": []string{}, "token_id": 7, "server_url": s.URL}}})
	env := map[string]string{"FOO": "bar"}
	for key, value := range values {
		env[key] = value
	}
	data, _ := json.Marshal(map[string]any{"env": env})
	writeRaw(t, h.settingsPath, string(data))

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
	// With the login token, before it is revoked.
	if got := s.received(); len(got) < 2 || got[0].Method != "DELETE" || got[0].Path != "/api/auth/tokens/7" || got[1].Path != "/api/auth/token" {
		t.Errorf("requests = %+v, want the recorded usage token revoked, then the login token", got)
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
		"usage_metrics": map[string]any{"current": map[string]any{"settings_path": h.settingsPath, "values": map[string]string{"OTEL_METRICS_EXPORTER": "otlp"}, "kept": []string{}}}})
	writeRaw(t, h.settingsPath, `{"env":`)

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not turn off usage metrics: "+h.settingsPath+" is not valid JSON")
	if cfg := h.config(); cfg["token"] != "1|abc" {
		t.Errorf("config = %v, want the login kept", cfg)
	}
}

// A delete-account retry keeps nothing but the agents to clean up, and the
// usage metrics record: without it, the retry could not recognize what
// memry wrote.
func TestDeleteAccountKeepsTheUsageMetricsRecordForARetry(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.useAgents(&fakeAgent{key: "codex", name: "Codex"})
	record := map[string]any{"current": map[string]any{"settings_path": h.settingsPath, "values": map[string]string{"OTEL_METRICS_EXPORTER": "otlp"}, "kept": []string{}, "token_id": 7, "server_url": s.URL}}
	h.previousConfig(map[string]any{"url": s.URL, "token": "1|abc", "agents": []string{"codex"}, "usage_metrics": record})
	writeRaw(t, h.settingsPath, `{"env":`)

	output, code := h.run(uninstall.DeleteAccount, nil, answer{label: typeYourEmail, value: "ana@example.com"})

	assertExit(t, code, 1, output)
	cfg := h.config()
	if _, ok := cfg["usage_metrics"]; !ok || cfg["token"] != nil {
		t.Errorf("config = %v, want the usage_metrics record without the login", cfg)
	}
	// The account deletion revoked every token already.
	for _, r := range s.received() {
		if strings.HasPrefix(r.Path, "/api/auth/tokens/") || r.Path == "/api/auth/usage-tokens" {
			t.Errorf("sent %s %s, want no usage token revocation", r.Method, r.Path)
		}
	}
}

// A malformed config may hold the usage metrics record: uninstall and
// delete-account stop before changing anything, never deleting it or
// writing over it.
func TestRemovalLeavesAMalformedConfigUntouched(t *testing.T) {
	for name, run := range map[string]func(*harness) (string, int){
		"uninstall":      func(h *harness) (string, int) { return h.run(uninstall.Uninstall, force) },
		"delete-account": func(h *harness) (string, int) { return h.run(uninstall.DeleteAccount, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t)
			h := newHarness(t)
			codex := &fakeAgent{key: "codex", name: "Codex"}
			h.useAgents(codex)
			writeRaw(t, h.configPath, `{"url":"`+s.URL+`",`)

			output, code := run(h)

			assertExit(t, code, 1, output)
			assertContains(t, output, "Could not read the memry config "+h.configPath+", so memry cannot tell whether usage metrics are on. Fix it, then try again.")
			if got := readText(t, h.configPath); got != `{"url":"`+s.URL+`",` {
				t.Errorf("config = %q, want it untouched", got)
			}
			assertCalls(t, codex)
			if got := s.received(); len(got) != 0 {
				t.Errorf("requests = %+v, want none", got)
			}
		})
	}
}
