package agents_test

import "testing"

// Ported from tests/Feature/Agents/OpenCodeAgentTest.php, asserting the
// exact JSON the PHP CLI writes where the PHP tests compare it decoded.

const openCodeServer = `{
            "type": "local",
            "command": [
                "/opt/homebrew/opt/memry/bin/memry",
                "mcp"
            ],
            "enabled": true
        }`

func TestOpenCodeRegistersMemryAsALocalMCPServerInANewOpencodeJSON(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".config", "opencode", "opencode.json")

	result := e.agent(t, "opencode").Install("https://memry.test")

	assertSuccessful(t, result, true)
	assertContents(t, config, "{\n    \"mcp\": {\n        \"memry\": "+openCodeServer+"\n    }\n}\n")
	assertHasLine(t, result, "info", "Registered the memry MCP server in "+config+".")
}

func TestOpenCodePassesACustomMEMRY_CONFIGToTheServerThroughItsEnvironment(t *testing.T) {
	e := newTestEnv(t)
	e.vars["MEMRY_CONFIG"] = "/tmp/memry/config.json"

	e.agent(t, "opencode").Install("https://memry.test")

	assertContents(t, e.path(".config", "opencode", "opencode.json"), `{
    "mcp": {
        "memry": {
            "type": "local",
            "command": [
                "/opt/homebrew/opt/memry/bin/memry",
                "mcp"
            ],
            "enabled": true,
            "environment": {
                "MEMRY_CONFIG": "/tmp/memry/config.json"
            }
        }
    }
}
`)
}

func TestOpenCodeUsesXDG_CONFIG_HOMEOpencodeWhenXDG_CONFIG_HOMEIsSet(t *testing.T) {
	e := newTestEnv(t)
	e.vars["XDG_CONFIG_HOME"] = e.path("xdg")

	e.agent(t, "opencode").Install("https://memry.test")

	if !exists(t, e.path("xdg", "opencode", "opencode.json")) || !exists(t, e.path("xdg", "opencode", "AGENTS.md")) {
		t.Error("the files were not written in $XDG_CONFIG_HOME/opencode")
	}
	if exists(t, e.path(".config", "opencode", "opencode.json")) {
		t.Error("~/.config/opencode/opencode.json was written")
	}
}

func TestOpenCodeEditsAnExistingOpencodeJSONCWhenThereIsNoOpencodeJSON(t *testing.T) {
	e := newTestEnv(t)
	jsonc := e.path(".config", "opencode", "opencode.jsonc")
	writeFile(t, jsonc, `{"theme": "dark"}`)

	e.agent(t, "opencode").Install("https://memry.test")

	assertContents(t, jsonc, "{\n    \"theme\": \"dark\",\n    \"mcp\": {\n        \"memry\": "+openCodeServer+"\n    }\n}\n")
	if exists(t, e.path(".config", "opencode", "opencode.json")) {
		t.Error("opencode.json was created")
	}
}

func TestOpenCodeKeepsEveryOtherSettingAndServerAndNonASCIIText(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".config", "opencode", "opencode.json")
	writeFile(t, config, `{"$schema": "https://opencode.ai/config.json", "theme": "Diseño ✓", "mcp": {"other": {"type": "remote", "url": "https://x.test/mcp", "headers": {}}}}`)

	e.agent(t, "opencode").Install("https://memry.test")

	assertContents(t, config, `{
    "$schema": "https://opencode.ai/config.json",
    "theme": "Diseño ✓",
    "mcp": {
        "other": {
            "type": "remote",
            "url": "https://x.test/mcp",
            "headers": {}
        },
        "memry": `+openCodeServer+`
    }
}
`)
}

func TestOpenCodeLeavesTheSameFilesAfterInstallingTwice(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".config", "opencode", "opencode.json"), e.path(".config", "opencode", "AGENTS.md")
	e.agent(t, "opencode").Install("https://memry.test")
	before, beforeRules := readFile(t, config), readFile(t, rules)

	e.agent(t, "opencode").Install("https://memry.test")

	assertContents(t, config, before)
	assertContents(t, rules, beforeRules)
}

func TestOpenCodeFailsWithManualInstructionsAndLeavesAnOpencodeJSONCWithCommentsUntouched(t *testing.T) {
	e := newTestEnv(t)
	jsonc := e.path(".config", "opencode", "opencode.jsonc")
	contents := "{\n  // my theme\n  \"theme\": \"dark\"\n}\n"
	writeFile(t, jsonc, contents)

	result := e.agent(t, "opencode").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertContents(t, jsonc, contents)
	assertHasLine(t, result, "error", "Could not register the memry MCP server: memry cannot edit "+jsonc+" safely.")
	assertHasLine(t, result, "line", "{\n    \"mcp\": {\n        \"memry\": "+openCodeServer+"\n    }\n}")
}

func TestOpenCodeAddsTheMemryInstructionsToAGENTSmdKeepingTheOtherInstructions(t *testing.T) {
	e := newTestEnv(t)
	rules := e.path(".config", "opencode", "AGENTS.md")
	writeFile(t, rules, "# My rules\n")

	e.agent(t, "opencode").Install("https://memry.test")

	assertContents(t, rules, "# My rules\n\n"+memryRules)
}

func TestOpenCodeRemovesOnlyTheMemryServerAndInstructionsOnUninstall(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".config", "opencode", "opencode.json"), e.path(".config", "opencode", "AGENTS.md")
	writeFile(t, config, `{"theme": "dark", "mcp": {"other": {"type": "local", "command": ["x"]}}}`)
	writeFile(t, rules, "# My rules\n")
	e.agent(t, "opencode").Install("https://memry.test")

	result := e.agent(t, "opencode").Uninstall()

	assertSuccessful(t, result, true)
	assertContents(t, config, "{\n    \"theme\": \"dark\",\n    \"mcp\": {\n        \"other\": {\n            \"type\": \"local\",\n            \"command\": [\n                \"x\"\n            ]\n        }\n    }\n}\n")
	assertContents(t, rules, "# My rules\n")
}

func TestOpenCodeSucceedsWithoutCreatingAnythingWhenThereIsNothingToUninstall(t *testing.T) {
	e := newTestEnv(t)

	result := e.agent(t, "opencode").Uninstall()

	assertSuccessful(t, result, true)
	if exists(t, e.path(".config", "opencode")) {
		t.Error("~/.config/opencode was created")
	}
}

func TestOpenCodeIsInstalledWhenTheOpencodeCLIIsOnThePATHOrItsConfigDirectoryExists(t *testing.T) {
	e := newTestEnv(t)
	if e.agent(t, "opencode").IsInstalled() {
		t.Error("IsInstalled = true with neither")
	}

	e.onPath = []string{"opencode"}
	if !e.agent(t, "opencode").IsInstalled() {
		t.Error("IsInstalled = false with opencode on the PATH")
	}

	e.onPath = nil
	mkdir(t, e.path(".config", "opencode"))
	if !e.agent(t, "opencode").IsInstalled() {
		t.Error("IsInstalled = false with ~/.config/opencode")
	}
}
