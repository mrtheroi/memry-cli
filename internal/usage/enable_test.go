package usage_test

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

func TestEnableAddsTheMetricsExportKeepingEverythingElse(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"model":"opus","env":{"FOO":"bar"}}`)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	want := map[string]any{
		"FOO":                                 "bar",
		"CLAUDE_CODE_ENABLE_TELEMETRY":        "1",
		"OTEL_METRICS_EXPORTER":               "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL":         "http/json",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": s.URL + "/api/otlp/v1/metrics",
		"OTEL_EXPORTER_OTLP_METRICS_HEADERS":  "Authorization=Bearer 7|usage-secret",
	}
	if got := h.settingsEnv(); !reflect.DeepEqual(got, want) {
		t.Errorf("env = %v, want %v", got, want)
	}
	assertContains(t, readText(t, h.settingsPath()), `"model": "opus"`)
	if got := s.received(); len(got) != 1 || got[0] != (request{"POST", "/api/auth/usage-token", "Bearer 1|login-token"}) {
		t.Errorf("requests = %+v, want one POST /api/auth/usage-token with the login token", got)
	}
}

func TestEnableNeedsALogin(t *testing.T) {
	h := newHarness(t)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "You are not logged in to memry. Run `memry setup` first.")
	if _, err := os.Stat(h.settingsPath()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("settings stat = %v, want no file", err)
	}
}

// Like setup, a usage token is never sent over plain HTTP to a server
// that is not this machine, unless --insecure allows it.
func TestEnableRefusesPlainHTTPToARemoteServerUnlessInsecure(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.via = s
	h.login("http://memry.example.com")

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Refusing to send usage metrics over plain HTTP to memry.example.com. Use an https:// server, or pass --insecure to allow it (for example on a trusted LAN).")
	if got := s.received(); len(got) != 0 {
		t.Errorf("requests = %+v, want none", got)
	}

	output, code = h.run(usage.Enable, "--insecure")

	assertExit(t, code, 0, output)
	if got := h.settingsEnv()["OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"]; got != "http://memry.example.com/api/otlp/v1/metrics" {
		t.Errorf("endpoint = %v", got)
	}
}

func TestEnableLeavesTheSettingsUntouchedWhenTheServerRefuses(t *testing.T) {
	tests := map[string]struct {
		status  int
		body    string
		message string
	}{
		"expired login": {401, `{"message":"Unauthenticated."}`, "Your memry login is no longer valid. Run `memry setup` and try again."},
		"rate limited":  {429, `{}`, "Too many attempts, try again later."},
		"server error":  {500, `oops`, "The memry server returned an unexpected error (HTTP 500)."},
		"no token":      {201, `{}`, "The memry server did not return a usage token."},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newServer(t)
			s.answer(tt.status, tt.body)
			h := newHarness(t)
			h.login(s.URL)
			writeRaw(t, h.settingsPath(), `{"env":{"FOO":"bar"}}`)

			output, code := h.run(usage.Enable)

			assertExit(t, code, 1, output)
			assertContains(t, output, tt.message)
			if got := readText(t, h.settingsPath()); got != `{"env":{"FOO":"bar"}}` {
				t.Errorf("settings = %s, want them untouched", got)
			}
		})
	}
}

// Enabling again rotates the usage token: only the header changes.
func TestEnableAgainOnlyRotatesTheToken(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"env":{"FOO":"bar"}}`)
	h.run(usage.Enable)
	first := readText(t, h.settingsPath())
	s.answer(201, `{"token":"8|rotated"}`)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	want := strings.Replace(first, "Bearer 7|usage-secret", "Bearer 8|rotated", 1)
	if got := readText(t, h.settingsPath()); got != want {
		t.Errorf("settings =\n%s\nwant\n%s", got, want)
	}
}

func TestEnableExplainsWhatIsSentWhereAndHowToStopIt(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are on.")
	assertContains(t, output, "Claude Code will send its token counts and cost per model to "+s.URL+metricsPath+", your memry server, and nowhere else.")
	assertContains(t, output, "No prompts, code or file contents are sent.")
	assertContains(t, output, "Saved in "+h.settingsPath()+"; restart Claude Code to apply it.")
	assertContains(t, output, "Run `memry usage disable` to turn it off.")
	assertNotContains(t, output, "usage-secret")
}

const metricsPath = "/api/otlp/v1/metrics"

func TestEnableLeavesSettingsThatAreNotValidJSONUntouched(t *testing.T) {
	for _, contents := range []string{`{"env":`, `{"env":["FOO"]}`} {
		s := newServer(t)
		h := newHarness(t)
		h.login(s.URL)
		writeRaw(t, h.settingsPath(), contents)

		output, code := h.run(usage.Enable)

		assertExit(t, code, 1, output)
		assertContains(t, output, "Could not turn on usage metrics: "+h.settingsPath()+` is not valid JSON or its "env" is not an object. Fix the file, then try again.`)
		if got := readText(t, h.settingsPath()); got != contents {
			t.Errorf("settings = %s, want them untouched", got)
		}
		if got := s.received(); len(got) != 0 {
			t.Errorf("requests = %+v, want none", got)
		}
	}
}
