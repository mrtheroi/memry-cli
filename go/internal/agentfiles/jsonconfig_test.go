package agentfiles_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Ported from tests/Unit/JsonMcpConfigTest.php.

func jsonPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "agent", "mcp_config.json")
}

func memryServer() *phpjson.Object {
	return phpjson.NewObject("command", "/opt/memry", "args", []string{"mcp"})
}

func putJSON(t *testing.T, path string) bool {
	t.Helper()
	ok, err := agentfiles.NewJSONConfig(path, "mcpServers").Put("memry", memryServer())
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	return ok
}

func removeJSON(t *testing.T, path string) agentfiles.Removal {
	t.Helper()
	removed, err := agentfiles.NewJSONConfig(path, "mcpServers").Remove("memry")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	return removed
}

func TestJSONConfigCreatesTheFileAndItsDirectoriesWithOnlyTheMemryServer(t *testing.T) {
	path := jsonPath(t)

	if !putJSON(t, path) {
		t.Error("Put = false, want true")
	}
	want := "{\n    \"mcpServers\": {\n        \"memry\": {\n            \"command\": \"/opt/memry\",\n            \"args\": [\n                \"mcp\"\n            ]\n        }\n    }\n}\n"
	if got := readFile(t, path); got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestJSONConfigKeepsEveryOtherServerAndSettingReplacingOnlyMemry(t *testing.T) {
	path := jsonPath(t)
	writeFile(t, path, `{"theme": "Diseño ✓", "empty": {}, "mcpServers": {"other": {"command": "npx", "args": ["-y", "other"]}, "memry": {"command": "old"}}, "url": "https://x.test/a"}`)

	putJSON(t, path)

	// What the PHP CLI writes, byte for byte.
	want := "{\n    \"theme\": \"Diseño ✓\",\n    \"empty\": {},\n    \"mcpServers\": {\n        \"other\": {\n            \"command\": \"npx\",\n            \"args\": [\n                \"-y\",\n                \"other\"\n            ]\n        },\n        \"memry\": {\n            \"command\": \"/opt/memry\",\n            \"args\": [\n                \"mcp\"\n            ]\n        }\n    },\n    \"url\": \"https://x.test/a\"\n}\n"
	if got := readFile(t, path); got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestJSONConfigLeavesAFileItCannotParseUntouched(t *testing.T) {
	tests := map[string]string{
		"invalid JSON":            `{"mcpServers": {`,
		"JSON with comments":      "{\n  // my servers\n  \"mcpServers\": {}\n}",
		"not an object":           `[]`,
		"servers not an object":   `{"mcpServers": []}`,
		"server list is a string": `{"mcpServers": "nope"}`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := jsonPath(t)
			writeFile(t, path, contents)

			if putJSON(t, path) {
				t.Error("Put = true, want false")
			}
			if got := readFile(t, path); got != contents {
				t.Errorf("contents = %q, want %q", got, contents)
			}
		})
	}
}

func TestJSONConfigRemovesOnlyTheMemryServerKeepingTheOthers(t *testing.T) {
	path := jsonPath(t)
	writeFile(t, path, `{"theme": "Diseño ✓", "mcpServers": {"other": {"command": "npx"}, "memry": {"command": "old"}}}`)

	if got := removeJSON(t, path); got != agentfiles.Removed {
		t.Errorf("Remove = %v, want Removed", got)
	}
	want := "{\n    \"theme\": \"Diseño ✓\",\n    \"mcpServers\": {\n        \"other\": {\n            \"command\": \"npx\"\n        }\n    }\n}\n"
	if got := readFile(t, path); got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestJSONConfigDropsTheServerListWhenMemryWasTheOnlyServerInIt(t *testing.T) {
	path := jsonPath(t)
	writeFile(t, path, `{"theme": "dark", "mcpServers": {"memry": {"command": "old"}}}`)

	removeJSON(t, path)

	if got, want := readFile(t, path), "{\n    \"theme\": \"dark\"\n}\n"; got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestJSONConfigRemovesNothingWhenThereIsNoMemryServerOrNoFile(t *testing.T) {
	t.Run("no file", func(t *testing.T) {
		path := jsonPath(t)

		if got := removeJSON(t, path); got != agentfiles.NotFound {
			t.Errorf("Remove = %v, want NotFound", got)
		}
		if exists(t, path) {
			t.Error("the file was created")
		}
	})
	tests := map[string]string{
		"no servers":    `{"theme":"dark"}`,
		"other servers": `{"mcpServers":{"other":{}}}`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := jsonPath(t)
			writeFile(t, path, contents)

			if got := removeJSON(t, path); got != agentfiles.NotFound {
				t.Errorf("Remove = %v, want NotFound", got)
			}
			if got := readFile(t, path); got != contents {
				t.Errorf("contents = %q, want %q", got, contents)
			}
		})
	}
}

func TestJSONConfigLeavesAFileItCannotParseUntouchedWhenRemoving(t *testing.T) {
	path := jsonPath(t)
	contents := `{"mcpServers": {`
	writeFile(t, path, contents)

	if got := removeJSON(t, path); got != agentfiles.Unsafe {
		t.Errorf("Remove = %v, want Unsafe", got)
	}
	if got := readFile(t, path); got != contents {
		t.Errorf("contents = %q, want %q", got, contents)
	}
}

func TestJSONConfigReplacesTheFileAtomicallyKeepingItsPermissions(t *testing.T) {
	path := jsonPath(t)
	writeFile(t, path, `{}`)
	before := chmod0640(t, path)

	putJSON(t, path)

	assertReplacedKeepingMode(t, path, before)
}

// Divergence from PHP, whose json_encode fails on a number too large for
// a float and then overwrites the file with a lone newline: the file is
// left untouched instead.
func TestJSONConfigLeavesAFileItCannotWriteBackUntouched(t *testing.T) {
	path := jsonPath(t)
	contents := `{"limit": 1e400, "mcpServers": {"memry": {}}}`
	writeFile(t, path, contents)

	if putJSON(t, path) {
		t.Error("Put = true, want false")
	}
	if got := removeJSON(t, path); got != agentfiles.Unsafe {
		t.Errorf("Remove = %v, want Unsafe", got)
	}
	if got := readFile(t, path); got != contents {
		t.Errorf("contents = %q, want %q", got, contents)
	}
}
