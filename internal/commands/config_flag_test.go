package commands_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configWith writes a login with token to a config file named name in a
// new temporary directory, and returns its path.
func configWith(t *testing.T, name, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(`{"url":"https://memry.test","token":"`+token+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// S1.3.a: --config <path> (or --config=<path>) makes mcp-headers read that
// file, over MEMRY_CONFIG, and leaves the process environment alone.
func TestMcpHeadersReadsTheTokenFromTheConfigFlag(t *testing.T) {
	isolate(t)
	envPath := os.Getenv("MEMRY_CONFIG")
	file := configWith(t, "other.json", "flag-token")
	want := `{"Authorization":"Bearer flag-token"}` + "\n"

	for _, args := range [][]string{{"mcp-headers", "--config", file}, {"mcp-headers", "--config=" + file}} {
		out, errOut, err := executeIn(t, "", args...)

		if err != nil || out != want || errOut != "" {
			t.Errorf("%q = %q, %q, %v; want %q", args, out, errOut, err, want)
		}
	}
	if got := os.Getenv("MEMRY_CONFIG"); got != envPath {
		t.Errorf("MEMRY_CONFIG = %q after the run, want it untouched (%q)", got, envPath)
	}
}

// Like the other global options, --config may come before the command:
// cobra must find the command and not take the path for it.
func TestConfigFlagBeforeTheCommandIsStripped(t *testing.T) {
	isolate(t)
	file := configWith(t, "other.json", "flag-token")
	want := `{"Authorization":"Bearer flag-token"}` + "\n"

	for _, args := range [][]string{{"--config", file, "mcp-headers"}, {"--config=" + file, "mcp-headers"}} {
		out, _, err := executeIn(t, "", args...)

		if err != nil || out != want {
			t.Errorf("%q = %q, %v; want %q", args, out, err, want)
		}
	}
}

// loginServer is a fake memry server answering every request with 200.
func loginServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(s.Close)
	return s
}

// S1.3.b: setup --config <path> saves the login to that file only, not to
// MEMRY_CONFIG or the default path.
func TestSetupWritesOnlyToTheConfigFlagPath(t *testing.T) {
	dir := isolate(t)
	envPath := os.Getenv("MEMRY_CONFIG")
	file := filepath.Join(t.TempDir(), "nested", "flag.json")
	s := loginServer(t)

	// Without the claude CLI the agent step fails (exit 1), after the login
	// was saved.
	out, _, _ := executeIn(t, "", "setup", "--url", s.URL, "--token=admin-token", "--agents=claude-code", "-n", "--config", file)

	data, readErr := os.ReadFile(file)
	if readErr != nil || !strings.Contains(string(data), `"token": "admin-token"`) {
		t.Errorf("--config file = %q, %v; want the saved login", data, readErr)
	}
	if !strings.Contains(out, "Credentials saved to "+file+".") {
		t.Errorf("output = %q, want it to say the credentials were saved to %s", out, file)
	}
	for _, other := range []string{envPath, filepath.Join(dir, ".config", "memry", "config.json")} {
		if _, statErr := os.Stat(other); !os.IsNotExist(statErr) {
			t.Errorf("%s exists (%v), want only the --config file written", other, statErr)
		}
	}
}

// uninstall --config <path> removes the login of that file, and leaves
// the one MEMRY_CONFIG points to.
func TestUninstallHonorsTheConfigFlag(t *testing.T) {
	isolate(t)
	envPath := os.Getenv("MEMRY_CONFIG")
	s := loginServer(t)
	login := `{"url":"` + s.URL + `","token":"secret-token"}`
	file := filepath.Join(t.TempDir(), "flag.json")
	for _, path := range []string{envPath, file} {
		if err := os.WriteFile(path, []byte(login), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	out, _, err := executeIn(t, "", "uninstall", "--force", "--config", file)

	if err != nil {
		t.Errorf("uninstall = %q, %v; want success", out, err)
	}
	if _, statErr := os.Stat(file); !os.IsNotExist(statErr) {
		t.Errorf("--config file still there (%v), want it deleted", statErr)
	}
	if _, statErr := os.Stat(envPath); statErr != nil {
		t.Errorf("MEMRY_CONFIG file: %v, want it left alone", statErr)
	}
}

// S1.2.c: --config wins over MEMRY_CONFIG, which wins over the default
// path under HOME.
func TestConfigFlagPrecedence(t *testing.T) {
	dir := isolate(t)
	defaultPath := filepath.Join(dir, ".config", "memry", "config.json")
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, token := range map[string]string{defaultPath: "default-token", os.Getenv("MEMRY_CONFIG"): "env-token"} {
		if err := os.WriteFile(path, []byte(`{"url":"https://memry.test","token":"`+token+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	flagFile := configWith(t, "flag.json", "flag-token")

	for _, tt := range []struct {
		args  []string
		token string
	}{
		{[]string{"mcp-headers", "--config", flagFile}, "flag-token"},
		{[]string{"mcp-headers"}, "env-token"},
	} {
		out, _, err := executeIn(t, "", tt.args...)

		if want := `{"Authorization":"Bearer ` + tt.token + `"}` + "\n"; err != nil || out != want {
			t.Errorf("%q = %q, %v; want %q", tt.args, out, err, want)
		}
	}
	t.Setenv("MEMRY_CONFIG", "")
	if out, _, err := executeIn(t, "", "mcp-headers"); err != nil || out != `{"Authorization":"Bearer default-token"}`+"\n" {
		t.Errorf("mcp-headers with no MEMRY_CONFIG = %q, %v; want the default path's token", out, err)
	}
}

// S1.3.c: an empty --config is an error before any file is read or
// written: setup does not save a login, uninstall does not delete one.
func TestEmptyConfigFlagReadsAndWritesNoFile(t *testing.T) {
	dir := isolate(t)
	envPath := os.Getenv("MEMRY_CONFIG")
	s := loginServer(t)
	if err := os.WriteFile(envPath, []byte(`{"url":"`+s.URL+`","token":"secret-token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(envPath)
	want := errorBlock("The --config option needs a value.")

	for _, args := range [][]string{
		{"setup", "--url", s.URL, "--token=admin-token", "-n", "--config="},
		{"uninstall", "--force", "--config", ""},
		{"mcp-headers", "--config"},
	} {
		out, _, err := executeIn(t, "", args...)

		if err == nil || out != want {
			t.Errorf("%q = %q, %v; want %q and an error", args, out, err, want)
		}
	}
	if after, _ := os.ReadFile(envPath); string(after) != string(before) {
		t.Errorf("MEMRY_CONFIG file = %q, want it unchanged (%q)", after, before)
	}
	if _, err := os.Stat(filepath.Join(dir, ".config")); !os.IsNotExist(err) {
		t.Errorf("default config dir exists (%v), want nothing written", err)
	}
	if out, _, err := executeIn(t, "", "mcp-headers", "--silent", "--config="); err == nil || out != "" {
		t.Errorf("--silent with an empty --config = %q, %v; want nothing and an error", out, err)
	}
}
