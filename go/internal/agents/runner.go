package agents

import (
	"context"
	"os/exec"
	"time"
)

// ExecRunner runs commands as subprocesses, like Laravel's Process::run:
// with the environment of memry, its output discarded, and stopped after
// Timeout (Laravel's default is 60 seconds).
type ExecRunner struct {
	Timeout time.Duration
}

// Run runs argv and reports whether it exited with status 0. A command
// that cannot start or times out has failed.
func (r ExecRunner) Run(argv []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()
	// Stdin, stdout and stderr are the null device.
	return exec.CommandContext(ctx, argv[0], argv[1:]...).Run() == nil
}
