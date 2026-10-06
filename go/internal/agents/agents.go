// Package agents knows the agents memry can be wired into.
package agents

// Registry lists the supported agents and detects the installed ones.
type Registry interface {
	// Keys are the agent keys, in display order.
	Keys() []string
	// IsInstalled reports whether the agent is found on this machine.
	IsInstalled(key string) bool
}

// Stub is the Registry until the agents are ported: it lists the keys of
// the PHP CLI, in its order, and detects none of them as installed.
type Stub struct{}

// Keys returns the keys of the PHP CLI's agents.
func (Stub) Keys() []string {
	return []string{"claude-code", "codex", "opencode", "antigravity", "windsurf"}
}

// IsInstalled is always false: detection comes with the agents.
func (Stub) IsInstalled(string) bool { return false }
