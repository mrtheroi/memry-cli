package agents

import (
	"strconv"
	"strings"

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

	// A server that was not registered is fine to "remove"; a remove that
	// could not run leaves a registration memry must not add over.
	for _, name := range []string{legacyMCPServer, mcpServer} {
		if _, failure := a.removeServer(name); failure != "" {
			lines.say("error", "Could not register the "+mcpServer+" MCP server in Claude Code: "+failure)
			return failWithManualRegistration(string(server), lines)
		}
	}
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
	return a.env.Runner.Run(append([]string{"claude"}, args...)).Succeeded()
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
// earlier versions. A server that is not registered is fine; any other
// failure to remove one fails the uninstall (see removeServer).
func (a *claudeCode) removeMCPServers(lines *lines) bool {
	if !a.IsInstalled() {
		lines.say("warn", "Claude Code CLI not found; skipped removing the "+mcpServer+" MCP server.")
		return false
	}
	ok := true
	if _, failure := a.removeServer(legacyMCPServer); failure != "" {
		lines.say("warn", "Could not remove the "+legacyMCPServer+" MCP server from Claude Code: "+failure)
		ok = false
	}
	switch removed, failure := a.removeServer(mcpServer); {
	case failure != "":
		lines.say("warn", "Could not remove the "+mcpServer+" MCP server from Claude Code: "+failure)
		return false
	case removed:
		lines.say("info", "Removed the "+mcpServer+" MCP server from Claude Code.")
	default:
		lines.say("line", "The "+mcpServer+" MCP server was not registered in Claude Code.")
	}
	return ok
}

// removeServer runs `claude mcp remove` for the user-scope server name. It
// returns whether the server was removed, or why the removal failed:
//
//   - exit 0: removed;
//   - a non-zero exit whose output contains "No MCP server found" (in any
//     case): the server was not registered, as in the PHP CLI;
//   - anything else (another non-zero exit, a command that could not start
//     or timed out): a failure, with an excerpt of the output.
//
// The PHP CLI takes every failure for "not registered". The trade-off: if
// Claude Code changes that message, removing a server that is not there
// reports a false failure, which never claims a removal that did not
// happen.
func (a *claudeCode) removeServer(name string) (removed bool, failure string) {
	result := a.env.Runner.Run([]string{"claude", "mcp", "remove", "--scope", "user", name})
	switch {
	case result.Succeeded():
		return true, ""
	case !result.Started:
		return false, "`claude mcp remove` could not start."
	case result.TimedOut:
		return false, "`claude mcp remove` timed out" + excerpt(result.Output) + "."
	case strings.Contains(strings.ToLower(result.Output), "no mcp server found"):
		return false, ""
	}
	return false, "`claude mcp remove` exited with " + strconv.Itoa(result.ExitCode) + excerpt(result.Output) + "."
}

// excerptLength is how many characters of a command's output a failure
// shows.
const excerptLength = 200

// excerpt is the start of output on one line, in parentheses after a
// space, or nothing when there is no output.
func excerpt(output string) string {
	text := []rune(strings.Join(strings.Fields(output), " "))
	if len(text) == 0 {
		return ""
	}
	if len(text) > excerptLength {
		return " (" + string(text[:excerptLength]) + "…)"
	}
	return " (" + string(text) + ")"
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
