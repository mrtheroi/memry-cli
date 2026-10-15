//go:build !windows

package usage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// readOnly makes dir read-only until the test ends, so nothing can be
// created in it.
func readOnly(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

// When the record of the new token cannot be saved, nothing changes: the
// settings keep the working token, and the new one is revoked.
func TestEnableAgainChangesNothingWhenTheRecordCannotBeSaved(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	settings := readText(t, h.settingsPath())
	readOnly(t, filepath.Dir(h.configPath))

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not save the usage metrics record to "+h.configPath)
	if got := readText(t, h.settingsPath()); got != settings {
		t.Errorf("settings =\n%s\nwant them unchanged:\n%s", got, settings)
	}
	if got := s.activeTokens(); !reflect.DeepEqual(got, []int{7}) {
		t.Errorf("active tokens = %v, want the old one only", got)
	}
}

// When the settings cannot be written, the previous record is restored
// and the new token revoked: the old headers, and the old token they hold,
// keep working.
func TestEnableAgainRestoresTheRecordWhenTheSettingsCannotBeWritten(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	settings, record := readText(t, h.settingsPath()), readText(t, h.configPath)
	readOnly(t, filepath.Dir(h.settingsPath()))

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not turn on usage metrics: ")
	if got := readText(t, h.settingsPath()); got != settings {
		t.Errorf("settings changed:\n%s", got)
	}
	if got := readText(t, h.configPath); got != record {
		t.Errorf("config =\n%s\nwant the previous record:\n%s", got, record)
	}
	if got := s.activeTokens(); !reflect.DeepEqual(got, []int{7}) {
		t.Errorf("active tokens = %v, want the old one only", got)
	}
}

// On a first enable, a failed settings write leaves no record behind.
func TestFirstEnableLeavesNoRecordWhenTheSettingsCannotBeWritten(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{}`)
	readOnly(t, filepath.Dir(h.settingsPath()))

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertNotContains(t, readText(t, h.configPath), `"usage_metrics"`)
}

// When the pending setup cannot be settled, which setup the settings
// hold is not recorded: disable fails closed and changes nothing.
func TestDisableFailsClosedWhenThePendingSetupCannotBeSettled(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	interruptedEnable(t, h, s, true)
	settings := readText(t, h.settingsPath())
	readOnly(t, filepath.Dir(h.configPath))

	output, code := h.run(usage.Disable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "so memry cannot tell whether usage metrics are on")
	if got := readText(t, h.settingsPath()); got != settings {
		t.Errorf("settings changed:\n%s", got)
	}
}

// A pending setup that cannot be settled, here because the config cannot
// be saved, is unknown state, never "off": the settings may hold its live
// exporter and token.
func TestStatusReportsUnknownWhenAPendingSetupCannotBeSettled(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	// A first enable stopped after writing the settings: only pending.
	var cfg map[string]any
	if err := json.Unmarshal([]byte(readText(t, h.configPath)), &cfg); err != nil {
		t.Fatal(err)
	}
	record := cfg["usage_metrics"].(map[string]any)
	record["pending"] = record["current"]
	delete(record, "current")
	h.writeConfig(cfg)
	readOnly(t, filepath.Dir(h.configPath))

	output, code := h.run(usage.Status)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not read the memry config "+h.configPath+", so memry cannot tell whether usage metrics are on. Fix it, then try again.")
	assertNotContains(t, output, "are off")
}
