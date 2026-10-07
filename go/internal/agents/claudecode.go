package agents

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
	return false
}

// Install registers the memry MCP server and installs the hook.
func (a *claudeCode) Install(string) Result {
	return Result{}
}

// Uninstall removes the memry MCP servers and the hook.
func (a *claudeCode) Uninstall() Result {
	return Result{}
}
