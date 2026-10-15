// Package usage is `memry usage`: it turns on and off, on the user's
// explicit request, Claude Code's export of its token and cost metrics to
// the user's own memry server.
package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	// Environ lists the environment as KEY=value, like os.Environ.
	Environ func() []string
	Out     io.Writer
	HTTP    *http.Client
	// CacheDir is the user's cache directory (os.UserCacheDir), where
	// the usage lock lives.
	CacheDir string
	// ManagedDir is the directory of Claude Code's managed settings
	// files (see ManagedSettingsDir); empty, there are none to read.
	ManagedDir string
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
	return env.locked(env.enable)
}

// enable is Enable once the usage lock is held.
func (env Env) enable() int {
	st := env.reconcile(env.load())
	if env.unknown(st) {
		return failure
	}
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
	if st.hasRecord && !st.sameServer() {
		env.line("Usage metrics are on for " + st.record.ServerURL + ". Run `memry usage disable` before enabling for " + st.url + ".")
		return failure
	}
	if st.settings.Path() != st.activePath {
		env.line("Usage metrics are on in " + st.settings.Path() + ". Run `memry usage disable` first to move them to " + st.activePath + ".")
		return failure
	}
	// Everything below is decided on this read of the settings, which the
	// write later compares the file with, byte for byte.
	snapshot, err := st.settings.Read()
	if failed := settingsFailure(st.settings, snapshot.OK(), err, "turning on"); failed != "" {
		env.line("Could not turn on usage metrics: " + failed)
		return failure
	}
	current := snapshot.Env()
	model := telemetry{environment: env.LookupEnv, environ: env.Environ, settings: current, path: st.settings.Path(), helper: snapshot.Setting(helperKey)}
	blockers := model.blockers(st.url + metricsPath)
	blockers = append(blockers, managedBlockers(env.ManagedDir, st.url+metricsPath)...)
	if len(blockers) > 0 {
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
	// The record goes first, in two phases: the new setup is pending, next
	// to the current one, until the settings hold it. Whatever stops
	// enable, the next usage command can tell which one the settings got.
	setup := config.UsageSetup{ServerURL: normalizeServer(st.url), SettingsPath: st.settings.Path(), Values: values, Kept: kept, TokenID: issued.ID}
	staged := config.UsageMetrics{Pending: &setup}
	if st.hasRecord {
		staged.Current = &st.record
	}
	st.cfg.SetUsageMetrics(staged)
	if err := st.cfg.Save(); err != nil {
		env.revokeNew(st, issued.ID)
		env.line("Could not save the usage metrics record to " + st.cfgPath + ": " + err.Error() + ". Nothing was changed.")
		return failure
	}
	beforeSettingsWrite()
	// Written only if the file is still the one enable decided on above,
	// before the token was issued.
	ok, err := st.settings.SetEnvFrom(snapshot, vars)
	if failed := settingsFailure(st.settings, ok, err, "turning on"); failed != "" {
		env.line("Could not turn on usage metrics: " + failed)
		env.restore(st, issued.ID)
		env.revokeNew(st, issued.ID)
		return failure
	}
	st.cfg.SetUsageMetrics(config.UsageMetrics{Current: &setup})
	switch err := st.cfg.Save(); {
	case err != nil:
		// The settings hold the new setup: the next usage command finds
		// it pending and makes it current.
		env.line("Could not update the usage metrics record in " + st.cfgPath + ": " + err.Error() + ". The next `memry usage` command finishes it.")
	case st.record.TokenID != 0 && !env.revokeToken(st.record.ServerURL, st.token, st.record.TokenID):
		// Only now that the new token is in use is the one this machine
		// used before revoked; other machines' tokens are never touched.
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
// settings could not be written: the settings still hold what it says. It
// reads the record again first and restores only while the pending setup
// is still this enable's, the one with token id, so it never undoes what
// another invocation saved.
func (env Env) restore(st state, id int64) {
	cfg, err := config.Load(st.cfgPath)
	if err != nil {
		env.line("Could not restore the usage metrics record in " + st.cfgPath + ": " + err.Error() + ".")
		return
	}
	if usage, _ := cfg.UsageMetrics(); usage.Pending == nil || usage.Pending.TokenID != id {
		return
	}
	if st.hasRecord {
		cfg.SetUsageMetrics(config.UsageMetrics{Current: &st.record})
	} else {
		cfg.DeleteUsageMetrics()
	}
	if err := cfg.Save(); err != nil {
		env.line("Could not restore the usage metrics record in " + st.cfgPath + ": " + err.Error() + ".")
	}
}

// disclosure tells, before memry asks for a usage token, where Claude Code
// sends its metrics once usage metrics are on, what the memry server keeps
// of them, and the scope of what memry checked. Claude Code decides what
// it sends; the guarantee is what the server keeps. Keep it in step with
// PRIVACY.md and the server.
func disclosure(endpoint string) []string {
	return []string{
		"Claude Code will send its OpenTelemetry metrics to " + endpoint + ", your memry server.",
		"What it sends is up to Claude Code, as documented at https://code.claude.com/docs/en/monitoring-usage: metrics such as token usage, cost, sessions and lines of code (not an exhaustive list).",
		"They include identity attributes, such as your email and organization id, and metric-specific attributes. memry turns off the optional attribute gates it can (OTEL_METRICS_INCLUDE_*).",
		"The memry server keeps only the token and cost data points (the count or cost, the model, the token type, the query source and the time window) and discards everything else on receipt.",
		"memry checked this shell's environment, your settings.json and the managed settings files. It cannot see per-session --settings, managed policy helpers, MDM, the registry, server-managed settings or the environment Claude Code is launched from; those can change what Claude Code sends, or where.",
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
	record    config.UsageSetup
	hasRecord bool
	// pending is a setup an interrupted enable may have written.
	pending *config.UsageSetup
	// settings are the ones enable wrote, when there is a record, else
	// the ones Claude Code reads now (activePath).
	settings   *agentfiles.ClaudeSettings
	activePath string
	// unreadable is why the config could not be read, if it could not:
	// then nothing is known about the usage metrics.
	unreadable error
}

// load reads the config file, which like the other commands is empty when
// it is missing or cannot be read as a JSON object.
func (env Env) load() state {
	st := state{activePath: agents.ClaudeSettingsPath(env.GOOS, env.getenv)}
	st.cfgPath, _ = config.Path(env.GOOS, env.getenv)
	// Unlike the other commands, a config that cannot be read as a JSON
	// object is no empty config here: it may hold the usage record.
	// Like the other commands, anything but a regular file is no config.
	cfg, err := config.Load(st.cfgPath)
	if info, statErr := os.Stat(st.cfgPath); statErr == nil && !info.Mode().IsRegular() {
		cfg, err = &config.File{}, nil
	}
	if err != nil {
		st.unreadable = err
		cfg = &config.File{}
	}
	st.cfg = cfg
	url, hasURL := cfg.URL()
	token, hasToken := cfg.Token()
	st.url, st.token, st.loggedIn = url, token, hasURL && url != "" && hasToken && token != ""
	usage, _ := cfg.UsageMetrics()
	if usage.Current != nil {
		st.record, st.hasRecord = *usage.Current, true
	}
	st.pending = usage.Pending
	path := st.activePath
	if setups := st.setups(); len(setups) > 0 {
		path = setups[0].SettingsPath
	}
	st.settings = agentfiles.NewClaudeSettings(path)
	return st
}

// unknown reports, after saying so, whether the config could not be read,
// so the usage metrics state is unknown and nothing may be done.
func (env Env) unknown(st state) bool {
	if st.unreadable == nil {
		return false
	}
	env.line("Could not read the memry config " + st.cfgPath + ", so memry cannot tell whether usage metrics are on. Fix it, then try again.")
	return true
}

// reconcile settles a pending setup an interrupted enable left, then
// loads the state again. The pending setup is current when the settings
// it names hold its headers, so the write happened: the previous token is
// revoked. Otherwise it never happened: the pending setup is dropped and
// its token revoked. Settings it cannot read leave it for later.
func (env Env) reconcile(st state) state {
	if st.pending == nil {
		return st
	}
	pending := *st.pending
	snapshot, err := agentfiles.NewClaudeSettings(pending.SettingsPath).Read()
	if err != nil || !snapshot.OK() {
		return st
	}
	var headers any
	if current := snapshot.Env(); current != nil {
		headers, _ = current.Get(headersKey)
	}
	usage, revoke := config.UsageMetrics{Current: &pending}, st.record
	if headers != pending.Values[headersKey] {
		usage, revoke = config.UsageMetrics{}, pending
		if st.hasRecord {
			usage.Current = &st.record
		}
	}
	if usage.Current == nil {
		st.cfg.DeleteUsageMetrics()
	} else {
		st.cfg.SetUsageMetrics(usage)
	}
	if err := st.cfg.Save(); err != nil {
		return st
	}
	if revoke.TokenID != 0 && st.loggedIn && normalizeServer(st.url) == revoke.ServerURL {
		_ = env.revokeToken(revoke.ServerURL, st.token, revoke.TokenID)
	}
	return env.load()
}

// sameServer reports whether memry is logged in to the server the
// recorded usage token belongs to: only that server can revoke it.
func (st state) sameServer() bool {
	return st.loggedIn && normalizeServer(st.url) == st.record.ServerURL
}

// normalizeServer is a server URL with its scheme and host in lower case
// and no trailing slash, so two spellings of one server compare equal.
func normalizeServer(raw string) string {
	parsed, err := neturl.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return raw
	}
	parsed.Scheme, parsed.Host = strings.ToLower(parsed.Scheme), strings.ToLower(parsed.Host)
	return parsed.String()
}

// owned reports whether key of current is memry's: it is there with
// exactly the value memry recorded writing, in the current setup or in a
// pending one not settled yet.
func (st state) owned(current *phpjson.Object, key string) bool {
	if current == nil {
		return false
	}
	value, _ := current.Get(key)
	for _, setup := range st.setups() {
		if recorded, ok := setup.Values[key]; ok && value == recorded {
			return true
		}
	}
	return false
}

// setups are the recorded setups: the current one and a pending one.
func (st state) setups() []config.UsageSetup {
	var setups []config.UsageSetup
	if st.hasRecord {
		setups = append(setups, st.record)
	}
	if st.pending != nil {
		setups = append(setups, *st.pending)
	}
	return setups
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
func settingsFailure(settings *agentfiles.ClaudeSettings, ok bool, err error, doing string) string {
	switch {
	case errors.Is(err, agentfiles.ErrChanged):
		return settings.Path() + " changed while memry was " + doing + " usage metrics, so nothing was written. Try again."
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
	// Leave out of every metric what memry has no use for: every
	// documented OTEL_METRICS_INCLUDE_* gate, off
	// (https://code.claude.com/docs/en/monitoring-usage#metrics-cardinality-control).
	{"OTEL_METRICS_INCLUDE_SESSION_ID", "false"},
	{"OTEL_METRICS_INCLUDE_ACCOUNT_UUID", "false"},
	{"OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES", "false"},
	{"OTEL_METRICS_INCLUDE_REPOSITORY", "false"},
	{"OTEL_METRICS_INCLUDE_VERSION", "false"},
	{"OTEL_METRICS_INCLUDE_ENTRYPOINT", "false"},
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
	return env.locked(env.disable)
}

// disable is Disable once the usage lock is held.
func (env Env) disable() int {
	removed, ok := env.turnOff(false)
	switch {
	case !ok:
		return failure
	case !removed:
		env.line("Usage metrics are not on; nothing to remove.")
	}
	return success
}

// TurnOff turns off the usage metrics memry set up, saying so, and says
// nothing when they are not on. It revokes the usage token it recorded
// too, unless accountDeleted on the recorded server, which revoked it with
// the account. It reports false, after telling why, when they could not be
// removed; the record then stays for a retry.
func TurnOff(env Env, accountDeleted bool) bool {
	return env.locked(func() int {
		if _, ok := env.turnOff(accountDeleted); !ok {
			return failure
		}
		return success
	}) == success
}

// turnOff removes what the record says enable wrote, from the settings it
// wrote, then, with revoke, revokes the usage token it recorded (best
// effort: it stops working after 30 days without use), and forgets the
// record. It
// reports whether there was anything to remove and whether that worked.
func (env Env) turnOff(accountDeleted bool) (removed, ok bool) {
	st := env.reconcile(env.load())
	if env.unknown(st) {
		return false, false
	}
	if len(st.setups()) == 0 {
		return false, true
	}
	snapshot, err := st.settings.Read()
	if failed := settingsFailure(st.settings, snapshot.OK(), err, "turning off"); failed != "" {
		env.line("Could not turn off usage metrics: " + failed)
		return false, false
	}
	keys := st.written(snapshot.Env())
	if len(keys) > 0 {
		beforeSettingsWrite()
		removal, err := st.settings.RemoveEnvFrom(snapshot, keys)
		if failed := settingsFailure(st.settings, removal != agentfiles.Unsafe, err, "turning off"); failed != "" {
			env.line("Could not turn off usage metrics: " + failed)
			return false, false
		}
	}
	switch {
	// Only the recorded server knows the token: memry revokes it there,
	// with the login, when that is the server it is logged in to.
	case st.record.TokenID == 0:
	case accountDeleted && st.sameServer():
		// Deleted with the account.
	case accountDeleted:
		env.line("The usage token on " + st.record.ServerURL + " stays valid until it goes 30 days without use.")
	case st.loggedIn && !st.sameServer():
		// Its id means nothing to another server.
		env.line("Did not revoke the usage token of " + st.record.ServerURL + ": you are logged in to another server now. It stops working after 30 days without use.")
	case !st.loggedIn || !env.revokeToken(st.record.ServerURL, st.token, st.record.TokenID):
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
	for _, setup := range st.setups() {
		for key := range setup.Values {
			if st.owned(current, key) && !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
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
	return env.locked(env.status)
}

// status is Status once the usage lock is held.
func (env Env) status() int {
	st := env.reconcile(env.load())
	if env.unknown(st) {
		return failure
	}
	if !st.hasRecord {
		env.line("Usage metrics are off.")
		return success
	}
	snapshot, err := st.settings.Read()
	if failed := settingsFailure(st.settings, snapshot.OK(), err, "reading"); failed != "" {
		env.line("Could not read the usage metrics setting: " + failed)
		return failure
	}
	current := snapshot.Env()
	// The recorded server is the one the token and endpoint belong to,
	// even when the endpoint was kept (already set by the user).
	endpoint := st.record.Values[endpointKey]
	if endpoint == "" {
		endpoint = st.record.ServerURL + metricsPath
	}
	model := telemetry{environment: env.LookupEnv, environ: env.Environ, settings: current, path: st.settings.Path(), helper: snapshot.Setting(helperKey)}
	reasons := append(model.inactive(endpoint), model.foreign(endpoint)...)
	if reasons = append(reasons, managedBlockers(env.ManagedDir, endpoint)...); len(reasons) > 0 {
		env.line("Usage metrics are not active:")
		for _, reason := range reasons {
			env.line("- " + reason)
		}
		return success
	}
	env.line("Usage metrics are on, as far as memry can see: Claude Code sends its metrics to " + endpoint + ".")
	env.line("Set in " + st.settings.Path() + ". Policies memry cannot read, like server-managed settings or an MDM profile, may still override it.")
	return success
}

// beforeSettingsWrite runs right before enable writes the settings; a
// seam for the tests.
var beforeSettingsWrite = func() {}

// lockPath is the usage lock of the memry config: in memry's directory
// of the user's cache directory, which other users cannot write to, named
// after the config's absolute path, so the lock works even where the
// config's own directory is read-only, and two configs never share one.
func (env Env) lockPath() string {
	path, _ := config.Path(env.GOOS, env.getenv)
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	sum := sha256.Sum256([]byte(path))
	return filepath.Join(env.CacheDir, "memry", "memry-usage-"+hex.EncodeToString(sum[:8])+".lock")
}
