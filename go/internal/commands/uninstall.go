package commands

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/prompt"
	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

// newUninstall returns `memry uninstall`.
func newUninstall(version string) *cobra.Command {
	cmd := removal("uninstall", "Remove memry from your agents and delete your login", version, uninstall.Uninstall)
	cmd.Flags().Bool("force", false, "Do not ask for confirmation")
	return cmd
}

// removal returns a command that removes memry, run by run. Like setup,
// it reads its own options the way the PHP CLI does, so cobra does not
// parse them.
func removal(name, short, version string, run func(uninstall.Env) int) *cobra.Command {
	return &cobra.Command{
		Use:                   name + " [options]",
		Short:                 short,
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if shown, err := helpOrVersion(cmd, args, version); shown {
				return err
			}
			out := cmd.OutOrStdout()
			code := run(uninstall.Env{
				Args:      args,
				LookupEnv: os.LookupEnv,
				Out:       out,
				Prompter:  prompt.New(cmd.InOrStdin(), out),
				Agents:    agents.New(agentsEnv()),
				HTTP:      client.New(version, 10*time.Second),
			})
			if code != 0 {
				return errFailed
			}
			return nil
		},
	}
}
