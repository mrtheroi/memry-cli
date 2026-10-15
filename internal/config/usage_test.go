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
	want := config.UsageMetrics{Current: validSetup()}
	cfg.SetUsageMetrics(want)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := loaded.UsageMetrics()

	if !ok || err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("UsageMetrics = %+v, %v, %v; want %+v", got, ok, err, want)
	}
}

// validSetup is a complete setup: every variable expected, the endpoint
// kept, the others owned.
func validSetup() *config.UsageSetup {
	expected := map[string]string{}
	for _, key := range config.UsageKeys {
		expected[key] = "x"
	}
	expected[config.UsageHeadersKey] = "Authorization=Bearer 7|secret"
	values := map[string]string{}
	for key, value := range expected {
		if key != "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT" {
			values[key] = value
		}
	}
	return &config.UsageSetup{ServerURL: "https://memry.test", SettingsPath: "/s.json", Expected: expected, Values: values,
		Kept: []string{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"}, TokenID: 7}
}

// A record is valid only with its whole expected configuration, and its
// owned and kept variables consistent with it: a record that lost part of
// it could not tell what to check or remove.
func TestUsageMetricsValidRequiresTheWholeExpectedConfiguration(t *testing.T) {
	if !(config.UsageMetrics{Current: validSetup()}).Valid() {
		t.Fatal("the complete setup is not valid")
	}
	for name, breakIt := range map[string]func(*config.UsageSetup){
		"no expected": func(s *config.UsageSetup) { s.Expected = nil },
		"expected lost the headers": func(s *config.UsageSetup) {
			delete(s.Expected, config.UsageHeadersKey)
			delete(s.Values, config.UsageHeadersKey)
		},
		"expected lost a gate": func(s *config.UsageSetup) {
			delete(s.Expected, "OTEL_METRICS_INCLUDE_VERSION")
			delete(s.Values, "OTEL_METRICS_INCLUDE_VERSION")
		},
		"headers not a bearer": func(s *config.UsageSetup) {
			s.Expected[config.UsageHeadersKey] = "x"
			s.Values[config.UsageHeadersKey] = "x"
		},
		"owned headers missing": func(s *config.UsageSetup) {
			delete(s.Values, config.UsageHeadersKey)
			s.Kept = append(s.Kept, config.UsageHeadersKey)
		},
		"owned value differs":         func(s *config.UsageSetup) { s.Values["OTEL_METRICS_EXPORTER"] = "y" },
		"owned key not expected":      func(s *config.UsageSetup) { s.Values["OTHER"] = "1" },
		"kept is not the rest":        func(s *config.UsageSetup) { s.Kept = nil },
		"kept also owned":             func(s *config.UsageSetup) { s.Kept = append(s.Kept, "OTEL_METRICS_EXPORTER") },
		"expected has an unknown key": func(s *config.UsageSetup) { s.Expected["OTHER"] = "1" },
	} {
		t.Run(name, func(t *testing.T) {
			setup := validSetup()
			breakIt(setup)
			if (config.UsageMetrics{Current: setup}).Valid() {
				t.Error("Valid = true, want false")
			}
		})
	}
}
