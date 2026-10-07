package agentfiles_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Ported from tests/Unit/TomlMcpConfigTest.php.

const tomlTable = "[mcp_servers.memry]\ncommand = \"/opt/memry\"\nargs = [\"mcp\"]\n"

func tomlPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "codex", "config.toml")
}

func putTOMLServer(t *testing.T, path string, server *phpjson.Object) bool {
	t.Helper()
	ok, err := agentfiles.NewTOMLConfig(path).Put("memry", server)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	return ok
}

func putTOML(t *testing.T, path string) bool {
	t.Helper()
	return putTOMLServer(t, path, memryServer())
}

func removeTOML(t *testing.T, path string) agentfiles.Removal {
	t.Helper()
	removed, err := agentfiles.NewTOMLConfig(path).Remove("memry")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	return removed
}

func assertContents(t *testing.T, path, want string) {
	t.Helper()
	if got := readFile(t, path); got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestTOMLConfigCreatesTheFileAndItsDirectoriesWithOnlyTheMemryTable(t *testing.T) {
	path := tomlPath(t)

	if !putTOML(t, path) {
		t.Error("Put = false, want true")
	}
	assertContents(t, path, tomlTable)
}

func TestTOMLConfigWritesTheServerEnvironmentAsAnInlineTableEscapingStrings(t *testing.T) {
	path := tomlPath(t)
	server := memryServer()
	server.Set("env", phpjson.NewObject("MEMRY_CONFIG", `/tmp/a "b"\c.json`))

	putTOMLServer(t, path, server)

	assertContents(t, path, tomlTable+"env = { MEMRY_CONFIG = \"/tmp/a \\\"b\\\"\\\\c.json\" }\n")
}

func TestTOMLConfigAppendsTheTableAfterTheExistingConfigLeavingItAsItWas(t *testing.T) {
	tests := map[string]struct{ existing, separator string }{
		"comments, tables and arrays of tables": {`# Codex config — Diseño ✓
model = "o3"  # inline comment

[mcp_servers.other]
command = "npx"
args = [
  "-y",   # a comment inside an array
  "other",
]

[mcp_servers.other.env]
TOKEN = "x"

[[profiles.list]]
name = "a"
`, "\n"},
		"without a final newline": {`model = "o3"`, "\n\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := tomlPath(t)
			writeFile(t, path, tt.existing)

			putTOML(t, path)

			assertContents(t, path, tt.existing+tt.separator+tomlTable)
		})
	}
}

func TestTOMLConfigReplacesAnExistingMemryTableAndItsSubtablesInPlace(t *testing.T) {
	path := tomlPath(t)
	writeFile(t, path, `model = "o3"

[mcp_servers.memry]
command = "old"
args = [
  "[not a header]",
]

[mcp_servers.memry.env]
MEMRY_CONFIG = "/old"

# Other servers
[mcp_servers."other"]
command = "npx"
`)

	putTOML(t, path)

	assertContents(t, path, `model = "o3"

`+tomlTable+`
# Other servers
[mcp_servers."other"]
command = "npx"
`)
}

func TestTOMLConfigLeavesAFileItCannotEditSafelyUntouched(t *testing.T) {
	tests := map[string]string{
		"unterminated string":                  "model = \"o3\n",
		"unterminated multi-line string":       "notes = \"\"\"\nstill open\n",
		"unclosed array":                       "args = [\n  \"a\",\n",
		"broken table header":                  "[mcp_servers.other\ncommand = \"x\"\n",
		"not a key/value line":                 "just some text\n",
		"memry as an inline table":             "[mcp_servers]\nmemry = { command = \"old\" }\n",
		"memry through dotted keys":            "[mcp_servers]\nmemry.command = \"old\"\n",
		"mcp_servers as a root inline table":   "mcp_servers = { other = { command = \"x\" } }\n",
		"mcp_servers through root dotted keys": "mcp_servers.other.command = \"x\"\n",
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := tomlPath(t)
			writeFile(t, path, contents)

			if putTOML(t, path) {
				t.Error("Put = true, want false")
			}
			assertContents(t, path, contents)
		})
	}
}

func TestTOMLConfigLeavesExactlyTheSameFileAfterPuttingTheSameServerTwice(t *testing.T) {
	path := tomlPath(t)
	writeFile(t, path, "model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\n")
	putTOML(t, path)
	once := readFile(t, path)

	putTOML(t, path)

	assertContents(t, path, once)
}

func TestTOMLConfigDoesNotMistakeAHeaderLikeLineInAMultiLineStringForATable(t *testing.T) {
	path := tomlPath(t)
	contents := "notes = '''\n[mcp_servers.memry]\n'''\n"
	writeFile(t, path, contents)

	putTOML(t, path)

	assertContents(t, path, contents+"\n"+tomlTable)
}

func TestTOMLConfigRemovesOnlyTheMemryTablesRestoringTheConfigItWasAppendedTo(t *testing.T) {
	path := tomlPath(t)
	existing := "# Diseño ✓\nmodel = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\n"
	writeFile(t, path, existing)
	server := memryServer()
	server.Set("env", phpjson.NewObject("A", "b"))
	putTOMLServer(t, path, server)

	if got := removeTOML(t, path); got != agentfiles.Removed {
		t.Errorf("Remove = %v, want Removed", got)
	}
	assertContents(t, path, existing)
}

