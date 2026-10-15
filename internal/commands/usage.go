package commands

import (
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/usage"
)

// newUsage returns `memry usage`, which groups the commands that turn on
// and off sending Claude Code's usage metrics to the memry server.
func newUsage(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Opt in or out of sending your Claude Code token and cost metrics to your memry server",
	}
	enable := usageCommand("enable", "Send your Claude Code token and cost metrics to your memry server (off by default)", version, usage.Enable)
	enable.Flags().Bool("force", false, "Replace telemetry settings already set to other values")
	enable.Flags().Bool("insecure", false, "Allow a plain http:// server that is not this machine, sending the usage token in clear text")
	cmd.AddCommand(
		enable,
		usageCommand("disable", "Stop sending Claude Code usage metrics, removing only what memry set", version, usage.Disable),
		usageCommand("status", "Show whether Claude Code usage metrics are sent, and where", version, usage.Status),
	)
	return cmd
}

// usageCommand returns a `memry usage` subcommand run by run. Like the
// other commands, it reads its own options, so cobra does not parse them.
func usageCommand(name, short, version string, run func(usage.Env) int) *cobra.Command {
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
			args, lookupEnv, ok := withConfigFlag(cmd, args)
			if !ok {
				return errFailed
			}
			code := run(usage.Env{
				GOOS:      runtime.GOOS,
				Args:      args,
				LookupEnv: lookupEnv,
				Environ:   os.Environ,
				Out:       cmd.OutOrStdout(),
				HTTP:      client.New(version, 10*time.Second),
				// Claude Code's managed settings, which win over the
				// user's.
				ManagedDir: usage.ManagedSettingsDir(runtime.GOOS),
				CacheDir:   cacheDir(),
			})
			if code != 0 {
				return errFailed
			}
			return nil
		},
	}
}

// cacheDir is the user's cache directory, where the usage lock lives, or
// "" when there is none.
func cacheDir() string {
	dir, _ := os.UserCacheDir()
	return dir
}
