package agents

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

// maxOutput is how much of a command's output a RunResult keeps.
const maxOutput = 64 * 1024

// ExecRunner runs commands as subprocesses, like Laravel's Process::run:
// with the environment of memry, and stopped after Timeout (Laravel's
// default is 60 seconds).
type ExecRunner struct {
	Timeout time.Duration
}

// Run runs argv and reports how it went, keeping the first maxOutput
// bytes of its stdout and stderr together.
func (r ExecRunner) Run(argv []string) RunResult {
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()
	output := &cappedBuffer{}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout, cmd.Stderr = output, output
	// A child the command leaves holding the output open must not keep
	// Run waiting after the command is stopped.
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return RunResult{ExitCode: -1}
	}
	err := cmd.Wait()
	result := RunResult{Started: true, ExitCode: cmd.ProcessState.ExitCode(), Output: string(output.data)}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.TimedOut, result.ExitCode = true, -1
	} else if err != nil && result.ExitCode == 0 {
		// The output could not be read in time: not a clean exit.
		result.ExitCode = -1
	}
	return result
}

// cappedBuffer keeps the first maxOutput bytes written to it and drops
// the rest, without failing the writes.
type cappedBuffer struct {
	data []byte
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := maxOutput - len(b.data); room > 0 {
		b.data = append(b.data, p[:min(room, len(p))]...)
	}
	return len(p), nil
}
