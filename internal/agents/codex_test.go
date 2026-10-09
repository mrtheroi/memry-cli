package agents_test

import (
	"runtime"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

// Ported from tests/Feature/Agents/CodexAgentTest.php.

const codexTable = "[mcp_servers.memry]\ncommand = \"/opt/homebrew/opt/memry/bin/memry\"\nargs = [\"mcp\"]\n"

const memryRules = "<!-- memry:start -->\n" + agents.Protocol + "\n<!-- memry:end -->\n"

func TestCodexRegistersMemryAsAStdioMCPServerInANewConfigToml(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".codex", "config.toml")

	result := e.agent(t, "codex").Install("https://memry.test")

	assertSuccessful(t, result, true)
	assertContents(t, config, codexTable)
	assertHasLine(t, result, "info", "Registered the memry MCP server in "+config+".")
}

func TestCodexPassesACustomMEMRY_CONFIGToTheServerThroughItsEnvironment(t *testing.T) {
	e := newTestEnv(t)
	e.vars["MEMRY_CONFIG"] = "/tmp/memry/config.json"

	e.agent(t, "codex").Install("https://memry.test")

	assertContents(t, e.path(".codex", "config.toml"), codexTable+"env = { MEMRY_CONFIG = \"/tmp/memry/config.json\" }\n")
}

func TestCodexUsesCODEX_HOMEWhenItIsAnAbsolutePath(t *testing.T) {
	tests := map[string]struct {
		codexHome func(home string) string
		want      []string
	}{
		"absolute": {func(home string) string { return home + "/custom-codex" }, []string{"custom-codex", "config.toml"}},
		"relative": {func(string) string { return "custom-codex" }, []string{".codex", "config.toml"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t)
			e.vars["CODEX_HOME"] = tt.codexHome(e.home)

			e.agent(t, "codex").Install("https://memry.test")

			if !exists(t, e.path(tt.want...)) {
				t.Errorf("%s was not written", e.path(tt.want...))
			}
		})
	}
}

func TestCodexAddsTheMemryInstructionsToAGENTSmdKeepingTheOtherInstructions(t *testing.T) {
	e := newTestEnv(t)
	rules := e.path(".codex", "AGENTS.md")
	writeFile(t, rules, "# My rules\n\nBe concise. Diseño ✓\n")

	result := e.agent(t, "codex").Install("https://memry.test")

	assertContents(t, rules, "# My rules\n\nBe concise. Diseño ✓\n\n"+memryRules)
	assertHasLine(t, result, "info", "Added the memry instructions to "+rules+".")
}

func TestCodexFailsWithManualInstructionsAndLeavesAConfigTomlItCannotEditUntouched(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".codex", "config.toml")
	contents := "model = \"o3\n"
	writeFile(t, config, contents)

	result := e.agent(t, "codex").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertContents(t, config, contents)
	assertHasLine(t, result, "error", "Could not register the memry MCP server: memry cannot edit "+config+" safely.")
	assertHasLine(t, result, "line", "Fix the file, then add this to it by hand:")
	assertHasLine(t, result, "line", codexTable[:len(codexTable)-1])
	if !exists(t, e.path(".codex", "AGENTS.md")) {
		t.Error("the instructions were not written")
	}
}

func TestCodexFailsAndLeavesTheInstructionsUntouchedWhenTheirMarkersAreBroken(t *testing.T) {
	e := newTestEnv(t)
	rules := e.path(".codex", "AGENTS.md")
	contents := "# My rules\n<!-- memry:start -->\nOld.\n"
	writeFile(t, rules, contents)

	result := e.agent(t, "codex").Install("https://memry.test")

	assertSuccessful(t, result, false)
	assertContents(t, rules, contents)
	assertHasLine(t, result, "error", "Could not add the memry instructions: the <!-- memry:start --> and <!-- memry:end --> markers in "+rules+" do not enclose one block.")
	assertHasLine(t, result, "line", "Fix or remove them, then run `memry setup` again.")
	assertContents(t, e.path(".codex", "config.toml"), codexTable)
}

func TestCodexKeepsEveryOtherSettingAndServerByteForByte(t *testing.T) {
	e := newTestEnv(t)
	config := e.path(".codex", "config.toml")
	existing := "# Diseño ✓\nmodel = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\nargs = [\"-y\", \"other\"]\n"
	writeFile(t, config, existing)

	e.agent(t, "codex").Install("https://memry.test")

	assertContents(t, config, existing+"\n"+codexTable)
}

