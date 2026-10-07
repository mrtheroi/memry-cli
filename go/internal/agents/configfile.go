package agents

import (
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
	return Result{}
}

// Uninstall removes memry from the agent.
func (a *configFileAgent) Uninstall() Result {
	return Result{}
}

func newCodex(env Env) Agent {
	return &configFileAgent{key: "codex", name: "Codex", env: env}
}

func newOpenCode(env Env) Agent {
	return &configFileAgent{key: "opencode", name: "OpenCode", env: env}
}

func newAntigravity(env Env) Agent {
	return &configFileAgent{key: "antigravity", name: "Antigravity", env: env}
}

func newWindsurf(env Env) Agent {
	return &configFileAgent{key: "windsurf", name: "Windsurf", env: env}
}
