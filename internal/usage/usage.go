// Package usage is `memry usage`: it turns on and off, on the user's
// explicit request, Claude Code's export of its token and cost metrics to
// the user's own memry server.
package usage

import (
	"io"
	"net/http"
	neturl "net/url"
	"slices"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/agents"
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
	if st.settings.Path() != st.activePath {
		env.line("Usage metrics are on in " + st.settings.Path() + ". Run `memry usage disable` first to move them to " + st.activePath + ".")
		return failure
	}
	current, ok, err := st.settings.Env()
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not turn on usage metrics: " + failed)
		return failure
	}
	model := telemetry{environment: env.LookupEnv, settings: current, path: st.settings.Path()}
	if blockers := model.blockers(st.url+metricsPath, model.value(switchKey) == "1" && (model.where(switchKey) == "the environment" || slices.Contains(st.kept(current, st.url+metricsPath), switchKey))); len(blockers) > 0 {
		for _, line := range blockers {
			env.line(line)
		}
		env.line(pronoun(len(blockers), "Remove it, then try again; --force does not override it.", "Remove them, then try again; --force does not override them."))
		return failure
	}
	force := flags.HasParameterOption(env.Args, "--force")
	if inTheWay := st.conflicts(current, st.url+metricsPath); len(inTheWay) > 0 && !force {
		for _, key := range inTheWay {
			env.line(key + " is already set to another value in " + st.settings.Path() + ".")
		}
		env.line("Pass --force to replace it, or remove it from the settings first.")
		return failure
	}
	for _, line := range disclosure(st.url + metricsPath) {
		env.line(line)
	}
	issued, failed := env.issueToken(st.url, st.token)
	if failed != "" {
		env.line(failed)
		return failure
	}
	usageToken := issued.Token
	endpoint := st.url + metricsPath
	kept := st.kept(current, endpoint)
	vars := phpjson.NewObject()
	values := map[string]string{}
	for _, v := range append(wanted(endpoint), variable{headersKey, bearerPrefix + usageToken}) {
		if !slices.Contains(kept, v.key) {
			vars.Set(v.key, v.value)
			values[v.key] = v.value
		}
	}
	// The record goes first: settings memry has no record of could never
	// be turned off.
	st.cfg.SetUsageMetrics(config.UsageMetrics{SettingsPath: st.settings.Path(), Values: values, Kept: kept, TokenID: issued.ID})
	if err := st.cfg.Save(); err != nil {
		env.revokeNew(st, issued.ID)
		env.line("Could not save the usage metrics record to " + st.cfgPath + ": " + err.Error() + ". Nothing was changed.")
		return failure
	}
	ok, err = st.settings.SetEnv(vars)
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not turn on usage metrics: " + failed)
		env.restore(st)
		env.revokeNew(st, issued.ID)
		return failure
	}
	// Only now that the new token is in use is the one this machine used
	// before revoked; other machines' tokens are never touched.
	if st.record.TokenID != 0 && !env.revokeToken(st.url, st.token, st.record.TokenID) {
		env.line("Could not revoke your previous usage token; it stops working after 30 days without use.")
	}
	env.line("Usage metrics are on. Saved in " + st.settings.Path() + "; restart Claude Code to apply it.")
	env.line("Run `memry usage disable` to turn it off.")
	return success
}

// revokeNew revokes the usage token id enable was just issued and could
// not put to use, best effort: unused, it stops working after 30 days.
func (env Env) revokeNew(st state, id int64) {
	if !env.revokeToken(st.url, st.token, id) {
		env.line("Could not revoke the new usage token; it stops working after 30 days without use.")
	}
}

// restore puts back the usage metrics record st was loaded with, after the
// settings could not be written: the settings still hold what it says.
func (env Env) restore(st state) {
	if st.hasRecord {
		st.cfg.SetUsageMetrics(st.record)
	} else {
		st.cfg.DeleteUsageMetrics()
	}
	if err := st.cfg.Save(); err != nil {
		env.line("Could not restore the usage metrics record in " + st.cfgPath + ": " + err.Error() + ".")
	}
}

