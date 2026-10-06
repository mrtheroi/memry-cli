package email_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/email"
)

// Ported from "trims and lowercases the email before sending it".
func TestNormalizeTrimsAndLowercases(t *testing.T) {
	got, ok := email.Normalize("  Ana@Example.COM \n")
	if !ok || got != "ana@example.com" {
		t.Errorf("Normalize = %q, %v; want ana@example.com, true", got, ok)
	}
}

// Ported from "fails without sending anything when the --email option is
// not valid UTF-8" (or "not an email address") and "asks for the email
// again until it is valid": Normalize accepts exactly what PHP's
// Email::normalize does, checked against PHP's own answers.
func TestNormalizeAcceptsWhatPHPFilterValidateEmailAccepts(t *testing.T) {
	data, err := os.ReadFile("testdata/php_filter_validate_email.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		want, quoted, _ := strings.Cut(line, " ")
		address, err := strconv.Unquote(quoted)
		if err != nil {
			t.Fatalf("bad line %q: %v", line, err)
		}
		if _, ok := email.Normalize(address); ok != (want == "1") {
			t.Errorf("Normalize(%q) ok = %v, want %v", address, ok, want == "1")
		}
	}
}
