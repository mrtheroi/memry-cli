package usage_test

import (
	"encoding/json"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// switchServer changes the login to url, as `memry setup` with another
// server does: the usage metrics record stays.
func (h *harness) switchServer(url string) {
	h.t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(readText(h.t, h.configPath)), &cfg); err != nil {
		h.t.Fatal(err)
	}
	cfg["url"] = url
	h.writeConfig(cfg)
}

// The usage token belongs to the server that issued it: enabling for
// another one first needs the old setup turned off.
func TestEnableForAnotherServerStopsWhileTheFirstIsOn(t *testing.T) {
	first, second := newServer(t), newServer(t)
	h := newHarness(t)
	h.login(first.URL)
	h.run(usage.Enable)
	h.switchServer(second.URL)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Usage metrics are on for "+first.URL+". Run `memry usage disable` before enabling for "+second.URL+".")
	if got := second.received(); len(got) != 0 {
		t.Errorf("requests to the new server = %+v, want none", got)
	}
}

// Disable after a switch to another server still removes memry's
// variables, but never sends the old server's token id to the new one.
func TestDisableOnAnotherServerSkipsTheRevoke(t *testing.T) {
	first, second := newServer(t), newServer(t)
	h := newHarness(t)
	h.login(first.URL)
	h.run(usage.Enable)
	h.switchServer(second.URL)

	output, code := h.run(usage.Disable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are off.")
	assertContains(t, output, "Did not revoke the usage token of "+first.URL+": you are logged in to another server now. It stops working after 30 days without use.")
	if got := second.received(); len(got) != 0 {
		t.Errorf("requests to the new server = %+v, want none", got)
	}
	if got := h.settingsEnv(); len(got) != 0 {
		t.Errorf("env = %v, want memry's variables removed", got)
	}
}
