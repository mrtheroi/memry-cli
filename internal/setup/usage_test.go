package setup_test

import (
	"reflect"
	"testing"
)

// Logging in to another server keeps the usage metrics record: it says
// which server the usage token belongs to, so `memry usage` can turn the
// export off later without sending that token's id to the new server.
func TestSetupKeepsTheUsageMetricsRecord(t *testing.T) {
	h, s, old := newHarness(t), newServer(t), newServer(t)
	h.useAgents(newClaude())
	record := map[string]any{"current": map[string]any{"server_url": old.URL, "settings_path": "/home/ana/.claude/settings.json", "values": map[string]any{"OTEL_METRICS_EXPORTER": "otlp"}, "token_id": float64(7)}}
	h.writeConfig(map[string]any{"url": old.URL, "token": "old-token", "usage_metrics": record})

	output, code := h.run(tokenArgs(s.URL, "admin-token"))

	assertExit(t, code, 0, output)
	if got := h.config()["usage_metrics"]; !reflect.DeepEqual(got, record) {
		t.Errorf("usage_metrics = %v, want %v kept", got, record)
	}
}
