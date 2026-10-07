package agents_test

import "testing"

// Ported from tests/Feature/Agents/WindsurfAgentTest.php, asserting the
// exact JSON the PHP CLI writes where the PHP tests compare it decoded.

func TestWindsurfRegistersMemryAsAStdioMCPServerInANewMcpConfigJSON(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".codeium", "windsurf", "mcp_config.json")

	result := e.agent(t, "windsurf").Install("https://memry.test")

	assertSuccessful(t, result, true)
	assertContents(t, config, stdioConfig)
	assertHasLine(t, result, "info", "Registered the memry MCP server in "+config+".")
}

func TestWindsurfIsInstalledWhenCodeiumWindsurfExists(t *testing.T) {
	e := newTestEnv(t)
	if e.agent(t, "windsurf").IsInstalled() {
		t.Error("IsInstalled = true without ~/.codeium/windsurf")
	}

	mkdir(t, e.path(".codeium", "windsurf"))

	if !e.agent(t, "windsurf").IsInstalled() {
		t.Error("IsInstalled = false with ~/.codeium/windsurf")
	}
}

func TestWindsurfPassesACustomMEMRY_CONFIGToTheServerThroughItsEnvironment(t *testing.T) {
	e := newTestEnv(t)
	e.vars["MEMRY_CONFIG"] = "/tmp/memry/config.json"

	e.agent(t, "windsurf").Install("https://memry.test")

	assertContents(t, e.path(".codeium", "windsurf", "mcp_config.json"), `{
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

func TestWindsurfKeepsEveryOtherSettingAndServerAndNonASCIIText(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".codeium", "windsurf", "mcp_config.json")
	writeFile(t, config, `{"mcpServers": {"other": {"serverUrl": "https://x.test/mcp", "env": {}}}, "note": "Diseño ✓"}`)

	e.agent(t, "windsurf").Install("https://memry.test")

	assertContents(t, config, `{
    "mcpServers": {
        "other": {
            "serverUrl": "https://x.test/mcp",
            "env": {}
        },
        "memry": `+stdioServer+`
    },
    "note": "Diseño ✓"
}
`)
}

func TestWindsurfLeavesTheSameFilesAfterInstallingTwice(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".codeium", "windsurf", "mcp_config.json"), e.path(".codeium", "windsurf", "memories", "global_rules.md")
	e.agent(t, "windsurf").Install("https://memry.test")
	before, beforeRules := readFile(t, config), readFile(t, rules)

	e.agent(t, "windsurf").Install("https://memry.test")

	assertContents(t, config, before)
	assertContents(t, rules, beforeRules)
}

func TestWindsurfFailsWithManualInstructionsAndLeavesAMalformedMcpConfigJSONUntouched(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".codeium", "windsurf", "mcp_config.json")
	writeFile(t, config, `{"mcpServers": []}`)

	result := e.agent(t, "windsurf").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertContents(t, config, `{"mcpServers": []}`)
	assertHasLine(t, result, "error", "Could not register the memry MCP server: memry cannot edit "+config+" safely.")
}

func TestWindsurfAddsTheMemryInstructionsToTheGlobalRulesKeepingTheOtherRules(t *testing.T) {
	e := newTestEnv(t)
	rules := e.path(".codeium", "windsurf", "memories", "global_rules.md")
	writeFile(t, rules, "# My rules\n")

	e.agent(t, "windsurf").Install("https://memry.test")

	assertContents(t, rules, "# My rules\n\n"+memryRules)
}

func TestWindsurfRemovesOnlyTheMemryServerAndInstructionsOnUninstall(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".codeium", "windsurf", "mcp_config.json"), e.path(".codeium", "windsurf", "memories", "global_rules.md")
	writeFile(t, config, `{"mcpServers": {"other": {"command": "x"}}}`)
	writeFile(t, rules, "# My rules\n")
	e.agent(t, "windsurf").Install("https://memry.test")

	result := e.agent(t, "windsurf").Uninstall()

	assertSuccessful(t, result, true)
	assertContents(t, config, "{\n    \"mcpServers\": {\n        \"other\": {\n            \"command\": \"x\"\n        }\n    }\n}\n")
	assertContents(t, rules, "# My rules\n")
}

func TestWindsurfSucceedsWithoutCreatingAnythingWhenThereIsNothingToUninstall(t *testing.T) {
	e := newTestEnv(t)

	result := e.agent(t, "windsurf").Uninstall()

	assertSuccessful(t, result, true)
	if exists(t, e.path(".codeium")) {
		t.Error("~/.codeium was created")
	}
}
