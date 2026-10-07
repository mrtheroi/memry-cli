package agentfiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// A value derived from the OS (an executable path, MEMRY_CONFIG) can hold
// bytes that are not valid UTF-8, which JSON cannot encode. Writing must
// then fail and leave the agent's existing file exactly as it was, never
// replace it with a lone newline or an empty TOML value.

const invalidUTF8 = "/opt/m\xffemry/bin/memry"

func unencodableServer() *phpjson.Object {
	return phpjson.NewObject("command", invalidUTF8, "args", []any{"mcp"})
}

func writeExisting(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertUnchanged(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("file = %q, want it unchanged: %q", got, want)
	}
}

func TestJSONConfigPutLeavesTheFileUntouchedForAnUnencodableValue(t *testing.T) {
	existing := "{\n    \"mcpServers\": {\n        \"other\": {\n            \"command\": \"other\"\n        }\n    }\n}\n"
	path := writeExisting(t, "mcp_config.json", existing)

	_, err := agentfiles.NewJSONConfig(path, "mcpServers").Put("memry", unencodableServer())

	if err == nil {
		t.Error("Put: want an error, got none")
	}
	assertUnchanged(t, path, existing)
}

func TestClaudeSettingsLeavesTheFileUntouchedForAnUnencodableHook(t *testing.T) {
	existing := "{\n    \"theme\": \"dark\"\n}\n"
	path := writeExisting(t, "settings.json", existing)
	group := phpjson.NewObject("matcher", "startup", "hooks", []any{
		phpjson.NewObject("type", "command", "command", invalidUTF8+" hook:session-start"),
	})

	_, err := agentfiles.NewClaudeSettings(path).ReplaceSessionStartHook(group, "hook:session-start")

	if err == nil {
		t.Error("ReplaceSessionStartHook: want an error, got none")
	}
	assertUnchanged(t, path, existing)
}

func TestTOMLConfigPutLeavesTheFileUntouchedForAnUnencodableValue(t *testing.T) {
	existing := "model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"other\"\n"
	path := writeExisting(t, "config.toml", existing)

	_, err := agentfiles.NewTOMLConfig(path).Put("memry", unencodableServer())

	if err == nil {
		t.Error("Put: want an error, got none")
	}
	assertUnchanged(t, path, existing)
}
