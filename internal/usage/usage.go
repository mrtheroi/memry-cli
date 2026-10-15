// Package usage is `memry usage`: it turns on and off, on the user's
// explicit request, Claude Code's export of its token and cost metrics to
// the user's own memry server.
package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"slices"
	"unicode/utf8"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/config"
	"github.com/mrtheroi/memry-cli/internal/console"
	"github.com/mrtheroi/memry-cli/internal/flags"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Env is what the commands run with.
type Env struct {
	// GOOS is the operating system; the zero value means Unix.
	GOOS string
	// Args are the arguments after the command name.
	Args      []string
	LookupEnv func(string) (string, bool)
	Out       io.Writer
	HTTP      *http.Client
}

// Exit codes, as in the other commands.
const (
	success = 0
	failure = 1
)

// metricsPath is the OTLP metrics endpoint of a memry server.
const metricsPath = "/api/otlp/v1/metrics"

// Enable runs `memry usage enable` and returns its exit code.
func Enable(env Env) int {
	env, ok := env.start("usage enable", "--force", "--insecure")
	if !ok {
		return failure
	}
	st := env.load()
	if !st.loggedIn {
		env.line("You are not logged in to memry. Run `memry setup` first.")
		return failure
	}
	if !flags.Scan(env.Args).Insecure && flags.IsRemotePlainHTTP(st.url) {
		parsed, _ := neturl.Parse(st.url)
		env.line("Refusing to send usage metrics over plain HTTP to " + parsed.Host +
			". Use an https:// server, or pass --insecure to allow it (for example on a trusted LAN).")
		return failure
	}
	current, ok, err := st.settings.Env()
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not turn on usage metrics: " + failed)
		return failure
	}
	force := flags.HasParameterOption(env.Args, "--force")
	if inTheWay := conflicts(current, st.ours); len(inTheWay) > 0 && !force {
		for _, key := range inTheWay {
			env.line(key + " is already set to another value in " + st.settings.Path() + ".")
		}
		env.line("Pass --force to replace it, or remove it from the settings first.")
		return failure
	}
	usageToken, failed := env.usageToken(st.url, st.token)
	if failed != "" {
		env.line(failed)
		return failure
	}
	endpoint := st.url + metricsPath
	kept := st.kept(current)
	ok, err = st.settings.SetEnv(phpjson.NewObject(
		"CLAUDE_CODE_ENABLE_TELEMETRY", "1",
		"OTEL_METRICS_EXPORTER", "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL", "http/json",
		endpointKey, endpoint,
		headersKey, "Authorization=Bearer "+usageToken,
	))
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not turn on usage metrics: " + failed)
		return failure
	}
	st.cfg.SetUsageMetrics(config.UsageMetrics{Endpoint: endpoint, Kept: kept})
	if err := st.cfg.Save(); err != nil {
		env.line("Could not save what memry wrote to " + st.cfgPath + ": " + err.Error() + ".")
		return failure
	}
	env.line("Usage metrics are on.")
	env.line("Claude Code will send its token counts and cost per model to " + endpoint + ", your memry server, and nowhere else.")
	env.line("No prompts, code or file contents are sent.")
	env.line("Saved in " + st.settings.Path() + "; restart Claude Code to apply it.")
	env.line("Run `memry usage disable` to turn it off.")
	return success
}

// start checks the arguments of command, which takes only the global
// options and its own value-less options, like the other commands: on an
// error it says why (unless silent) and reports false. Quiet hides the
// output from then on.
func (env Env) start(command string, options ...string) (Env, bool) {
	args := flags.Scan(env.Args)
	if message, bad := flags.UnexpectedArgument(command, env.Args, options...); bad {
		if !flags.IsSilent(args, env.LookupEnv) {
			console.ErrorBlock(env.Out, message)
		}
		return env, false
	}
	if flags.IsQuiet(args, env.LookupEnv) {
		env.Out = io.Discard
	}
	return env, true
}