func TestTOMLConfigRemovesATableBetweenOtherTablesWithoutLeavingADoubleBlankLine(t *testing.T) {
	path := tomlPath(t)
	writeFile(t, path, "[a]\nx = 1\n\n"+tomlTable+"\n# b\n[b]\ny = 2\n")

	removeTOML(t, path)

	assertContents(t, path, "[a]\nx = 1\n\n# b\n[b]\ny = 2\n")
}

func TestTOMLConfigRemovesNothingWhenThereIsNoMemryServerOrNoFile(t *testing.T) {
	t.Run("no file", func(t *testing.T) {
		path := tomlPath(t)

		if got := removeTOML(t, path); got != agentfiles.NotFound {
			t.Errorf("Remove = %v, want NotFound", got)
		}
		if exists(t, path) {
			t.Error("the file was created")
		}
	})
	tests := map[string]string{
		"other servers":                         "[mcp_servers.other]\ncommand = \"x\"\n",
		"a server whose name starts with memry": "[mcp_servers.memry-old]\ncommand = \"x\"\n",
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := tomlPath(t)
			writeFile(t, path, contents)

			if got := removeTOML(t, path); got != agentfiles.NotFound {
				t.Errorf("Remove = %v, want NotFound", got)
			}
			assertContents(t, path, contents)
		})
	}
}

func TestTOMLConfigLeavesAFileItCannotEditSafelyUntouchedWhenRemoving(t *testing.T) {
	path := tomlPath(t)
	contents := "[mcp_servers]\nmemry = { command = \"old\" }\n"
	writeFile(t, path, contents)

	if got := removeTOML(t, path); got != agentfiles.Unsafe {
		t.Errorf("Remove = %v, want Unsafe", got)
	}
	assertContents(t, path, contents)
}

func TestTOMLConfigReplacesTheFileAtomicallyKeepingItsPermissions(t *testing.T) {
	path := tomlPath(t)
	writeFile(t, path, "model = 1\n")
	before := chmod0640(t, path)

	putTOML(t, path)

	assertReplacedKeepingMode(t, path, before)
}

// Codex review of PR #34. Divergence from PHP, whose json_decode cannot
// read TOML-only escapes like \UXXXXXXXX: it keeps the segment quoted,
// misses [mcp_servers."me\U0000006Dry"] (which is memry) and appends a
// duplicate table, making the TOML invalid. A quoted key segment whose
// escapes are not ones JSON and TOML read the same way (\b \t \n \f \r
// \" \\ and \uXXXX outside the surrogates) makes the file unsafe instead.
func TestTOMLConfigLeavesAFileWithAKeyItCannotDecodeWithCertaintyUntouched(t *testing.T) {
	tests := map[string]string{
		"a \\U escape":          "[mcp_servers.\"me\\U0000006Dry\"]\ncommand = \"old\"\n",
		"a \\U escape in a key": "[mcp_servers]\n\"\\U0001F600\" = 1\n",
		"an escaped slash":      "[mcp_servers.\"me\\/mry\"]\ncommand = \"old\"\n",
		"a surrogate pair":      "[mcp_servers.\"\\ud83d\\ude00\"]\ncommand = \"old\"\n",
		"a literal tab":         "[mcp_servers.\"me\tmry\"]\ncommand = \"old\"\n",
		"an unknown escape":     "[mcp_servers.\"me\\emry\"]\ncommand = \"old\"\n",
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := tomlPath(t)
			writeFile(t, path, contents)

			if putTOML(t, path) {
				t.Error("Put = true, want false")
			}
			if got := removeTOML(t, path); got != agentfiles.Unsafe {
				t.Errorf("Remove = %v, want Unsafe", got)
			}
			assertContents(t, path, contents)
		})
	}
}

// The escapes JSON and TOML share still name memry's table.
func TestTOMLConfigReplacesAMemryTableNamedWithSharedEscapes(t *testing.T) {
	path := tomlPath(t)
	writeFile(t, path, "[mcp_servers.\"\\u006demry\"]\ncommand = \"old\"\n")

	putTOML(t, path)

	assertContents(t, path, tomlTable)
}

// Codex review of PR #34: TOML basic strings forbid U+007F (DEL) and the
// control characters unescaped, which JSON's escaping would leave as they
// are (DEL). Every string memry writes, keys and values, escapes them.
func TestTOMLConfigEscapesTheCharactersTOMLForbidsInBasicStrings(t *testing.T) {
	path := tomlPath(t)
	server := memryServer()
	server.Set("env", phpjson.NewObject("A\x7fB", "/tmp/a\x7fb\x01c\td.json"))

	putTOMLServer(t, path, server)

	assertContents(t, path, tomlTable+"env = { \"A\\u007fB\" = \"/tmp/a\\u007fb\\u0001c\\td.json\" }\n")
}
