package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/executable"
	"github.com/mrtheroi/memry-cli/internal/flags"
	"github.com/mrtheroi/memry-cli/internal/prompt"
	"github.com/mrtheroi/memry-cli/internal/setup"
)

// errFailed makes the CLI exit with code 1 once the command has said why.
var errFailed = errors.New("command failed")

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
			if shown, err := helpOrVersion(cmd, args, version); shown {
				return err
			}
			out := cmd.OutOrStdout()
			code := setup.Run(setup.Env{
				Args:      args,
				LookupEnv: os.LookupEnv,
				Unsetenv:  os.Unsetenv,
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

// agentsEnv is what the agents run with: this process's environment and
// PATH, and its own binary as the memry executable agents run.
func agentsEnv() agents.Env {
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	return agents.Env{
		Getenv:     os.Getenv,
		LookPath:   exec.LookPath,
		Runner:     agents.ExecRunner{Timeout: 60 * time.Second},
		Executable: executable.Executable{Getenv: os.Getenv, Self: self},
	}
}

// helpOrVersion shows the help or the version for Symfony's --help and
// --version options, which cobra does not see without parsing, reporting
// whether it did. Like Symfony they match by prefix (--help=x still shows
// the help), and quiet or silent hides them.
func helpOrVersion(cmd *cobra.Command, args []string, version string) (bool, error) {
	showsHelp := flags.HasParameterOption(args, "--help", "-h")
	if !showsHelp && !flags.HasParameterOption(args, "--version", "-V") {
		return false, nil
	}
	if flags.IsQuiet(flags.Scan(args), os.LookupEnv) {
		return true, nil
	}
	if showsHelp {
		return true, cmd.Help()
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Memry %s\n", version)
	return true, err
}
