package agents

import (
	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/executable"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// legacyMCPServer is the name of the MCP server earlier versions
// registered in Claude Code.
const legacyMCPServer = "db-memory"

// claudeCode is Claude Code: the user-scope memry MCP server, registered
// with the `claude` CLI, and a SessionStart hook in its settings.json.
type claudeCode struct {
	env Env
}

func newClaudeCode(env Env) Agent {
	return &claudeCode{env: env}
}

func (a *claudeCode) Key() string  { return "claude-code" }
func (a *claudeCode) Name() string { return "Claude Code" }

// IsInstalled reports whether the claude CLI is on the PATH.
func (a *claudeCode) IsInstalled() bool {
	return a.env.onPath("claude")
}

// Install registers the memry MCP server and installs the hook.
func (a *claudeCode) Install(url string) Result {
	var lines lines
	// The hook only needs the config file, so install it even when MCP registration fails.
	registered := a.registerMCPServer(url, &lines)
	installed := a.installSessionStartHook(&lines)
	return Result{Successful: registered && installed, Lines: lines}
}

// hookMarker identifies memry's SessionStart hook by its command.
const hookMarker = "hook:session-start"

// settings is Claude Code's settings.json: $CLAUDE_CONFIG_DIR/settings.json,
// or ~/.claude/settings.json by default.
func (a *claudeCode) settings() *agentfiles.ClaudeSettings {
	dir := a.env.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		dir = a.env.Getenv("HOME") + "/.claude"
	}
	return agentfiles.NewClaudeSettings(dir + "/settings.json")
}

// installSessionStartHook installs the Claude Code SessionStart hook that
// prints the memry context of the current project.
func (a *claudeCode) installSessionStartHook(lines *lines) bool {
	settings := a.settings()
	group := phpjson.NewObject(
		"matcher", "startup|resume|clear|compact",
		"hooks", []any{phpjson.NewObject("type", "command", "command", a.env.Executable.Command(hookMarker), "timeout", 10)},
	)
	installed, err := settings.ReplaceSessionStartHook(group, hookMarker)
	if err != nil {
		lines.say("error", "Could not install the memry SessionStart hook: "+err.Error()+".")
		return false
	}
	if !installed {
		lines.say("error", "Could not install the memry SessionStart hook: "+settings.Path()+" is not valid JSON.")
		snippet, ok := phpjson.Encode(group, phpjson.PrettyPrint|phpjson.UnescapedSlashes)
		if !ok {
			lines.say("line", fixNotUTF8)
			return false
		}
		lines.say("line", `Fix the file, then add this group to the "hooks.SessionStart" array by hand:`)
		lines.say("line", string(snippet))
		return false
	}
	lines.say("info", "Installed the memry SessionStart hook in "+settings.Path()+".")
	return true
}

// registerMCPServer registers memry as the user-scope memry MCP server in
// Claude Code. The token stays in the config file; Claude Code gets it
// from the headers helper.
func (a *claudeCode) registerMCPServer(url string, lines *lines) bool {
	server, ok := phpjson.Encode(phpjson.NewObject(
		"type", "http",
		"url", url+"/mcp/memory",
		"headersHelper", a.env.Executable.Command("mcp-headers"),
	), phpjson.UnescapedSlashes)
	// Checked before any claude command: removing the registration and
	// then adding an empty one would delete a working server.
	if !ok {
		lines.say("error", "Could not register the "+mcpServer+" MCP server in Claude Code: "+notUTF8)
		lines.say("line", fixNotUTF8)
		return false
	}

	if !a.IsInstalled() {
		lines.say("warn", "Claude Code CLI not found; skipped MCP registration.")
		return failWithManualRegistration(string(server), lines)
	}

	a.claude("mcp", "remove", "--scope", "user", legacyMCPServer)
	a.claude("mcp", "remove", "--scope", "user", mcpServer)
	if !a.claude("mcp", "add-json", "--scope", "user", mcpServer, string(server)) || !a.claude("mcp", "get", mcpServer) {
		lines.say("error", "Could not register the "+mcpServer+" MCP server in Claude Code.")
		return failWithManualRegistration(string(server), lines)
	}
	lines.say("info", "Registered the "+mcpServer+" MCP server in Claude Code (user scope).")
	return true
}

// What memry says when a command it writes for an agent cannot be
// encoded as JSON. PHP's json_encode fails on the same values.
const (
	notUTF8    = "the memry executable path or MEMRY_CONFIG is not valid UTF-8."
	fixNotUTF8 = "Set them to valid UTF-8 paths, then run `memry setup` again."
)

func failWithManualRegistration(server string, lines *lines) bool {
	lines.say("line", "Your login was saved. Register the server manually with:")
	lines.say("line", "  claude mcp add-json --scope user "+mcpServer+" "+executable.EscapeShellArg(server))
	return false
}

// claude runs the claude CLI with args, reporting whether it succeeded.
func (a *claudeCode) claude(args ...string) bool {
	return a.env.Runner.Run(append([]string{"claude"}, args...))
}

// Uninstall removes the memry MCP servers and the hook.
func (a *claudeCode) Uninstall() Result {
	var lines lines
	// Each step runs even when the other one fails.
	removed := a.removeMCPServers(&lines)
	unhooked := a.removeSessionStartHook(&lines)
	return Result{Successful: removed && unhooked, Lines: lines}
}

// removeMCPServers removes the memry MCP server and the legacy one of
// earlier versions. Failing to remove one only means it was not
// registered.
func (a *claudeCode) removeMCPServers(lines *lines) bool {
	if !a.IsInstalled() {
		lines.say("warn", "Claude Code CLI not found; skipped removing the "+mcpServer+" MCP server.")
		return false
	}
	a.claude("mcp", "remove", "--scope", "user", legacyMCPServer)
	if a.claude("mcp", "remove", "--scope", "user", mcpServer) {
		lines.say("info", "Removed the "+mcpServer+" MCP server from Claude Code.")
	} else {
		lines.say("line", "The "+mcpServer+" MCP server was not registered in Claude Code.")
	}
	return true
}

// removeSessionStartHook removes the hook setup installed, identified
// like setup does by the hook:session-start command.
func (a *claudeCode) removeSessionStartHook(lines *lines) bool {
	settings := a.settings()
	removed, err := settings.RemoveSessionStartHook(hookMarker)
	switch {
	case err != nil:
		lines.say("warn", "Could not remove the memry SessionStart hook: "+err.Error()+".")
		return false
	case removed == agentfiles.Unsafe:
		lines.say("warn", "Could not remove the memry SessionStart hook: "+settings.Path()+" is not valid JSON.")
		return false
	case removed == agentfiles.Removed:
		lines.say("info", "Removed the memry SessionStart hook from "+settings.Path()+".")
	default:
		lines.say("line", "No memry SessionStart hook to remove.")
	}
	return true
}
