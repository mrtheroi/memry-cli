package agents

import (
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// mcpServer is the name memry's MCP server is registered under.
const mcpServer = "memry"

// configFileAgent is an agent memry is wired into by editing its files:
// the memry MCP server, run over stdio with `memry mcp`, in its config
// file, and the memry Protocol in its global instructions, as it has no
// SessionStart hook. Like the PHP CLI's ConfigFileAgent, the paths are
// resolved on every use, from the environment.
type configFileAgent struct {
	key, name string
	env       Env
	installed func() bool
	mcpConfig func() agentfiles.MCPConfig
	server    func() *phpjson.Object
	rulesFile func() *agentfiles.RulesFile
}

func (a *configFileAgent) Key() string       { return a.key }
func (a *configFileAgent) Name() string      { return a.name }
func (a *configFileAgent) IsInstalled() bool { return a.installed() }

// Install wires memry into the agent. The server URL is not written
// anywhere: `memry mcp` reads it, with the token, from the memry config.
func (a *configFileAgent) Install(string) Result {
	var lines lines
	// The instructions only need their own file, so write them even when registration fails.
	registered := a.registerMCPServer(&lines)
	instructed := a.addInstructions(&lines)
	return Result{Successful: registered && instructed, Lines: lines}
}

func (a *configFileAgent) addInstructions(lines *lines) bool {
	rules := a.rulesFile()
	put, err := rules.Put(Protocol)
	if err != nil {
		lines.say("error", "Could not add the memry instructions: "+err.Error()+".")
		return false
	}
	if !put {
		lines.say("error", brokenMarkers("Could not add the memry instructions", rules))
		lines.say("line", "Fix or remove them, then run `memry setup` again.")
		return false
	}
	lines.say("info", "Added the memry instructions to "+rules.Path()+".")
	return true
}

func brokenMarkers(failure string, rules *agentfiles.RulesFile) string {
	return failure + ": the " + agentfiles.RulesStart + " and " + agentfiles.RulesEnd + " markers in " + rules.Path() + " do not enclose one block."
}

func (a *configFileAgent) registerMCPServer(lines *lines) bool {
	config := a.mcpConfig()
	put, err := config.Put(mcpServer, a.server())
	if err != nil {
		lines.say("error", "Could not register the "+mcpServer+" MCP server: "+err.Error()+".")
		return false
	}
	if !put {
		lines.say("error", "Could not register the "+mcpServer+" MCP server: memry cannot edit "+config.Path()+" safely.")
		lines.say("line", "Fix the file, then add this to it by hand:")
		lines.say("line", strings.TrimRight(config.Snippet(mcpServer, a.server()), " \t\n\r\x00\x0B"))
		return false
	}
	lines.say("info", "Registered the "+mcpServer+" MCP server in "+config.Path()+".")
	return true
}

// commandServer is the server as the common {command, args, env} entry.
func (a *configFileAgent) commandServer() *phpjson.Object {
	arguments := a.env.Executable.Arguments("mcp")
	return a.withEnvironment(phpjson.NewObject("command", arguments[0], "args", arguments[1:]), "env")
}

// withEnvironment adds the environment `memry mcp` needs, if any, to
// server under key.
func (a *configFileAgent) withEnvironment(server *phpjson.Object, key string) *phpjson.Object {
	environment := a.env.Executable.Environment()
	if len(environment) == 0 {
		return server
	}
	vars := phpjson.NewObject()
	for _, name := range slices.Sorted(maps.Keys(environment)) {
		vars.Set(name, environment[name])
	}
	server.Set(key, vars)
	return server
}

// Uninstall removes memry from the agent.
func (a *configFileAgent) Uninstall() Result {
	var lines lines
	// Each step runs even when the other one fails.
	removed := a.removeMCPServer(&lines)
	uninstructed := a.removeInstructions(&lines)
	return Result{Successful: removed && uninstructed, Lines: lines}
}

func (a *configFileAgent) removeMCPServer(lines *lines) bool {
	config := a.mcpConfig()
	removed, err := config.Remove(mcpServer)
	switch {
	case err != nil:
		lines.say("warn", "Could not remove the "+mcpServer+" MCP server: "+err.Error()+".")
		return false
	case removed == agentfiles.Unsafe:
		lines.say("warn", "Could not remove the "+mcpServer+" MCP server: memry cannot edit "+config.Path()+" safely.")
		return false
	case removed == agentfiles.Removed:
		lines.say("info", "Removed the "+mcpServer+" MCP server from "+config.Path()+".")
	default:
		lines.say("line", "No "+mcpServer+" MCP server to remove from "+config.Path()+".")
	}
	return true
}

func (a *configFileAgent) removeInstructions(lines *lines) bool {
	rules := a.rulesFile()
	removed, err := rules.Remove()
	switch {
	case err != nil:
		lines.say("warn", "Could not remove the memry instructions: "+err.Error()+".")
		return false
	case removed == agentfiles.Unsafe:
		lines.say("warn", brokenMarkers("Could not remove the memry instructions", rules))
		return false
	case removed == agentfiles.Removed:
		lines.say("info", "Removed the memry instructions from "+rules.Path()+".")
	default:
		lines.say("line", "No memry instructions to remove from "+rules.Path()+".")
	}
	return true
}

// newCodex is OpenAI Codex CLI: the memry MCP server in
// $CODEX_HOME/config.toml and the memry instructions in
// $CODEX_HOME/AGENTS.md.
func newCodex(env Env) Agent {
	// $CODEX_HOME when it is an absolute path, as Codex requires, or ~/.codex.
	home := func() string {
		if home := env.Getenv("CODEX_HOME"); strings.HasPrefix(home, "/") {
			return strings.TrimRight(home, "/")
		}
		return env.Getenv("HOME") + "/.codex"
	}
	a := &configFileAgent{key: "codex", name: "Codex", env: env}
	a.mcpConfig = func() agentfiles.MCPConfig { return agentfiles.NewTOMLConfig(home() + "/config.toml") }
	a.server = a.commandServer
	a.rulesFile = func() *agentfiles.RulesFile { return agentfiles.NewRulesFile(home() + "/AGENTS.md") }
	a.installed = func() bool { return env.onPath("codex") || isDir(home()) }
	return a
}

// newOpenCode is OpenCode: the memry MCP server in
// ~/.config/opencode/opencode.json and the memry instructions in
// ~/.config/opencode/AGENTS.md.
func newOpenCode(env Env) Agent {
	// $XDG_CONFIG_HOME/opencode, or ~/.config/opencode, as OpenCode resolves it.
	directory := func() string {
		if config := env.Getenv("XDG_CONFIG_HOME"); config != "" {
			return config + "/opencode"
		}
		return env.Getenv("HOME") + "/.config/opencode"
	}
	a := &configFileAgent{key: "opencode", name: "OpenCode", env: env}
	// opencode.json, or else an existing opencode.jsonc, like `opencode mcp
	// add` picks it. A .jsonc file with comments cannot be parsed, so it is
	// left untouched with manual instructions instead.
	a.mcpConfig = func() agentfiles.MCPConfig {
		path := directory() + "/opencode.json"
		if jsonc := directory() + "/opencode.jsonc"; !isFile(path) && isFile(jsonc) {
			path = jsonc
		}
		return agentfiles.NewJSONConfig(path, "mcp")
	}
	a.server = func() *phpjson.Object {
		return a.withEnvironment(phpjson.NewObject("type", "local", "command", env.Executable.Arguments("mcp"), "enabled", true), "environment")
	}
	a.rulesFile = func() *agentfiles.RulesFile { return agentfiles.NewRulesFile(directory() + "/AGENTS.md") }
	a.installed = func() bool { return env.onPath("opencode") || isDir(directory()) }
	return a
}

func newAntigravity(env Env) Agent {
	return &configFileAgent{key: "antigravity", name: "Antigravity", env: env}
}

func newWindsurf(env Env) Agent {
	return &configFileAgent{key: "windsurf", name: "Windsurf", env: env}
}

// lines are the lines a Result shows.
type lines []Line

func (l *lines) say(style, text string) {
	*l = append(*l, Line{Style: style, Text: text})
}

// onPath reports whether the command name is on the PATH.
func (env Env) onPath(name string) bool {
	_, err := env.LookPath(name)
	return err == nil
}

// isDir reports whether path is a directory, like PHP's is_dir.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isFile reports whether path is a regular file, like PHP's is_file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
