// Package executable is the memry command agents run: Claude Code for the
// MCP headers helper and the SessionStart hook, the other agents for
// `memry mcp`.
package executable

import "strings"

// Executable is the memry executable agents run.
type Executable struct {
	// GOOS is the operating system; the zero value means Unix.
	GOOS string
	// Getenv reads MEMRY_EXECUTABLE and MEMRY_CONFIG.
	Getenv func(string) string
	// Self is the running memry binary (os.Executable), run when
	// MEMRY_EXECUTABLE is not set, like the PHP CLI runs its own PHAR.
	Self string
}

// Command is the shell command that runs subcommand: MEMRY_EXECUTABLE as
// it is (it may hold several words), else the running binary quoted, and
// a custom MEMRY_CONFIG before it. On Windows it is the hook command: the
// bare `memry` (or MEMRY_EXECUTABLE) and a custom MEMRY_CONFIG as a
// --config flag, both with forward slashes, which Git Bash, PowerShell and
// CMD all accept and Git Bash does not eat as escapes.
func (e Executable) Command(subcommand string) string {
	executable := e.Getenv("MEMRY_EXECUTABLE")
	if e.GOOS == "windows" {
		executable = strings.ReplaceAll(executable, `\`, "/")
		if executable == "" {
			executable = "memry"
		}
		if config := e.Getenv("MEMRY_CONFIG"); config != "" {
			executable += ` --config "` + strings.ReplaceAll(config, `\`, "/") + `"`
		}
		return executable + " " + subcommand
	}
	if executable == "" {
		executable = EscapeShellArg(e.Self)
	}
	// Claude Code runs our commands without our environment, so a custom
	// config path must travel with them.
	if config := e.Getenv("MEMRY_CONFIG"); config != "" {
		executable = "MEMRY_CONFIG=" + EscapeShellArg(config) + " " + executable
	}
	return executable + " " + subcommand
}

// EscapeShellArg quotes s for a POSIX shell, like PHP's escapeshellarg.
func EscapeShellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Arguments is the same command as a list of arguments, for agents that
// take the executable and its arguments separately instead of a shell
// command.
func (e Executable) Arguments(subcommand string) []string {
	executable := e.Getenv("MEMRY_EXECUTABLE")
	if executable == "" {
		executable = e.Self
	}
	return []string{executable, subcommand}
}

// Environment is the environment to run those arguments with: a custom
// MEMRY_CONFIG must travel with them, as agents do not pass on ours.
func (e Executable) Environment() map[string]string {
	if config := e.Getenv("MEMRY_CONFIG"); config != "" {
		return map[string]string{"MEMRY_CONFIG": config}
	}
	return nil
}

// SafeInHook reports whether a config path can go double-quoted in the
// Windows hook command: Git Bash expands $ and backticks inside double
// quotes, CMD expands %, and " ! CR and LF break the quoting. PowerShell
// also closes a double-quoted string on the typographic quotes.
func SafeInHook(path string) bool {
	return !strings.ContainsAny(path, "\"$`%!\r\n“”„‘’")
}
