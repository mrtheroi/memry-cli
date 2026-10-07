// Package agents knows the agents memry can be wired into: how to detect
// each one, and how to install and uninstall memry in it.
package agents

import (
	"slices"

	"github.com/mrtheroi/memry-cli/internal/executable"
)

// Registry lists the supported agents, in display order.
type Registry interface {
	// All returns the agents.
	All() []Agent
	// Keys are the agent keys.
	Keys() []string
	// Only returns the agents with the given keys, in display order.
	Only(keys []string) []Agent
}

// Agent is an AI agent memry can be wired into. Adding an agent means one
// type implementing it, listed by New.
type Agent interface {
	// Key is the stable key used in --agents and the config file, e.g.
	// "claude-code".
	Key() string
	// Name is the name shown to the user, e.g. "Claude Code".
	Name() string
	// IsInstalled reports whether the agent is found on this machine.
	IsInstalled() bool
	// Install wires memry into the agent, for the memry server at url.
	Install(url string) Result
	// Uninstall removes memry from the agent.
	Uninstall() Result
}

// Result is the outcome of installing or uninstalling memry in an agent:
// whether it succeeded, and the lines to show the user.
type Result struct {
	Successful bool
	Lines      []Line
}

// Line is a line to show the user, in one of the styles info, warn,
// error or line.
type Line struct {
	Style, Text string
}

// Runner runs a command, like the PHP CLI's Process::run with a list of
// arguments.
type Runner interface {
	// Run runs argv without a shell and reports how it went.
	Run(argv []string) RunResult
}

// RunResult is how running a command went.
type RunResult struct {
	// Started is whether the command started at all.
	Started bool
	// ExitCode is its exit status, -1 when it did not start or was killed.
	ExitCode int
	// TimedOut is whether it was stopped after the runner's timeout.
	TimedOut bool
	// Output is its stdout and stderr, at most maxOutput bytes.
	Output string
}

// Succeeded reports whether the command started and exited with 0.
func (r RunResult) Succeeded() bool {
	return r.Started && !r.TimedOut && r.ExitCode == 0
}

// Env is what the agents run with. Every path derives from Getenv (HOME
// and the agents' own variables), so tests use a temporary home.
type Env struct {
	Getenv func(string) string
	// LookPath finds a command on the PATH, like exec.LookPath.
	LookPath   func(string) (string, error)
	Runner     Runner
	Executable executable.Executable
}

// Supported is the agents memry supports, in display order, like the PHP
// CLI's AgentRegistry.
type Supported struct {
	agents []Agent
}

// New returns the supported agents: Claude Code, Codex, OpenCode,
// Antigravity and Windsurf.
func New(env Env) *Supported {
	return &Supported{agents: []Agent{
		newClaudeCode(env),
		newCodex(env),
		newOpenCode(env),
		newAntigravity(env),
		newWindsurf(env),
	}}
}

// All returns the agents, in display order.
func (s *Supported) All() []Agent {
	return s.agents
}

// Keys returns the agent keys, in display order.
func (s *Supported) Keys() []string {
	keys := make([]string, len(s.agents))
	for i, agent := range s.agents {
		keys[i] = agent.Key()
	}
	return keys
}

// NewSupported returns the given agents, in that order.
func NewSupported(agents ...Agent) *Supported {
	return &Supported{agents: agents}
}

// Only returns the agents with the given keys, in display order.
func (s *Supported) Only(keys []string) []Agent {
	var only []Agent
	for _, agent := range s.agents {
		if slices.Contains(keys, agent.Key()) {
			only = append(only, agent)
		}
	}
	return only
}