// disclosure tells what Claude Code sends to endpoint once usage metrics
// are on, and what the memry server keeps of it, before memry asks for a
// usage token. Keep it in step with PRIVACY.md and the server.
func disclosure(endpoint string) []string {
	return []string{
		"Claude Code will send its OpenTelemetry metrics to " + endpoint + ", your memry server, and nowhere else.",
		"It sends all 8 of its metrics: session count, lines of code, pull requests, commits, cost, token usage, code edit tool decisions and active time.",
		"Each one carries your email, your organization id, an anonymous user id and the terminal type; Claude Code cannot leave these out.",
		"memry turns off the session id, the account uuid and the resource attributes.",
		"The memry server keeps only the token and cost data points, tied to your memry account: the count or cost, the model, the token type, the query source and the time window.",
		"It discards the other metrics on receipt and does not store your email or the other attributes; it only hashes them, with the metric, into a series identity.",
		"No prompts or code are sent. If you later turn on a logs or traces exporter, the telemetry switch memry sets starts it too.",
	}
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
	// record is what enable wrote, when hasRecord: the single source of
	// what is memry's.
	record    config.UsageMetrics
	hasRecord bool
	// settings are the ones enable wrote, when there is a record, else
	// the ones Claude Code reads now (activePath).
	settings   *agentfiles.ClaudeSettings
	activePath string
}

// load reads the config file, which like the other commands is empty when
// it is missing or cannot be read as a JSON object.
func (env Env) load() state {
	st := state{activePath: agents.ClaudeSettingsPath(env.GOOS, env.getenv)}
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
	path := st.activePath
	if st.hasRecord {
		path = st.record.SettingsPath
	}
	st.settings = agentfiles.NewClaudeSettings(path)
	return st
}

// owned reports whether key of current is memry's: it is there with
// exactly the value memry recorded writing.
func (st state) owned(current *phpjson.Object, key string) bool {
	recorded, ok := st.record.Values[key]
	if !st.hasRecord || !ok || current == nil {
		return false
	}
	value, _ := current.Get(key)
	return value == recorded
}

// wanted are the variables memry sets for its export to endpoint, but for
// the headers, which hold a usage token not issued yet.
func wanted(endpoint string) []variable {
	return append(slices.Clone(constants), variable{endpointKey, endpoint})
}

