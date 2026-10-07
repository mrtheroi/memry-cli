package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/config"
)

func TestPath(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"MEMRY_CONFIG wins", map[string]string{"MEMRY_CONFIG": "/tmp/custom.json", "HOME": "/home/ana"}, "/tmp/custom.json"},
		{"defaults to the home directory", map[string]string{"HOME": "/home/ana"}, "/home/ana/.config/memry/config.json"},
		{"an empty MEMRY_CONFIG is unset", map[string]string{"MEMRY_CONFIG": "", "HOME": "/home/ana"}, "/home/ana/.config/memry/config.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string { return tt.env[key] }
			if got := config.Path(getenv); got != tt.want {
				t.Errorf("Path() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadOfAMissingFileIsAnEmptyConfig(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing", "config.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if url, ok := cfg.URL(); ok {
		t.Errorf("URL() = %q, true; want no url", url)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestURLAndTokenAreReadOnlyWhenTheyAreStrings(t *testing.T) {
	tests := []struct {
		name      string
		contents  string
		wantURL   string
		wantURLOK bool
		wantTok   string
		wantTokOK bool
	}{
		{"strings", `{"url": "https://memry.test", "token": "secret"}`, "https://memry.test", true, "secret", true},
		{"empty strings are still strings", `{"url": "", "token": ""}`, "", true, "", true},
		{"missing keys", `{"project": "kept"}`, "", false, "", false},
		{"other types", `{"url": 1, "token": null}`, "", false, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(writeConfig(t, tt.contents))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if url, ok := cfg.URL(); url != tt.wantURL || ok != tt.wantURLOK {
				t.Errorf("URL() = %q, %v; want %q, %v", url, ok, tt.wantURL, tt.wantURLOK)
			}
			if token, ok := cfg.Token(); token != tt.wantTok || ok != tt.wantTokOK {
				t.Errorf("Token() = %q, %v; want %q, %v", token, ok, tt.wantTok, tt.wantTokOK)
			}
		})
	}
}

func TestLoadFailsOnAFileThatIsNotAJSONObject(t *testing.T) {
	for _, contents := range []string{`{"url": `, `not json`, `["a"]`, `"text"`, `null`, ``, `{} {}`} {
		t.Run(contents, func(t *testing.T) {
			if _, err := config.Load(writeConfig(t, contents)); err == nil {
				t.Errorf("Load(%q) = nil error, want one", contents)
			}
		})
	}
}

func TestAgentsAreReadOnlyWhenTheyAreAList(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     []string
		wantOK   bool
	}{
		{"a list", `{"agents": ["claude-code", "codex"]}`, []string{"claude-code", "codex"}, true},
		{"an empty list is a saved selection of none", `{"agents": []}`, []string{}, true},
		{"missing", `{}`, nil, false},
		{"not a list", `{"agents": "codex"}`, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(writeConfig(t, tt.contents))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got, ok := cfg.Agents()
			if !reflect.DeepEqual(got, tt.want) || ok != tt.wantOK {
				t.Errorf("Agents() = %#v, %v; want %#v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// Ported from "overwrites url and token but keeps other keys of an existing
// config file": the bytes are what the PHP CLI writes, so either CLI reads
// the other's file.
func TestSaveOverwritesTheGivenKeysKeepingTheOthersInOrder(t *testing.T) {
	path := writeConfig(t, `{"url":"https://old.test","token":"old-token","project":"kept"}`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	cfg.Set("url", "https://memry.test")
	cfg.Set("token", "secret-token")
	cfg.Set("agents", []string{"claude-code"})
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, _ := os.ReadFile(path)
	want := `{
    "url": "https://memry.test",
    "token": "secret-token",
    "project": "kept",
    "agents": [
        "claude-code"
    ]
}
`
	if string(got) != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// Ported from "makes the config file readable by the owner only" and
// "creates the missing parent directory readable by the owner only".
func TestSaveMakesTheFileAndNewDirectoriesReadableByTheOwnerOnly(t *testing.T) {
	t.Run("a new file in a missing directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "memry", "config.json")
		cfg, _ := config.Load(path)
		cfg.Set("token", "secret-token")

		if err := cfg.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}

		if got := mode(t, path); got != 0o600 {
			t.Errorf("file mode = %o, want 0600", got)
		}
		if got := mode(t, filepath.Dir(path)); got != 0o700 {
			t.Errorf("dir mode = %o, want 0700", got)
		}
	})

	t.Run("an existing file readable by others", func(t *testing.T) {
		path := writeConfig(t, `{}`)
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, _ := config.Load(path)
		cfg.Set("token", "secret-token")

		if err := cfg.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}

		if got := mode(t, path); got != 0o600 {
			t.Errorf("file mode = %o, want 0600", got)
		}
	})
}

func TestSaveLeavesHTMLCharactersUnescapedLikePHP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := config.Load(path)
	cfg.Set("url", "https://memry.test/a?b=<c>&d")

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, _ := os.ReadFile(path)
	if want := "{\n    \"url\": \"https://memry.test/a?b=<c>&d\"\n}\n"; string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

// PHP's ConfigFile::read() decodes with json_decode(...) ?? [], so a file
// that is not valid JSON (which includes invalid UTF-8), or is null or [],
// is an empty config, and setup overwrites it.
func TestLoadOrEmptyTreatsWhatPHPReadsAsEmptyAsAnEmptyConfig(t *testing.T) {
	for _, contents := range []string{"", "{not json", "null", "[]", " [ ] \n", "{\"url\": \"\xc3\"}", "\xef\xbb\xbf{}"} {
		path := writeConfig(t, contents)
		cfg, err := config.LoadOrEmpty(path)
		if err != nil {
			t.Fatalf("LoadOrEmpty(%q): %v", contents, err)
		}
		cfg.Set("token", "secret")
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if want := "{\n    \"token\": \"secret\"\n}\n"; string(got) != want {
			t.Errorf("after LoadOrEmpty(%q) and Save, file = %q, want %q", contents, got, want)
		}
	}
}

func TestLoadOrEmptyReadsAnObjectAndRejectsOtherJSONLikeLoad(t *testing.T) {
	cfg, err := config.LoadOrEmpty(writeConfig(t, `{"url": "https://memry.test"}`))
	if err != nil {
		t.Fatalf("LoadOrEmpty: %v", err)
	}
	if url, _ := cfg.URL(); url != "https://memry.test" {
		t.Errorf("URL() = %q, want https://memry.test", url)
	}
	for _, contents := range []string{`"text"`, `5`, `[1, 2]`} {
		if _, err := config.LoadOrEmpty(writeConfig(t, contents)); err == nil {
			t.Errorf("LoadOrEmpty(%q) error = nil, want an error", contents)
		}
	}
}
