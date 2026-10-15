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
	writeRaw(t, h.settingsPath(), `{"model":"opus","env":{"FOO":"bar","EDITOR":"vim"}}`)
	h.run(usage.Enable)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are off. Removed them from "+h.settingsPath()+"; restart Claude Code to apply it.")
	want := map[string]any{"FOO": "bar", "EDITOR": "vim"}
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

// A variable that already had memry's value before enable was not written
// by memry: disable keeps it.
func TestDisableKeepsWhatTheUserHadSetBeforeEnable(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"env":{"CLAUDE_CODE_ENABLE_TELEMETRY":"1","EDITOR":"vim"}}`)
	h.run(usage.Enable)
	h.run(usage.Enable) // Enabling again does not take it for memry's.

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are off.")
	want := map[string]any{"CLAUDE_CODE_ENABLE_TELEMETRY": "1", "EDITOR": "vim"}
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

// Disable revokes exactly the usage token this machine recorded, once
// memry's variables are gone; another machine's survives.
func TestDisableRevokesTheRecordedUsageTokenOnly(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	other := s.issueElsewhere()

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	if got := s.activeTokens(); !reflect.DeepEqual(got, []int{other}) {
		t.Errorf("active tokens = %v, want only the other machine's", got)
	}
	for _, r := range s.received() {
		if r.Path == "/api/auth/usage-tokens" {
			t.Errorf("sent %s %s?%s, want no bulk revocation", r.Method, r.Path, r.Query)
		}
	}
}

func TestDisableSucceedsWithAWarningWhenTheTokenCannotBeRevoked(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	s.answerRevoke(401)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Could not revoke your usage token; it stops working after 30 days without use.")
	assertContains(t, output, "Usage metrics are off.")
	assertNotContains(t, readText(t, h.configPath), `"usage_metrics"`)
}

// A recorded token the server no longer has (404) is already revoked.
func TestDisableTakesATokenAlreadyGoneAsRevoked(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	s.answerRevoke(404)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertNotContains(t, output, "Could not revoke")
}
