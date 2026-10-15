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
