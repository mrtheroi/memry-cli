package usage_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// Ownership comes from the record of what memry wrote, never from which
// values happen to be present: after the user removes only the endpoint,
// enabling again does not take memry's other values for the user's.
func TestReEnableAfterAPartialEditKeepsMemrysOwnership(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"env":{"FOO":"bar"}}`)
	h.run(usage.Enable)
	env := h.settingsEnv()
	delete(env, "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	h.writeSettingsEnv(env)
	h.run(usage.Enable)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	if got, want := h.settingsEnv(), map[string]any{"FOO": "bar"}; !reflect.DeepEqual(got, want) {
		t.Errorf("env = %v, want %v", got, want)
	}
}

// Disable cleans up the settings enable wrote, even when Claude Code's
// config dir is another one now.
func TestDisableUsesTheSettingsPathEnableWrote(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	profile := filepath.Join(t.TempDir(), "work")
	h.env["CLAUDE_CONFIG_DIR"] = profile
	h.run(usage.Enable)
	delete(h.env, "CLAUDE_CONFIG_DIR")

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Removed them from "+filepath.Join(profile, "settings.json"))
	if got := readText(t, filepath.Join(profile, "settings.json")); got != "{}\n" {
		t.Errorf("settings = %s, want memry's variables removed", got)
	}
}

// Without a record, nothing is memry's, even values matching the server.
func TestDisableWithoutARecordRemovesNothing(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	settings := `{"env":{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT":"` + s.URL + `/api/otlp/v1/metrics"}}`
	writeRaw(t, h.settingsPath(), settings)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are not on; nothing to remove.")
	if got := readText(t, h.settingsPath()); got != settings {
		t.Errorf("settings = %s, want them untouched", got)
	}
}

// When the settings enable wrote cannot be read, disable says so and keeps
// the record for a retry.
func TestDisableFailsClearlyWhenTheRecordedSettingsAreUnreadable(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	writeRaw(t, h.settingsPath(), `{"env":`)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not turn off usage metrics: "+h.settingsPath()+" is not valid JSON")
	assertContains(t, readText(t, h.configPath), `"usage_metrics"`)
}

// One record, one settings file: enabling for another Claude Code config
// dir while memry's export is in the first one would lose track of it.
func TestEnableForAnotherSettingsPathStopsWhileTheFirstIsOn(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	h.env["CLAUDE_CONFIG_DIR"] = filepath.Join(t.TempDir(), "work")
	before := len(s.received())

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Usage metrics are on in "+h.settingsPath()+". Run `memry usage disable` first to move them to "+filepath.Join(h.env["CLAUDE_CONFIG_DIR"], "settings.json")+".")
	if got := s.received(); len(got) != before {
		t.Errorf("requests = %+v, want no new one", got[before:])
	}
}

// When the user already removed everything memry wrote, disable has
// nothing to remove but still forgets the record.
func TestDisableForgetsARecordWhoseValuesAreGone(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	h.writeSettingsEnv(map[string]any{})

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are not on; nothing to remove.")
	assertNotContains(t, readText(t, h.configPath), `"usage_metrics"`)
}
