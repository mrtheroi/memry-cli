package agentfiles_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// goDivergences are the inputs where the Go CLI deliberately refuses what
// the PHP CLI does: a quoted TOML key json_decode cannot read, which PHP
// keeps quoted, missing memry's table and appending a duplicate one.
// Both put and remove leave the file untouched as unsafe instead.
var goDivergences = map[string]bool{
	"[mcp_servers.\"me\\U0000006Dry\"]\ncommand = \"old\"\n": true,
}

// goldenCase is one line of testdata/php_agentfiles.jsonl: what the PHP
// CLI's own classes do to a file.
type goldenCase struct {
	Kind     string
	Contents *string
	Put      [2]any
	Remove   [2]any
}

// Put and Remove do to every file what the PHP CLI does, byte for byte,
// as recorded by testdata/php_agentfiles.php.
func TestEditorsMatchThePHPCLI(t *testing.T) {
	file, err := os.Open("testdata/php_agentfiles.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	server := phpjson.NewObject("command", "/opt/memry", "args", []string{"mcp"},
		"env", phpjson.NewObject("MEMRY_CONFIG", `/tmp/a "b"/é.json`))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var tt goldenCase
		if err := json.Unmarshal(scanner.Bytes(), &tt); err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{"put", "remove"} {
			path := filepath.Join(t.TempDir(), "file")
			if tt.Contents != nil {
				writeFile(t, path, *tt.Contents)
			}
			var result any
			var err error
			switch {
			case method == "put" && tt.Kind == "rules":
				result, err = agentfiles.NewRulesFile(path).Put("Use memry.\nÉ")
			case method == "put":
				result, err = editor(tt.Kind, path).Put("memry", server)
			default:
				var removal agentfiles.Removal
				if tt.Kind == "rules" {
					removal, err = agentfiles.NewRulesFile(path).Remove()
				} else {
					removal, err = editor(tt.Kind, path).Remove("memry")
				}
				result = map[agentfiles.Removal]any{agentfiles.NotFound: false, agentfiles.Removed: true, agentfiles.Unsafe: nil}[removal]
			}
			if err != nil {
				t.Fatalf("%s %s(%q): %v", tt.Kind, method, deref(tt.Contents), err)
			}
			var after any
			if exists(t, path) {
				after = readFile(t, path)
			}
			want := tt.Put
			if method == "remove" {
				want = tt.Remove
			}
			if tt.Contents != nil && goDivergences[*tt.Contents] {
				want = [2]any{false, *tt.Contents}
				if method == "remove" {
					want[0] = nil
				}
			}
			if result != want[0] || after != want[1] {
				t.Errorf("%s %s(%q) = %v leaving %q, want %v leaving %q", tt.Kind, method, deref(tt.Contents), result, after, want[0], want[1])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func editor(kind, path string) agentfiles.MCPConfig {
	if kind == "toml" {
		return agentfiles.NewTOMLConfig(path)
	}
	return agentfiles.NewJSONConfig(path, "mcpServers")
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