// kept returns the variables of current that memry does not write because
// they already have memry's value and are not memry's: the ones the record
// kept since the first enable, while they still have it, else, on the
// first enable, the ones that have it now.
func (st state) kept(current *phpjson.Object, endpoint string) []string {
	kept := []string{}
	if current == nil {
		return kept
	}
	for _, v := range wanted(endpoint) {
		value, _ := current.Get(v.key)
		if value != v.value {
			continue
		}
		if !st.hasRecord || slices.Contains(st.record.Kept, v.key) {
			kept = append(kept, v.key)
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

// variable is an environment variable memry sets, and its value.
type variable struct{ key, value string }

// constants are the variables memry sets to the same value for everyone.
var constants = []variable{
	{switchKey, "1"},
	{"OTEL_METRICS_EXPORTER", "otlp"},
	{"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL", "http/json"},
	// Leave out of every metric what memry has no use for.
	{"OTEL_METRICS_INCLUDE_SESSION_ID", "false"},
	{"OTEL_METRICS_INCLUDE_ACCOUNT_UUID", "false"},
	{"OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES", "false"},
}

// Variables memry writes besides the constants.
const (
	// switchKey turns on Claude Code's telemetry, every signal of it.
	switchKey   = "CLAUDE_CODE_ENABLE_TELEMETRY"
	endpointKey = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
	headersKey  = "OTEL_EXPORTER_OTLP_METRICS_HEADERS"
	// bearerPrefix starts the headers, before the usage token.
	bearerPrefix = "Authorization=Bearer "
)

// conflicts returns memry's variables in the settings that hold a value
// memry did not write (see owned) and is not memry's either, so enable
// would replace it: the user's.
func (st state) conflicts(current *phpjson.Object, endpoint string) []string {
	if current == nil {
		return nil
	}
	var inTheWay []string
	for _, v := range append(wanted(endpoint), variable{headersKey, ""}) {
		value, _ := current.Get(v.key)
		if value != nil && value != v.value && !st.owned(current, v.key) {
			inTheWay = append(inTheWay, v.key)
		}
	}
	return inTheWay
}

// pronoun is one when there is a single thing, else many.
func pronoun(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

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
	removed, ok := env.turnOff(true)
	switch {
	case !ok:
		return failure
	case !removed:
		env.line("Usage metrics are not on; nothing to remove.")
	}
	return success
}

// TurnOff turns off the usage metrics memry set up, saying so, and says
// nothing when they are not on. With revoke it revokes the usage token it
// recorded too (not needed once the account is deleted). It reports false, after
// telling why, when they could not be removed; the record then stays for
// a retry.
func TurnOff(env Env, revoke bool) bool {
	_, ok := env.turnOff(revoke)
	return ok
}

// turnOff removes what the record says enable wrote, from the settings it
// wrote, then, with revoke, revokes the usage token it recorded (best
// effort: it stops working after 30 days without use), and forgets the
// record. It
// reports whether there was anything to remove and whether that worked.
func (env Env) turnOff(revoke bool) (removed, ok bool) {
	st := env.load()
	if !st.hasRecord {
		return false, true
	}
	current, ok, err := st.settings.Env()
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not turn off usage metrics: " + failed)
		return false, false
	}
	keys := st.written(current)
	if len(keys) > 0 {
		removal, err := st.settings.RemoveEnv(keys)
		if failed := settingsFailure(st.settings, removal != agentfiles.Unsafe, err); failed != "" {
			env.line("Could not turn off usage metrics: " + failed)
			return false, false
		}
	}
	if revoke && st.record.TokenID != 0 && (!st.loggedIn || !env.revokeToken(st.url, st.token, st.record.TokenID)) {
		env.line("Could not revoke your usage token; it stops working after 30 days without use.")
	}
	st.cfg.DeleteUsageMetrics()
	if err := st.cfg.Save(); err != nil {
		env.line("Could not update " + st.cfgPath + ": " + err.Error() + ".")
		return len(keys) > 0, false
	}
	if len(keys) > 0 {
		env.line("Usage metrics are off. Removed them from " + st.settings.Path() + "; restart Claude Code to apply it.")
	}
	return len(keys) > 0, true
}

// written returns the variables of current that are memry's (see owned),
// in a stable order.
func (st state) written(current *phpjson.Object) []string {
	var keys []string
	for key := range st.record.Values {
		if st.owned(current, key) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// Status runs `memry usage status` and returns its exit code. Usage
// metrics are on only when what Claude Code runs with, its launch
// environment first, exports them to memry. It never prints the usage
// token, nor any value but memry's endpoint.
func Status(env Env) int {
	env, ok := env.start("usage status")
	if !ok {
		return failure
	}
	st := env.load()
	if !st.hasRecord {
		env.line("Usage metrics are off.")
		return success
	}
	current, ok, err := st.settings.Env()
	if failed := settingsFailure(st.settings, ok, err); failed != "" {
		env.line("Could not read the usage metrics setting: " + failed)
		return failure
	}
	endpoint := st.record.Values[endpointKey]
	if endpoint == "" {
		endpoint = st.url + metricsPath
	}
	model := telemetry{environment: env.LookupEnv, settings: current, path: st.settings.Path()}
	if reasons := model.inactive(endpoint); len(reasons) > 0 {
		env.line("Usage metrics are not active:")
		for _, reason := range reasons {
			env.line("- " + reason)
		}
		return success
	}
	env.line("Usage metrics are on: Claude Code sends its metrics to " + endpoint + ".")
	env.line("Set in " + st.settings.Path() + ".")
	return success
}