// state is memry's login and usage metrics setup, and Claude Code's
// settings.
type state struct {
	cfgPath string
	cfg     *config.File
	// url and token are the login, when loggedIn.
	url, token string
	loggedIn   bool
	// record is what enable saved, when hasRecord.
	record    config.UsageMetrics
	hasRecord bool
	settings  *agentfiles.ClaudeSettings
}

// load reads the config file, which like the other commands is empty when
// it is missing or cannot be read as a JSON object.
func (env Env) load() state {
	st := state{settings: agentfiles.NewClaudeSettings(agents.ClaudeSettingsPath(env.GOOS, env.getenv))}
	st.cfgPath, _ = config.Path(env.GOOS, env.getenv)
	cfg, err := config.LoadOrEmpty(st.cfgPath)
	if err != nil {
		cfg = &config.File{}
	}
	st.cfg = cfg
	url, hasURL := cfg.URL()
	token, hasToken := cfg.Token()
	st.url, st.token, st.loggedIn = url, token, hasURL && url != "" && hasToken && token != ""
	st.record, st.hasRecord = cfg.UsageMetrics()
	return st
}

// ours reports whether endpoint is memry's: the one enable saved, or the
// one of the server memry is logged in to.
func (st state) ours(endpoint any) bool {
	return (st.hasRecord && endpoint == st.record.Endpoint) || (st.url != "" && endpoint == st.url+metricsPath)
}

// kept returns the constants of current that memry did not write: the
// ones enable kept before, when memry's export is already there, else the
// ones that already have memry's value.
func (st state) kept(current *phpjson.Object) []string {
	if current == nil {
		return nil
	}
	if endpoint, _ := current.Get(endpointKey); st.ours(endpoint) {
		return st.record.Kept
	}
	kept := []string{}
	for _, c := range constants {
		if value, _ := current.Get(c.key); value == c.value {
			kept = append(kept, c.key)
		}
	}
	return kept
}

// settingsFailure says why the settings could not be read or written, or
// nothing when they could.
func settingsFailure(settings *agentfiles.ClaudeSettings, ok bool, err error) string {
	switch {
	case err != nil:
		return err.Error() + "."
	case !ok:
		return settings.Path() + ` is not valid JSON or its "env" is not an object. Fix the file, then try again.`
	}
	return ""
}

// constants are the variables memry sets to the same value for everyone.
var constants = []struct{ key, value string }{
	{"CLAUDE_CODE_ENABLE_TELEMETRY", "1"},
	{"OTEL_METRICS_EXPORTER", "otlp"},
	{"OTEL_EXPORTER_OTLP_PROTOCOL", "http/json"},
}

// Variables memry writes besides the constants.
const (
	endpointKey = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
	headersKey  = "OTEL_EXPORTER_OTLP_METRICS_HEADERS"
)

// conflicts returns the variables of current in the way of memry's export
// to endpoint: the ones memry would replace that hold another value it did
// not write, and the generic OTLP endpoint and headers, which would send
// the user's other telemetry elsewhere or their headers to memry. memry's
// own export is recognized by its endpoint (see ours); the headers next to
// it are memry's too, with the token of an earlier enable.
func conflicts(current *phpjson.Object, ours func(any) bool) []string {
	if current == nil {
		return nil
	}
	var inTheWay []string
	has := func(key string) (any, bool) {
		value, ok := current.Get(key)
		return value, ok && value != nil
	}
	for _, c := range constants {
		if value, ok := has(c.key); ok && value != c.value {
			inTheWay = append(inTheWay, c.key)
		}
	}
	value, hasEndpoint := has(endpointKey)
	isOurs := hasEndpoint && ours(value)
	if hasEndpoint && !isOurs {
		inTheWay = append(inTheWay, endpointKey)
	}
	if _, ok := has(headersKey); ok && !isOurs {
		inTheWay = append(inTheWay, headersKey)
	}
	if value, ok := has("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"); ok && value != "http/json" {
		inTheWay = append(inTheWay, "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL")
	}
	for _, key := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS"} {
		if _, ok := has(key); ok {
			inTheWay = append(inTheWay, key)
		}
	}
	return inTheWay
}

