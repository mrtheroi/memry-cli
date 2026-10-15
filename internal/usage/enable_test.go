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
	wantEnv := map[string]any{
		"FOO":                                      "bar",
		"CLAUDE_CODE_ENABLE_TELEMETRY":             "1",
		"OTEL_METRICS_EXPORTER":                    "otlp",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL":      "http/json",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT":      s.URL + "/api/otlp/v1/metrics",
		"OTEL_EXPORTER_OTLP_METRICS_HEADERS":       "Authorization=Bearer " + usageToken(7),
		"OTEL_METRICS_INCLUDE_SESSION_ID":          "false",
		"OTEL_METRICS_INCLUDE_ACCOUNT_UUID":        "false",
		"OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES": "false",
		"OTEL_METRICS_INCLUDE_REPOSITORY":          "false",
		"OTEL_METRICS_INCLUDE_VERSION":             "false",
		"OTEL_METRICS_INCLUDE_ENTRYPOINT":          "false",
	}
	if got := h.settingsEnv(); !reflect.DeepEqual(got, wantEnv) {
		t.Errorf("env = %v, want %v", got, wantEnv)
	}
	assertContains(t, readText(t, h.settingsPath()), `"model": "opus"`)
	// A first enable has no previous token to revoke: only the new one is
	// asked for, with the login token.
	want := []request{
		{"POST", "/api/auth/usage-token", "", "Bearer 1|login-token"},
	}
	if got := s.received(); !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %+v, want %+v", got, want)
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
		"no id":         {201, `{"token":"x"}`, "The memry server did not return a usage token."},
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

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	want := strings.Replace(first, "Bearer "+usageToken(7), "Bearer "+usageToken(8), 1)
	if got := readText(t, h.settingsPath()); got != want {
		t.Errorf("settings =\n%s\nwant\n%s", got, want)
	}
	if got := s.activeTokens(); !reflect.DeepEqual(got, []int{8}) {
		t.Errorf("active tokens = %v, want only the new one", got)
	}
}

// What Claude Code sends, and what the server keeps, is told before the
// usage token is asked for, so even a failed enable has said it.
func TestEnableDisclosesThePayloadBeforeAskingForAToken(t *testing.T) {
	s := newServer(t)
	s.answer(500, `{}`)
	h := newHarness(t)
	h.login(s.URL)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	for _, line := range disclosure(s.URL + metricsPath) {
		assertContains(t, output, line)
	}
}

func TestEnableSaysWhereItIsSavedAndHowToStopIt(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Usage metrics are on. Saved in "+h.settingsPath()+"; restart Claude Code to apply it.")
	assertContains(t, output, "Run `memry usage disable` to turn it off.")
	assertNotContains(t, output, "usage-secret")
	assertNotContains(t, output, "only token counts")
}

func disclosure(endpoint string) []string {
	return []string{
		"Claude Code will send its OpenTelemetry metrics to " + endpoint + ", your memry server, and nowhere else.",
		"It sends all 8 of its metrics: session count, lines of code, pull requests, commits, cost, token usage, code edit tool decisions and active time.",
		"Each one carries your email, your organization id, an anonymous user id and the terminal type; Claude Code cannot leave these out.",
		"The OTLP resource block also names the service and its version, the operating system type and version, and the host architecture.",
		"memry turns off every optional attribute Claude Code documents: the session id, the account uuid, the repository, the version, the entrypoint and custom resource attributes.",
		"It does not turn usage metrics on while any other telemetry setting is present, in the environment, the settings or the managed settings.",
		"This is what Claude Code sends today; a later version may add fields.",
		"The memry server keeps only the token and cost data points, tied to your memry account: the count or cost, the model, the token type, the query source and the time window.",
		"It discards the other metrics on receipt and does not store your email or the other attributes; it only hashes them, with the metric, into a series identity.",
		"No prompts or code are sent. If you later turn on a logs or traces exporter, the telemetry switch memry sets starts it too.",
	}
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

// Revoking the previous usage token is best effort: it stops working
// after 30 days without use anyway.
func TestEnableAgainSucceedsWithAWarningWhenThePreviousTokenCannotBeRevoked(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	s.answerRevoke(500)

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Could not revoke your previous usage token; it stops working after 30 days without use.")
	assertContains(t, output, "Usage metrics are on.")
}

// Enabling again revokes exactly the token this machine used before: a
// usage token of another machine, older or newer, survives.
func TestEnableAgainRevokesOnlyThePreviousTokenOfThisMachine(t *testing.T) {
	s := newServer(t)
	older := s.issueElsewhere()
	h := newHarness(t)
	h.login(s.URL)
	h.run(usage.Enable)
	newer := s.issueElsewhere()

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	if got, want := s.activeTokens(), []int{older, newer, newer + 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("active tokens = %v, want %v", got, want)
	}
	got := s.received()
	if last := got[len(got)-1]; last != (request{"DELETE", "/api/auth/tokens/8", "", "Bearer 1|login-token"}) {
		t.Errorf("last request = %+v, want DELETE /api/auth/tokens/8", last)
	}
}

// A telemetry setting edited while the token is being issued is never
// overwritten: enable decided on the settings it read before.
func TestEnableStopsWhenTheSettingsChangeMeanwhile(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	writeRaw(t, h.settingsPath(), `{"env":{"FOO":"bar"}}`)
	edited := `{"env":{"FOO":"bar","OTEL_METRICS_EXPORTER":"prometheus"}}`
	s.onIssue = func() { writeRaw(t, h.settingsPath(), edited) }

	output, code := h.run(usage.Enable)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not turn on usage metrics: "+h.settingsPath()+" changed while memry was turning on usage metrics, so nothing was written. Try again.")
	if got := readText(t, h.settingsPath()); got != edited {
		t.Errorf("settings = %s, want the edit kept", got)
	}
	if got := s.activeTokens(); len(got) != 0 {
		t.Errorf("active tokens = %v, want the new one revoked", got)
	}
	assertNotContains(t, readText(t, h.configPath), `"usage_metrics"`)
}
