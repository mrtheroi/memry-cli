package commands

import (
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/console"
	"github.com/mrtheroi/memry-cli/internal/flags"
	"github.com/mrtheroi/memry-cli/internal/hook"
	"github.com/mrtheroi/memry-cli/internal/mcp"
)

// newMcp returns `memry mcp`, the stdio proxy agents launch as their
// memry MCP server.
func newMcp(version string) *cobra.Command {
	return hidden("mcp", "Proxy MCP messages between stdio and the memry server", version, func(cmd *cobra.Command, _ bool) int {
		return mcp.Proxy(proxyEnv(cmd, version))
	})
}

// proxyEnv is the proxy's environment. Like the PHP CLI, it gives the
// server 30 seconds per message.
func proxyEnv(cmd *cobra.Command, version string) mcp.Env {
	return mcp.Env{
		LookupEnv: os.LookupEnv,
		In:        cmd.InOrStdin(),
		Out:       cmd.OutOrStdout(),
		HTTP:      client.New(version, 30*time.Second),
	}
}

// newSessionStartHook returns `memry hook:session-start`, Claude Code's
// SessionStart hook.
func newSessionStartHook(version string) *cobra.Command {
	return hidden("hook:session-start", "Print the memry context of the current project (Claude Code SessionStart hook)", version, func(cmd *cobra.Command, quiet bool) int {
		return hook.SessionStart(sessionStartEnv(cmd, version, quiet))
	})
}

// sessionStartEnv is the hook's environment. Like the PHP CLI, it gives
// the server 3 seconds, so a slow server never holds up a session.
func sessionStartEnv(cmd *cobra.Command, version string, quiet bool) hook.Env {
	return hook.Env{
		LookupEnv:   os.LookupEnv,
		In:          cmd.InOrStdin(),
		Out:         output(cmd.OutOrStdout(), quiet),
		HTTP:        client.New(version, 3*time.Second),
		Getwd:       os.Getwd,
		GitTopLevel: hook.GitTopLevel,
	}
}

// newMcpHeaders returns `memry mcp-headers`, Claude Code's headersHelper.
func newMcpHeaders(version string) *cobra.Command {
	return hidden("mcp-headers", "Print the memry MCP authorization header as JSON (Claude Code headersHelper)", version, func(cmd *cobra.Command, quiet bool) int {
		return mcp.Headers(mcp.HeadersEnv{
			LookupEnv: os.LookupEnv,
			Out:       output(cmd.OutOrStdout(), quiet),
			Err:       output(cmd.ErrOrStderr(), quiet),
		})
	})
}

// hidden returns a command left out of the command list that, like the
// PHP CLI's, takes no arguments and only the global options. Cobra does
// not parse them: they are read the way Symfony does, and run learns
// whether they make the output quiet.
func hidden(name, short, version string, run func(cmd *cobra.Command, quiet bool) int) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              short,
		Hidden:             true,
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if shown, err := helpOrVersion(cmd, args, version); shown {
				return err
			}
			if message, ok := flags.UnexpectedArgument(name, args); ok {
				if !flags.IsSilent(flags.Scan(args), os.LookupEnv) {
					console.ErrorBlock(cmd.OutOrStdout(), message)
				}
				return errFailed
			}
			if run(cmd, flags.IsQuiet(flags.Scan(args), os.LookupEnv)) != 0 {
				return errFailed
			}
			return nil
		},
	}
}

// output is out, or nothing when quiet.
func output(out io.Writer, quiet bool) io.Writer {
	if quiet {
		return io.Discard
	}
	return out
}
