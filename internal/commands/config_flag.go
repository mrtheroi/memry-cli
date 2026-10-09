package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/mrtheroi/memry-cli/internal/console"
	"github.com/mrtheroi/memry-cli/internal/flags"
)

// withConfigFlag takes the global --config option out of args. It returns
// the remaining arguments and the environment lookup the command must use:
// the process environment, where --config answers MEMRY_CONFIG with its
// absolute path (so --config wins over MEMRY_CONFIG, which wins over the
// default) without touching the process environment. A bad --config is
// reported like any other error (nothing when silent) and ok is false.
func withConfigFlag(cmd *cobra.Command, args []string) (rest []string, lookupEnv func(string) (string, bool), ok bool) {
	path, given, rest, err := flags.ExtractConfig(args)
	if err != nil {
		if !flags.IsSilent(flags.Scan(args), os.LookupEnv) {
			console.ErrorBlock(cmd.OutOrStdout(), err.Error())
		}
		return nil, nil, false
	}
	if !given {
		return rest, os.LookupEnv, true
	}
	return rest, func(key string) (string, bool) {
		if key == "MEMRY_CONFIG" {
			return path, true
		}
		return os.LookupEnv(key)
	}, true
}