func TestCodexLeavesTheSameFilesAfterInstallingTwice(t *testing.T) {
	e := newTestEnv(t)
	e.agent(t, "codex").Install("https://memry.test")
	config := readFile(t, e.path(".codex", "config.toml"))
	rules := readFile(t, e.path(".codex", "AGENTS.md"))

	result := e.agent(t, "codex").Install("https://memry.test")

	assertSuccessful(t, result, true)
	assertContents(t, e.path(".codex", "config.toml"), config)
	assertContents(t, e.path(".codex", "AGENTS.md"), rules)
}

func TestCodexRemovesOnlyTheMemryServerAndInstructionsOnUninstall(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".codex", "config.toml"), e.path(".codex", "AGENTS.md")
	writeFile(t, config, "model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\n")
	writeFile(t, rules, "# My rules\n")
	e.agent(t, "codex").Install("https://memry.test")

	result := e.agent(t, "codex").Uninstall()

	assertSuccessful(t, result, true)
	assertContents(t, config, "model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\n")
	assertContents(t, rules, "# My rules\n")
	assertLines(t, result,
		[2]string{"info", "Removed the memry MCP server from " + config + "."},
		[2]string{"info", "Removed the memry instructions from " + rules + "."},
	)
}

func TestCodexSucceedsWithoutCreatingAnythingWhenThereIsNothingToUninstall(t *testing.T) {
	e := newTestEnv(t)

	result := e.agent(t, "codex").Uninstall()

	assertSuccessful(t, result, true)
	if exists(t, e.path(".codex")) {
		t.Error("~/.codex was created")
	}
	assertLines(t, result,
		[2]string{"line", "No memry MCP server to remove from " + e.path(".codex", "config.toml") + "."},
		[2]string{"line", "No memry instructions to remove from " + e.path(".codex", "AGENTS.md") + "."},
	)
}

func TestCodexWarnsFailsAndLeavesFilesItCannotEditUntouchedOnUninstall(t *testing.T) {
	e := newTestEnv(t)
	config, rules := e.path(".codex", "config.toml"), e.path(".codex", "AGENTS.md")
	writeFile(t, config, "model = \"o3\n")
	writeFile(t, rules, "<!-- memry:start -->\nOld.\n")

	result := e.agent(t, "codex").Uninstall()

	assertSuccessful(t, result, false)
	assertContents(t, config, "model = \"o3\n")
	assertContents(t, rules, "<!-- memry:start -->\nOld.\n")
	assertLines(t, result,
		[2]string{"warn", "Could not remove the memry MCP server: memry cannot edit " + config + " safely."},
		[2]string{"warn", "Could not remove the memry instructions: the <!-- memry:start --> and <!-- memry:end --> markers in " + rules + " do not enclose one block."},
	)
}

func TestCodexIsInstalledWhenTheCodexCLIIsOnThePATHOrItsHomeDirectoryExists(t *testing.T) {
	e := newTestEnv(t)
	if e.agent(t, "codex").IsInstalled() {
		t.Error("IsInstalled = true with neither")
	}

	e.onPath = []string{"codex"}
	if !e.agent(t, "codex").IsInstalled() {
		t.Error("IsInstalled = false with codex on the PATH")
	}

	e.onPath = nil
	mkdir(t, e.path(".codex"))
	if !e.agent(t, "codex").IsInstalled() {
		t.Error("IsInstalled = false with ~/.codex")
	}
}

// Not in the PHP CLI, which throws when it cannot read or write a file:
// each step fails on its own, saying why.
func TestCodexFailsEachStepItCannotReadOrWriteTheFilesOf(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ENOTDIR error text; Windows reports ERROR_PATH_NOT_FOUND")
	}
	e := newTestEnv(t)
	writeFile(t, e.path(".codex"), "a file, not a directory")

	install := e.agent(t, "codex").Install("https://memry.test")
	uninstall := e.agent(t, "codex").Uninstall()

	assertSuccessful(t, install, false)
	assertSuccessful(t, uninstall, false)
	config, rules := e.path(".codex", "config.toml"), e.path(".codex", "AGENTS.md")
	assertLines(t, install,
		[2]string{"error", "Could not register the memry MCP server: open " + config + ": not a directory."},
		[2]string{"error", "Could not add the memry instructions: open " + rules + ": not a directory."},
	)
	assertLines(t, uninstall,
		[2]string{"warn", "Could not remove the memry MCP server: open " + config + ": not a directory."},
		[2]string{"warn", "Could not remove the memry instructions: open " + rules + ": not a directory."},
	)
}