// usageToken asks the memry server at url for the user's usage token,
// which replaces any previous one. It returns it, or why it failed.
func (env Env) usageToken(url, token string) (usageToken, failed string) {
	unreachable := "Could not reach the memry server at " + url + "."
	req, err := http.NewRequest(http.MethodPost, url+"/api/auth/usage-token", nil)
	if err != nil {
		return "", unreachable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := env.HTTP.Do(req)
	if err != nil {
		return "", unreachable
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := client.ReadBody(resp.Body, maxResponseBytes)
	if err != nil {
		return "", unreachable
	}
	var body struct {
		Token   string `json:"token"`
		Message string `json:"message"`
	}
	decoded := utf8.Valid(payload) && json.Unmarshal(payload, &body) == nil
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", "Your memry login is no longer valid. Run `memry setup` and try again."
	case resp.StatusCode == http.StatusTooManyRequests:
		return "", "Too many attempts, try again later."
	case !client.Succeeded(resp) && decoded && body.Message != "":
		return "", body.Message
	case !client.Succeeded(resp):
		return "", fmt.Sprintf("The memry server returned an unexpected error (HTTP %d).", resp.StatusCode)
	case !decoded || body.Token == "":
		return "", "The memry server did not return a usage token."
	}
	return body.Token, ""
}

// maxResponseBytes bounds the response read, so a misbehaving server
// cannot make memry buffer an unbounded body. The real one is tiny.
const maxResponseBytes = 1 << 20

// line prints one line of output.
func (env Env) line(text string) {
	_, _ = io.WriteString(env.Out, text+"\n")
}

func (env Env) getenv(key string) string {
	value, _ := env.LookupEnv(key)
	return value
}

// Disable runs `memry usage disable` and returns its exit code.
func Disable(env Env) int {
	env, ok := env.start("usage disable")
	if !ok {
		return failure
	}
	st := env.load()
	current, ok, err := st.settings.Env()
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not turn off usage metrics: " + failed)
		return failure
	}
	keys := st.written(current)
	if len(keys) == 0 {
		env.line("Usage metrics are not on; nothing to remove.")
		return success
	}
	removed, err := st.settings.RemoveEnv(keys)
	if failed := settingsFailure(st.settings, removed != agentfiles.Unsafe, err); failed != "" {
		env.line("Could not turn off usage metrics: " + failed)
		return failure
	}
	if st.hasRecord {
		st.cfg.DeleteUsageMetrics()
		if err := st.cfg.Save(); err != nil {
			env.line("Could not update " + st.cfgPath + ": " + err.Error() + ".")
			return failure
		}
	}
	env.line("Usage metrics are off. Removed them from " + st.settings.Path() + "; restart Claude Code to apply it.")
	return success
}

// written returns the variables of current that memry wrote: none unless
// its endpoint is there, else the endpoint, the headers next to it and
// the constants that still have memry's value, except the ones it kept.
func (st state) written(current *phpjson.Object) []string {
	if current == nil {
		return nil
	}
	if endpoint, _ := current.Get(endpointKey); !st.ours(endpoint) {
		return nil
	}
	keys := []string{endpointKey, headersKey}
	for _, c := range constants {
		if value, _ := current.Get(c.key); value == c.value && !slices.Contains(st.record.Kept, c.key) {
			keys = append(keys, c.key)
		}
	}
	return keys
}

// Status runs `memry usage status` and returns its exit code. It never
// prints the usage token.
func Status(env Env) int {
	env, ok := env.start("usage status")
	if !ok {
		return failure
	}
	st := env.load()
	current, ok, err := st.settings.Env()
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not read the usage metrics setting: " + failed)
		return failure
	}
	if len(st.written(current)) == 0 {
		env.line("Usage metrics are off.")
		return success
	}
	endpoint, _ := current.Get(endpointKey)
	env.line(fmt.Sprintf("Usage metrics are on: Claude Code sends its token counts and cost per model to %s.", endpoint))
	env.line("Set in " + st.settings.Path() + ".")
	return success
}
