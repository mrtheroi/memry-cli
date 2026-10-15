package usage_test

import (
	"os"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// On Windows the home is USERPROFILE, not the HOME Git Bash and MSYS set to
// a POSIX-style path, as for the agents.
func TestEnableOnWindowsWritesUnderUserprofile(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.goos = "windows"
	decoy := t.TempDir()
	h.env["HOME"] = decoy
	h.login(s.URL)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	if got := h.settingsEnv()["OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"]; got != s.URL+metricsPath {
		t.Errorf("endpoint under USERPROFILE = %v", got)
	}
	if entries, _ := os.ReadDir(decoy); len(entries) != 0 {
		t.Errorf("HOME holds %v, want nothing", entries)
	}
}

// A drive-relative CLAUDE_CONFIG_DIR would be recorded as a path that
// means another file from another directory: enable refuses it.
func TestEnableRefusesADriveRelativeConfigDir(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.goos = "windows"
	h.login(s.URL)
	h.env["CLAUDE_CONFIG_DIR"] = "C:profile"

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, `Could not turn on usage metrics: CLAUDE_CONFIG_DIR C:profile is relative to a drive's current directory; set it to an absolute path, like C:\profile.`)
	if got := s.received(); len(got) != 0 {
		t.Errorf("requests = %+v, want none", got)
	}
}
