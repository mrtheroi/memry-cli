package flags_test

import (
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/flags"
)

func TestScanReadsAValueGivenWithOrWithoutAnEqualsSign(t *testing.T) {
	tests := []struct {
		argv []string
		want flags.Args
	}{
		{[]string{"--url=https://memry.test"}, flags.Args{URL: flags.Option{Present: true, HasValue: true, Value: "https://memry.test"}}},
		{[]string{"--url", "https://memry.test"}, flags.Args{URL: flags.Option{Present: true, HasValue: true, Value: "https://memry.test"}}},
		{[]string{"--email=ana@example.com"}, flags.Args{Email: flags.Option{Present: true, HasValue: true, Value: "ana@example.com"}}},
		{[]string{"--email", "ana@example.com"}, flags.Args{Email: flags.Option{Present: true, HasValue: true, Value: "ana@example.com"}}},
		{[]string{"--token=admin-token"}, flags.Args{Token: flags.Option{Present: true, HasValue: true, Value: "admin-token"}}},
		{[]string{"--token", "admin-token"}, flags.Args{Token: flags.Option{Present: true, HasValue: true, Value: "admin-token"}}},
		{[]string{"--agents=claude-code,codex"}, flags.Args{Agents: flags.Option{Present: true, HasValue: true, Value: "claude-code,codex"}}},
		{[]string{"--agents", "claude-code,codex"}, flags.Args{Agents: flags.Option{Present: true, HasValue: true, Value: "claude-code,codex"}}},
	}
	for _, tt := range tests {
		if got := flags.Scan(tt.argv); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Scan(%q) = %+v, want %+v", tt.argv, got, tt.want)
		}
	}
}

func TestScanTellsAnOptionWithoutAValueFromAnEmptyValue(t *testing.T) {
	present := flags.Option{Present: true}
	empty := flags.Option{Present: true, HasValue: true, Value: ""}
	tests := []struct {
		argv []string
		want flags.Args
	}{
		{[]string{"--token"}, flags.Args{Token: present}},
		{[]string{"--url", "--token=admin-token"}, flags.Args{URL: present, Token: flags.Option{Present: true, HasValue: true, Value: "admin-token"}}},
		{[]string{"--email", "-n"}, flags.Args{Email: present, NoInteraction: true}},
		{[]string{"--agents="}, flags.Args{Agents: empty}},
		{[]string{"--agents", ""}, flags.Args{Agents: empty}},
		{[]string{"--agents=", "codex"}, flags.Args{Agents: empty, Rest: []string{"codex"}}},
	}
	for _, tt := range tests {
		if got := flags.Scan(tt.argv); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Scan(%q) = %+v, want %+v", tt.argv, got, tt.want)
		}
	}
}

func TestScanKeepsTheLastOccurrenceAndStopsAtADoubleDash(t *testing.T) {
	tests := []struct {
		argv []string
		want flags.Args
	}{
		{[]string{"--url=https://a.test", "--url", "https://b.test"}, flags.Args{URL: flags.Option{Present: true, HasValue: true, Value: "https://b.test"}}},
		{[]string{"--token=admin-token", "--token"}, flags.Args{Token: flags.Option{Present: true}}},
		{[]string{"--", "--token=admin-token", "-n"}, flags.Args{Rest: []string{"--", "--token=admin-token", "-n"}}},
		{[]string{"--force", "url=x"}, flags.Args{Rest: []string{"--force", "url=x"}}},
	}
	for _, tt := range tests {
		if got := flags.Scan(tt.argv); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Scan(%q) = %+v, want %+v", tt.argv, got, tt.want)
		}
	}
}
