package config_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/config"
)

func TestUsageMetricsIsSavedAndLoadedBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := config.Load(path)
	want := config.UsageMetrics{
		SettingsPath: "/home/ana/.claude/settings.json",
		Values:       map[string]string{"OTEL_METRICS_EXPORTER": "otlp"},
		Kept:         []string{"CLAUDE_CODE_ENABLE_TELEMETRY"},
	}
	cfg.SetUsageMetrics(want)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := loaded.UsageMetrics()

	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("UsageMetrics = %+v, %v; want %+v", got, ok, want)
	}
}
