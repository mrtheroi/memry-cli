package agents

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// command prepares argv to run. The command is found through PATH and
// PATHEXT; a .cmd or .bat runs through cmd.exe with a command line built
// by CmdLine, because Go does not quote arguments for cmd.exe.
func command(ctx context.Context, argv []string) (*exec.Cmd, error) {
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, err
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext != ".cmd" && ext != ".bat" {
		return exec.CommandContext(ctx, path, argv[1:]...), nil
	}
	line, err := CmdLine(path, argv[1:])
	if err != nil {
		return nil, err
	}
	// The system directory, not %ComSpec%, so the environment cannot pick
	// the shell.
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, filepath.Join(system, "cmd.exe"))
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
	return cmd, nil
}
