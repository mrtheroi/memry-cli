package flags_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/flags"
)

const configNeedsValue = "The --config option needs a value."

// A --config without a value (nothing after it, or another option) is an
// error, and so is an empty one.
func TestExtractConfigRejectsAMissingOrEmptyValue(t *testing.T) {
	for _, argv := range [][]string{{"--config"}, {"-q", "--config"}, {"--config="}, {"--config", ""}, {"--config", "-q"}} {
		if _, given, _, err := flags.ExtractConfig(argv); err == nil || err.Error() != configNeedsValue {
			t.Errorf("ExtractConfig(%q) = given %v, error %v; want the error %q", argv, given, err, configNeedsValue)
		}
	}
}

// --config=<path> and --config <path> are taken out of argv, wherever they
// are, and the rest is kept in order.
func TestExtractConfigAcceptsEqualsAndSpace(t *testing.T) {
	want := filepath.Join(t.TempDir(), "memry.json")
	for _, argv := range [][]string{
		{"--config=" + want, "-q", "--force"},
		{"-q", "--config", want, "--force"},
		{"-q", "--force", "--config=" + want},
	} {
		path, given, rest, err := flags.ExtractConfig(argv)
		if err != nil || !given || path != want || !reflect.DeepEqual(rest, []string{"-q", "--force"}) {
			t.Errorf("ExtractConfig(%q) = %q, %v, %q, %v; want %q, true, [-q --force], nil", argv, path, given, rest, err, want)
		}
	}
}

// Nothing after "--" is an option, so --config there stays in the rest.
func TestExtractConfigStopsAtDoubleDash(t *testing.T) {
	want := filepath.Join(t.TempDir(), "memry.json")
	argv := []string{"--config", want, "--", "--config=other"}

	path, given, rest, err := flags.ExtractConfig(argv)

	if err != nil || !given || path != want || !reflect.DeepEqual(rest, []string{"--", "--config=other"}) {
		t.Errorf("ExtractConfig(%q) = %q, %v, %q, %v; want %q, true, [-- --config=other], nil", argv, path, given, rest, err, want)
	}
}

// The last --config wins, as for any option.
func TestExtractConfigLastWins(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "second.json")
	argv := []string{"--config", filepath.Join(dir, "first.json"), "--config=" + want}

	path, _, _, err := flags.ExtractConfig(argv)

	if err != nil || path != want {
		t.Errorf("ExtractConfig(%q) = %q, %v; want %q", argv, path, err, want)
	}
}

// A relative path is made absolute, so what hooks and agents record does
// not depend on their working directory.
func TestExtractConfigMakesThePathAbsolute(t *testing.T) {
	path, _, _, err := flags.ExtractConfig([]string{"--config=memry.json"})

	if err != nil || !filepath.IsAbs(path) || filepath.Base(path) != "memry.json" {
		t.Errorf("ExtractConfig(--config=memry.json) = %q, %v; want an absolute path ending in memry.json", path, err)
	}
}
