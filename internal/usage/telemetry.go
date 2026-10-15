package usage

import (
	"slices"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// genericKeys are the OTLP settings shared by every signal. memry never
// writes them: a generic endpoint or protocol would redirect the user's
// other telemetry, and Claude Code merges generic headers (often another
// collector's credentials) into the metrics headers sent to memry.
var genericKeys = []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_PROTOCOL"}

// resourceKey adds custom attributes, like a team or a cost center, to the
// OTLP resource block, which OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES=false
// does not stop: memry never sends them.
const resourceKey = "OTEL_RESOURCE_ATTRIBUTES"

// telemetry is the telemetry configuration Claude Code runs with: its
// launch environment, which wins, and the env of its settings.json.
type telemetry struct {
	environment func(string) (string, bool)
	settings    *phpjson.Object
	path        string
	// helper is the otelHeadersHelper of the settings, nil when none.
	helper any
}

// inEnvironment returns the value of key in the launch environment. Like
// the OpenTelemetry SDKs, an empty value is no value.
func (t telemetry) inEnvironment(key string) (string, bool) {
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

// where says where key is set, the environment first, or "" when it is
// not set.
func (t telemetry) where(key string) string {
	if _, ok := t.inEnvironment(key); ok {
		return "the environment"
	}
	if _, ok := t.inSettings(key); ok {
		return t.path
	}
	return ""
}

// helperKey is the settings key of a script that makes OTLP headers.
const helperKey = "otelHeadersHelper"

// otherSignals are the exporters of the signals besides metrics. Each is
// started by CLAUDE_CODE_ENABLE_TELEMETRY.
var otherSignals = []string{"OTEL_LOGS_EXPORTER", "OTEL_TRACES_EXPORTER"}

// value is the value of key Claude Code runs with, the launch environment
// first, or "" when it is not set (or not a string).
func (t telemetry) value(key string) string {
	if value, ok := t.inEnvironment(key); ok {
		return value
	}
	value, _ := t.inSettings(key)
	text, _ := value.(string)
	return text
}

// foreign says which settings memry does not write would redirect its
// export or add to what it sends: the generic OTLP settings and custom
// resource attributes, wherever they are set, and a headers helper, whose
// headers Claude Code adds to the ones it sends.
func (t telemetry) foreign() []string {
	var lines []string
	for _, key := range append(slices.Clone(genericKeys), resourceKey) {
		if where := t.where(key); where != "" {
			lines = append(lines, key+" is set in "+where+".")
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
// memry's token), the foreign settings (see foreign), and,
// unless the user turned telemetry on themselves (userOn), the exporters
// of other signals that memry's switch would start.
func (t telemetry) blockers(endpoint string, userOn bool) []string {
	var lines []string
	// No value is memry's usage token, which is not issued yet.
	wanted := append(slices.Clone(constants), variable{endpointKey, endpoint}, variable{headersKey, ""})
	for _, v := range wanted {
		if value, ok := t.inEnvironment(v.key); ok && value != v.value {
			lines = append(lines, v.key+" is set in the environment.")
		}
	}
	lines = append(lines, t.foreign()...)
	for _, key := range otherSignals {
		if where := t.where(key); where != "" && !userOn && t.value(key) != "none" {
			lines = append(lines, key+" is set in "+where+"; turning on telemetry would start that export too.")
		}
	}
	return lines
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
