package agents_test

import "testing"

// Ported from tests/Feature/Agents/AntigravityAgentTest.php, asserting the
// exact JSON the PHP CLI writes where the PHP tests compare it decoded.

// stdioServer is the {command, args} server Antigravity and Windsurf get.
const stdioServer = `{
            "command": "/opt/homebrew/opt/memry/bin/memry",
            "args": [
                "mcp"
            ]
        }`

const stdioConfig = "{\n    \"mcpServers\": {\n        \"memry\": " + stdioServer + "\n    }\n}\n"

func TestAntigravityRegistersMemryAsAStdioMCPServerInANewMcpConfigJSON(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".gemini", "config", "mcp_config.json")

	result := e.agent(t, "antigravity").Install("https://memry.test")

	assertSuccessful(t, result, true)
	assertContents(t, config, stdioConfig)
	assertHasLine(t, result, "info", "Registered the memry MCP server in "+config+".")
}

func TestAntigravityIsInstalledWhenItsDirectoryInGeminiExistsNotWhenOnlyGeminiCLIIsThere(t *testing.T) {
	tests := map[string]struct {
		directory string
		installed bool
	}{
		"only Gemini CLI": {"", false},
		"antigravity":     {"antigravity", true},
		"config":          {"config", true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t)
			mkdir(t, e.path(".gemini", tt.directory))

			if got := e.agent(t, "antigravity").IsInstalled(); got != tt.installed {
				t.Errorf("IsInstalled = %v, want %v", got, tt.installed)
			}
		})
	}
}

func TestAntigravityPassesACustomMEMRY_CONFIGToTheServerThroughItsEnvironment(t *testing.T) {
	e := newTestEnv(t)
	e.vars["MEMRY_CONFIG"] = "/tmp/memry/config.json"

	e.agent(t, "antigravity").Install("https://memry.test")

	assertContents(t, e.path(".gemini", "config", "mcp_config.json"), `{
    "mcpServers": {
        "memry": {
            "command": "/opt/homebrew/opt/memry/bin/memry",
            "args": [
                "mcp"
            ],
            "env": {
                "MEMRY_CONFIG": "/tmp/memry/config.json"
            }
        }
    }
}
`)
}

func TestAntigravityKeepsEveryOtherSettingAndServerAndNonASCIIText(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".gemini", "config", "mcp_config.json")
	writeFile(t, config, `{"mcpServers": {"other": {"serverUrl": "https://x.test/mcp", "headers": {}}}, "note": "Diseño ✓"}`)

	e.agent(t, "antigravity").Install("https://memry.test")

	assertContents(t, config, `{
    "mcpServers": {
        "other": {
            "serverUrl": "https://x.test/mcp",
            "headers": {}
        },
        "memry": `+stdioServer+`
    },
    "note": "Diseño ✓"
}
`)
}

func TestAntigravityLeavesTheSameFilesAfterInstallingTwice(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".gemini", "config", "mcp_config.json"), e.path(".gemini", "config", "GEMINI.md")
	e.agent(t, "antigravity").Install("https://memry.test")
	before, beforeRules := readFile(t, config), readFile(t, rules)

	e.agent(t, "antigravity").Install("https://memry.test")

	assertContents(t, config, before)
	assertContents(t, rules, beforeRules)
}

func TestAntigravityFailsWithManualInstructionsAndLeavesAMalformedMcpConfigJSONUntouched(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".gemini", "config", "mcp_config.json")
	writeFile(t, config, `{"mcpServers": `)

	result := e.agent(t, "antigravity").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertContents(t, config, `{"mcpServers": `)
	assertHasLine(t, result, "error", "Could not register the memry MCP server: memry cannot edit "+config+" safely.")
	assertHasLine(t, result, "line", stdioConfig[:len(stdioConfig)-1])
}

func TestAntigravityAddsTheInstructionsToItsOwnGEMINImdLeavingTheOneGeminiCLIReadsAlone(t *testing.T) {
	e := newTestEnv(t)
	rules := e.path(".gemini", "config", "GEMINI.md")
	writeFile(t, rules, "# My rules\n")
	writeFile(t, e.path(".gemini", "GEMINI.md"), "# Gemini CLI rules\n")

	e.agent(t, "antigravity").Install("https://memry.test")

	assertContents(t, rules, "# My rules\n\n"+memryRules)
	assertContents(t, e.path(".gemini", "GEMINI.md"), "# Gemini CLI rules\n")
}

func TestAntigravityRemovesOnlyTheMemryServerAndInstructionsOnUninstall(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".gemini", "config", "mcp_config.json"), e.path(".gemini", "config", "GEMINI.md")
	writeFile(t, config, `{"mcpServers": {"other": {"command": "x"}}}`)
	writeFile(t, rules, "# My rules\n")
	e.agent(t, "antigravity").Install("https://memry.test")

	result := e.agent(t, "antigravity").Uninstall()

	assertSuccessful(t, result, true)
	assertContents(t, config, "{\n    \"mcpServers\": {\n        \"other\": {\n            \"command\": \"x\"\n        }\n    }\n}\n")
	assertContents(t, rules, "# My rules\n")
}

func TestAntigravitySucceedsWithoutCreatingAnythingWhenThereIsNothingToUninstall(t *testing.T) {
	e := newTestEnv(t)

	result := e.agent(t, "antigravity").Uninstall()

	assertSuccessful(t, result, true)
	if exists(t, e.path(".gemini")) {
		t.Error("~/.gemini was created")
	}
}
