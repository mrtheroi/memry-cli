// Package commands holds the memry cobra commands.
package commands

import "github.com/spf13/cobra"

// NewRoot returns the memry root command. --version (or -V) prints
// "Memry <version>", like the PHP CLI.
func NewRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "memry",
		Short:   "CLI for memry: log in from the terminal and wire the memry memory MCP into Claude Code.",
		Version: version,
	}
	root.Flags().BoolP("version", "V", false, "Display this application version")
	root.SetVersionTemplate("Memry {{.Version}}\n")
	// Symfony's global options, which may come before the command
	// (`memry -n setup`). Declaring them lets cobra find the command; the
	// command still reads them from its own arguments.
	global := root.PersistentFlags()
	global.BoolP("no-interaction", "n", false, "Do not ask any interactive question")
	global.BoolP("quiet", "q", false, "Do not output any message")
	global.Bool("silent", false, "Do not output any message")
	global.CountP("verbose", "v", "Increase the verbosity of messages")
	global.Bool("ansi", false, "Force ANSI output")
	global.Bool("no-ansi", false, "Disable ANSI output")
	root.AddCommand(newSetup(version), newUninstall(version), newMcp(version), newSessionStartHook(version), newMcpHeaders(version))
	return root
}
