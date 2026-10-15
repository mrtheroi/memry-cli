package usage_test

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// interruptedEnable leaves what an enable killed after saving its pending
// setup leaves: a pending setup with a new token, issued by the server, on
// top of the current one (token 7). With written, the settings write also
// happened. It returns the new token's id.
func interruptedEnable(t *testing.T, h *harness, s *server, written bool) int {
	t.Helper()
	h.run(usage.Enable)
	id := s.issueElsewhere()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(readText(t, h.configPath)), &cfg); err != nil {
		t.Fatal(err)
	}
	record := cfg["usage_metrics"].(map[string]any)
	current := record["current"].(map[string]any)
	pending := map[string]any{}
	for key, value := range current {
		pending[key] = value
	}
	values := map[string]any{}
	for key, value := range current["values"].(map[string]any) {
		values[key] = value
	}
	values["OTEL_EXPORTER_OTLP_METRICS_HEADERS"] = "Authorization=Bearer " + usageToken(id)
	expected := map[string]any{}
	for key, value := range current["expected"].(map[string]any) {
		expected[key] = value
	}
	expected["OTEL_EXPORTER_OTLP_METRICS_HEADERS"] = values["OTEL_EXPORTER_OTLP_METRICS_HEADERS"]
	pending["values"], pending["expected"], pending["token_id"] = values, expected, id
	record["pending"] = pending
	h.writeConfig(cfg)
	if written {
		env := h.settingsEnv()
		env["OTEL_EXPORTER_OTLP_METRICS_HEADERS"] = values["OTEL_EXPORTER_OTLP_METRICS_HEADERS"]
		h.writeSettingsEnv(env)
	}
	return id
}

// The settings still hold the old token: the interrupted enable never
// wrote them, so its pending setup is dropped and its token revoked.
func TestAnyCommandDropsAPendingSetupTheSettingsNeverGot(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	interruptedEnable(t, h, s, false)

	output, code := h.run(usage.Status)

	assertExit(t, code, 0, output)
	if got := s.activeTokens(); !reflect.DeepEqual(got, []int{7}) {
		t.Errorf("active tokens = %v, want the current one only", got)
	}
	assertNotContains(t, readText(t, h.configPath), `"pending"`)
	assertContains(t, output, "Usage metrics are on")
}

// The settings hold the new token: the interrupted enable wrote them, so
// its pending setup becomes current and the previous token is revoked.
// Disable then removes and revokes the new one.
func TestAnyCommandPromotesAPendingSetupTheSettingsGot(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	id := interruptedEnable(t, h, s, true)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	if got := s.activeTokens(); len(got) != 0 {
		t.Errorf("active tokens = %v, want none", got)
	}
	got := s.received()
	if revoked := got[len(got)-2:]; revoked[0].Path != "/api/auth/tokens/7" || revoked[1].Path != "/api/auth/tokens/"+strconv.Itoa(id) {
		t.Errorf("revocations = %+v, want the previous token, then the promoted one", revoked)
	}
	if got := h.settingsEnv(); len(got) != 0 {
		t.Errorf("env = %v, want memry's variables removed", got)
	}
}

// Enable settles an interrupted enable first, so no token is left behind.
func TestEnableSettlesAnInterruptedEnableFirst(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	interruptedEnable(t, h, s, false)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	if got := s.activeTokens(); !reflect.DeepEqual(got, []int{9}) {
		t.Errorf("active tokens = %v, want only the new one", got)
	}
}

// Before it writes the settings, enable has saved both setups: a crash
// there leaves the previous one current and the new one pending.
func TestEnableSavesThePendingSetupBeforeWritingTheSettings(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	var record struct {
		UsageMetrics struct {
			Current, Pending *struct {
				TokenID int `json:"token_id"`
			}
		} `json:"usage_metrics"`
	}
	t.Cleanup(usage.SetBeforeSettingsWrite(func() {
		if err := json.Unmarshal([]byte(readText(t, h.configPath)), &record); err != nil {
			t.Fatal(err)
		}
	}))

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	if r := record.UsageMetrics; r.Current == nil || r.Current.TokenID != 7 || r.Pending == nil || r.Pending.TokenID != 8 {
		t.Errorf("record before the write = %+v, want token 7 current and token 8 pending", r)
	}
	assertNotContains(t, readText(t, h.configPath), `"pending"`)
}

// A failed enable restores the record only while the pending setup is
// still its own: one another invocation saved meanwhile is left alone.
func TestAFailedEnableLeavesAnotherInvocationsPendingSetup(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	var theirs string
	t.Cleanup(usage.SetBeforeSettingsWrite(func() {
		var cfg map[string]any
		_ = json.Unmarshal([]byte(readText(t, h.configPath)), &cfg)
		record := cfg["usage_metrics"].(map[string]any)
		pending := record["pending"].(map[string]any)
		pending["token_id"] = 99
		h.writeConfig(cfg)
		theirs = readText(t, h.configPath)
		env := h.settingsEnv()
		env["EDITOR"] = "vim"
		h.writeSettingsEnv(env)
	}))

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	if got := readText(t, h.configPath); got != theirs {
		t.Errorf("config =\n%s\nwant the other invocation's record kept:\n%s", got, theirs)
	}
}
