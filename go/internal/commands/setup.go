package commands

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/flags"
	"github.com/mrtheroi/memry-cli/internal/prompt"
	"github.com/mrtheroi/memry-cli/internal/setup"
)

// errFailed makes the CLI exit with code 1 once setup has said why.
var errFailed = errors.New("setup failed")

// newSetup returns `memry setup`. Its options are read by setup itself
// (flags.Scan), the way the PHP CLI reads them, so cobra does not parse
// them.
func newSetup(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "setup [options]",
		Short:                 "Log in to memry with an email one-time code or a token",
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			// Symfony's --help and --version, which cobra does not see
			// without parsing. Like Symfony they match by prefix (--help=x
			// still shows the help), and quiet or silent hides them.
			showsHelp := flags.HasParameterOption(args, "--help", "-h")
			showsVersion := flags.HasParameterOption(args, "--version", "-V")
			if showsHelp || showsVersion {
				if flags.IsQuiet(flags.Scan(args), os.LookupEnv) {
					return nil
				}
				if showsHelp {
					return cmd.Help()
				}
				_, err := fmt.Fprintf(out, "Memry %s\n", version)
				return err
			}
			code := setup.Run(setup.Env{
				Args:      args,
				LookupEnv: os.LookupEnv,
				Unsetenv:  os.Unsetenv,
				Out:       out,
				Prompter:  prompt.New(cmd.InOrStdin(), out),
				Agents:    agents.Stub{},
				HTTP:      client.New(version, 10*time.Second),
			})
			if code != 0 {
				return errFailed
			}
			return nil
		},
	}
	// The options setup reads, declared for the usage only.
	options := cmd.Flags()
	options.String("url", "", "The memry server URL")
	options.String("email", "", "The email to log in with")
	options.String("token", "", "Log in with a token created by the server admin, read from MEMRY_TOKEN or asked with a hidden prompt (or given as --token=<value>)")
	options.String("agents", "", "Comma-separated keys of the agents to wire memry into")
	options.BoolP("no-interaction", "n", false, "Do not ask any interactive question")
	options.BoolP("quiet", "q", false, "Do not output any message")
	return cmd
}
