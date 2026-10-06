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
	return root
}
