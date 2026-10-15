package usage

import (
	"slices"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// isTelemetry reports whether key configures Claude Code's telemetry, by
// namespace, so a variable a later Claude Code adds is covered too: every
// OTEL_* variable, the telemetry switches and their aliases
// (CLAUDE_CODE_ENABLE_TELEMETRY*, CLAUDE_CODE_ENHANCED_TELEMETRY*,
// ENABLE_ENHANCED_TELEMETRY*), Claude Code's own OTel tuning
// (CLAUDE_CODE_OTEL_*) and beta tracing (*BETA_TRACING*). See
// https://code.claude.com/docs/en/monitoring-usage.
func isTelemetry(key string) bool {
	for _, prefix := range []string{"OTEL_", "CLAUDE_CODE_ENABLE_TELEMETRY", "CLAUDE_CODE_ENHANCED_TELEMETRY", "ENABLE_ENHANCED_TELEMETRY", "CLAUDE_CODE_OTEL_"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return strings.Contains(key, "BETA_TRACING")
}

// isMemrys reports whether key is one of the variables memry writes.
func isMemrys(key string) bool {
	return key == endpointKey || key == headersKey || slices.ContainsFunc(constants, func(v variable) bool { return v.key == key })
}

// allowed reports whether a telemetry variable may be set to value next to
// memry's export to endpoint: memry's own variable with memry's value, or
// an exporter of another signal set to none, which only turns it off.
// Headers are never allowed: their token is not memry's yet.
func allowed(key string, value any, endpoint string) bool {
	if (key == "OTEL_LOGS_EXPORTER" || key == "OTEL_TRACES_EXPORTER") && value == "none" {
		return true
	}
	for _, v := range append(slices.Clone(constants), variable{endpointKey, endpoint}) {
		if v.key == key {
			return value == v.value
		}
	}
	return false
}

// telemetry is the telemetry configuration Claude Code runs with: its
// launch environment, which wins, and the env of its settings.json.
type telemetry struct {
	// goos is the operating system; on Windows environment names are
	// case-insensitive.
	goos        string
	environment func(string) (string, bool)
	// environ lists the launch environment as KEY=value.
	environ  func() []string
	settings *phpjson.Object
	path     string
	// helper is the otelHeadersHelper of the settings, nil when none.
	helper any
}

// inEnvironment returns the value of key in the launch environment. Like
// the OpenTelemetry SDKs, an empty value is no value. On Windows the name
// matches whatever its case, as Claude Code reads it.
func (t telemetry) inEnvironment(key string) (string, bool) {
	if t.goos == "windows" && t.environ != nil {
		value, ok := t.environmentVars()[strings.ToUpper(key)]
		return value, ok
	}
	value, ok := t.environment(key)
	return value, ok && value != ""
}

// inSettings returns the value of key in the settings' env. A null or
// empty value is no value.
func (t telemetry) inSettings(key string) (any, bool) {
	if t.settings == nil {
		return nil, false
	}
	value, _ := t.settings.Get(key)
	return value, value != nil && value != ""
}

// helperKey is the settings key of a script that makes OTLP headers.
const helperKey = "otelHeadersHelper"

// environmentVars are the variables of the launch environment with a
// value, their names in upper case on Windows, where names are
// case-insensitive.
func (t telemetry) environmentVars() map[string]string {
	vars := map[string]string{}
	if t.environ == nil {
		return vars
	}
	for _, pair := range t.environ() {
		key, value, _ := strings.Cut(pair, "=")
		if t.goos == "windows" {
			key = strings.ToUpper(key)
		}
		if value != "" {
			vars[key] = value
		}
	}
	return vars
}

// environmentKeys are the telemetry variables of the launch environment
// with a value, in order.
func (t telemetry) environmentKeys() []string {
	var keys []string
	for key := range t.environmentVars() {
		if isTelemetry(key) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// foreign says which telemetry settings that are not memry's stand next
// to memry's export to endpoint: any telemetry variable of the launch
// environment or the settings that is not allowed (see allowed), but for
// memry's own variables in the settings, which conflicts handles, and a
// headers helper, whose headers Claude Code adds to the ones it sends.
func (t telemetry) foreign(endpoint string) []string {
	var lines []string
	for _, key := range t.environmentKeys() {
		if value, _ := t.inEnvironment(key); !isMemrys(key) && !allowed(key, value, endpoint) {
			lines = append(lines, key+" is set in the environment.")
		}
	}
	if t.settings != nil {
		for key, value := range t.settings.All() {
			if isTelemetry(key) && value != nil && value != "" && !isMemrys(key) && !allowed(key, value, endpoint) {
				lines = append(lines, key+" is set in "+t.path+".")
			}
		}
	}
	if t.helper != nil {
		lines = append(lines, helperKey+" is set in "+t.path+".")
	}
	return lines
}

// blockers says which settings stop memry from turning on usage metrics
// to endpoint, whatever --force says: memry's variables exported in the
// launch environment with another value (any headers there would replace
// memry's token), and every foreign setting (see foreign). Anything that
// is not memry's export, or does not only turn something off, is in the
// way: it may send more, or send elsewhere.
func (t telemetry) blockers(endpoint string) []string {
	var lines []string
	for _, key := range t.environmentKeys() {
		if value, _ := t.inEnvironment(key); isMemrys(key) && !allowed(key, value, endpoint) {
			lines = append(lines, key+" is set in the environment.")
		}
	}
	return append(lines, t.foreign(endpoint)...)
}

// inactive says why Claude Code does not export metrics to endpoint with
// a usage token, or nothing when it does: the switch, the exporter, the
// protocol, the endpoint and the headers, each as Claude Code reads it.
func (t telemetry) inactive(endpoint string) []string {
	var reasons []string
	for _, v := range []variable{{switchKey, "1"}, {"OTEL_METRICS_EXPORTER", "otlp"}, {"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL", "http/json"}, {endpointKey, endpoint}, {headersKey, ""}} {
		if value, ok := t.inEnvironment(v.key); ok {
			if !v.matches(value) {
				reasons = append(reasons, v.key+" is overridden in the environment.")
			}
			continue
		}
		value, ok := t.inSettings(v.key)
		text, _ := value.(string)
		switch {
		case !ok:
			reasons = append(reasons, v.key+" is missing from "+t.path+".")
		case !v.matches(text):
			reasons = append(reasons, v.key+" is not memry's value in "+t.path+".")
		}
	}
	return reasons
}

// matches reports whether value is the variable's: for the headers, which
// hold a token, any bearer token.
func (v variable) matches(value string) bool {
	if v.key == headersKey {
		token, ok := strings.CutPrefix(value, bearerPrefix)
		return ok && token != ""
	}
	return value == v.value
}
