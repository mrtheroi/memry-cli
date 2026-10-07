package mcp_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/mcp"
)

// runHeaders runs `memry mcp-headers` with the config file contents (none
// when nil) and returns its exit code, stdout and stderr.
func runHeaders(t *testing.T, contents *string) (int, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if contents != nil {
		if err := os.WriteFile(path, []byte(*contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	code := mcp.Headers(mcp.HeadersEnv{
		LookupEnv: func(key string) (string, bool) {
			value, ok := map[string]string{"MEMRY_CONFIG": path}[key]
			return value, ok
		},
		Out: &out,
		Err: &errOut,
	})
	return code, out.String(), errOut.String()
}

// Ported from "prints the authorization header as JSON from the config
// file".
func TestHeadersPrintsTheAuthorizationHeader(t *testing.T) {
	config := `{"url":"https://memry.test","token":"secret-token"}`

	code, out, _ := runHeaders(t, &config)

	if want := `{"Authorization":"Bearer secret-token"}` + "\n"; code != 0 || out != want {
		t.Errorf("mcp-headers = %d, %q; want 0, %q", code, out, want)
	}
}

// Ported from "fails with nothing on stdout when the config file is
// missing", "… when the config has no token" (missing or empty) and "…
// when the config file is not valid JSON".
func TestHeadersFailsWithoutALogin(t *testing.T) {
	missingToken, emptyToken, invalid := `{"url":"https://memry.test"}`, `{"url":"https://memry.test","token":""}`, `{not json`
	for name, contents := range map[string]*string{
		"missing config": nil,
		"missing token":  &missingToken,
		"empty token":    &emptyToken,
		"invalid json":   &invalid,
	} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := runHeaders(t, contents)

			if code != 1 || out != "" || errOut != "memry is not logged in. Run memry setup.\n" {
				t.Errorf("mcp-headers = %d, stdout %q, stderr %q; want 1, nothing, the login message", code, out, errOut)
			}
		})
	}
}

// Not in the PHP tests: the JSON is PHP's json_encode without flags, which
// escapes "/" (and non-ASCII characters).
func TestHeadersEncodesTheTokenLikePHP(t *testing.T) {
	config := `{"url":"https://memry.test","token":"1|a/b"}`

	_, out, _ := runHeaders(t, &config)

	if want := `{"Authorization":"Bearer 1|a\/b"}` + "\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}
