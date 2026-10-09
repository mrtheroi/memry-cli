package setup_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// Ported from "overwrites url and token but keeps other keys of an
// existing config file".
func TestOverwritesURLAndTokenButKeepsOtherKeys(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	previous := `{"url":"` + unreachableURL(t) + `","token":"old-token","project":"kept"}`
	if err := os.WriteFile(h.configPath, []byte(previous), 0o600); err != nil {
		t.Fatal(err)
	}

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	data, _ := os.ReadFile(h.configPath)
	want := "{\n    \"url\": \"" + s.URL + "\",\n    \"token\": \"secret-token\",\n    \"project\": \"kept\",\n    \"agents\": [\n        \"claude-code\"\n    ]\n}\n"
	if string(data) != want {
		t.Errorf("config = %q, want %q", data, want)
	}
}

// Like the PHP CLI, which reads it as an empty config, setup overwrites a
// config file that is not valid JSON, null or [].
func TestOverwritesAConfigFileThePHPCLIReadsAsEmpty(t *testing.T) {
	for _, contents := range []string{"{not json", "null", "[]"} {
		h, s := newHarness(t), newServer(t)
		if err := os.WriteFile(h.configPath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}

		output, code := h.run(emailArgs(s.URL), code123456)

		assertExit(t, code, 0, output)
		want := map[string]any{"url": s.URL, "token": "secret-token", "agents": []any{"claude-code"}}
		if got := h.config(); !reflect.DeepEqual(got, want) {
			t.Errorf("config after %q = %v, want %v", contents, got, want)
		}
	}
}

// The credentials cannot be saved over other JSON, or where no file can
// be written: setup fails, telling why, and keeps the file as it was.
func TestFailsWhenTheCredentialsCannotBeSaved(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	if err := os.WriteFile(h.configPath, []byte(`"text"`), 0o600); err != nil {
		t.Fatal(err)
	}

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not save the credentials to "+h.configPath+": ")
	assertNotContains(t, output, "Logged in")
	if data, _ := os.ReadFile(h.configPath); string(data) != `"text"` {
		t.Errorf("config = %q, want it untouched", data)
	}

	h, s = newHarness(t), newServer(t)
	h.env["MEMRY_CONFIG"] = filepath.Join(h.dir, "a-file", "config.json")
	if err := os.WriteFile(filepath.Join(h.dir, "a-file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	output, code = h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not save the credentials to "+h.env["MEMRY_CONFIG"]+": ")
}

// Ported from "makes the config file readable by the owner only", "creates
// the missing parent directory readable by the owner only" and "writes to
// ~/.config/memry/config.json when MEMRY_CONFIG is not set".
func TestSavesTheConfigReadableByTheOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits; the Windows owner-only DACL is covered in slice 4a")
	}
	h, s := newHarness(t), newServer(t)
	delete(h.env, "MEMRY_CONFIG")
	path := filepath.Join(h.dir, ".config", "memry", "config.json")

	output, code := h.run(emailArgs(s.URL), code123456)

	assertExit(t, code, 0, output)
	assertContains(t, output, "Credentials saved to "+path+".\n")
	for name, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", name, got, want)
		}
	}
}

// Ported from "strips a trailing slash from the server url" and "uses the
// configured server url when no --url option is given" (MEMRY_URL).
func TestUsesTheServerURLWithoutItsTrailingSlash(t *testing.T) {
	for _, viaEnv := range []bool{false, true} {
		h, s := newHarness(t), newServer(t)
		argv := []string{"--url", s.URL + "/", "--email", "ana@example.com", "--agents", "claude-code"}
		if viaEnv {
			h.env["MEMRY_URL"] = s.URL + "/"
			argv = argv[2:]
		}

		output, code := h.run(argv, code123456)

		assertExit(t, code, 0, output)
		if len(s.sent("POST", "/api/auth/code")) != 1 {
			t.Errorf("POST /api/auth/code not sent: %+v", s.received())
		}
		if got := h.config()["url"]; got != s.URL {
			t.Errorf("saved url = %v, want %s", got, s.URL)
		}
	}
}
