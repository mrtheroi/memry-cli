//go:build !windows

package agents

import (
	"context"
	"os/exec"
)

// command prepares argv to run as it is.
func command(ctx context.Context, argv []string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, argv[0], argv[1:]...), nil
}
