package usage

import (
	"slices"

	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// genericKeys are the OTLP settings shared by every signal. memry never
// writes them: a generic endpoint or protocol would redirect the user's
// other telemetry, and Claude Code merges generic headers (often another
// collector's credentials) into the metrics headers sent to memry.
var genericKeys = []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_PROTOCOL"}

// telemetry is the telemetry configuration Claude Code runs with: its
// launch environment, which wins, and the env of its settings.json.
type telemetry struct {
	environment func(string) (string, bool)
	settings    *phpjson.Object
	path        string
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

// blockers says which settings stop memry from turning on usage metrics
// to endpoint, whatever --force says: memry's variables exported in the
// launch environment with another value (any headers there would replace
// memry's token), the generic OTLP settings, wherever they are set, and,
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
	for _, key := range genericKeys {
		if where := t.where(key); where != "" {
			lines = append(lines, key+" is set in "+where+".")
		}
	}
	for _, key := range otherSignals {
		if where := t.where(key); where != "" && !userOn && t.value(key) != "none" {
			lines = append(lines, key+" is set in "+where+"; turning on telemetry would start that export too.")
		}
	}
	return lines
}
