<p align="center">
  <img src="art/memry-logo.png" alt="memry" width="400">
</p>

# memry CLI

Command-line client for [memry](https://github.com/mrtheroi/memry-server), a private remote memory MCP server for AI agents.

`memry setup` logs in from the terminal with an email one-time code, registers memry in Claude Code as the `memry` MCP server, and installs a `SessionStart` hook that loads your project's memry context into every session.

Requires the [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI (`claude`) on your `PATH`.

## Installation

```bash
brew install mrtheroi/tap/memry
```

The Homebrew formula installs the memry PHAR behind a wrapper that sets `MEMRY_EXECUTABLE` to
the stable `$(brew --prefix)/opt/memry/bin/memry` path, so the commands `memry setup` writes into
Claude Code keep working after `brew upgrade`. Requires PHP 8.3+.

## Usage

```bash
memry setup                                   # asks for your email, then the 6-digit login code
memry setup --email you@example.com           # skip the email prompt
memry setup --url https://your-memry.example  # use another memry server
memry uninstall                               # undo setup (asks for confirmation; --force skips it)
```

Without `--url`, `memry setup` uses `https://api.memry.com.mx` (or `MEMRY_URL`).

`memry setup` emails you a one-time login code, exchanges it for an API token, and saves
`{"url": "...", "token": "..."}` to `~/.config/memry/config.json` (permissions 0600).
Other keys already in that file are kept.

If the file already held a token from an earlier login, `memry setup` then revokes it with
`DELETE <previous url>/api/auth/token`, so only the new token stays valid. A failed revoke
(for example a token that is already revoked or an unreachable server) only prints a warning;
it never fails setup.

It then registers the `memry` MCP server in Claude Code (user scope), replacing any existing entry and removing the legacy `db-memory` entry of earlier versions:

```bash
claude mcp add-json --scope user memry '{"type":"http","url":"<url>/mcp/memory","headersHelper":"<memry> mcp-headers"}'
```

The server uses a `headersHelper`: Claude Code runs `memry mcp-headers`, which prints
`{"Authorization":"Bearer <token>"}` from `config.json`. The token lives only in `config.json`
and is never stored in Claude Code's configuration.

If the `claude` CLI is not found or the registration fails, `memry setup` keeps your login,
prints the command above to run by hand, and exits with code 1.

### SessionStart hook

`memry setup` also adds this hook to Claude Code's user settings
(`$CLAUDE_CONFIG_DIR/settings.json`, default `~/.claude/settings.json`), replacing any earlier
memry hook and keeping every other setting and hook:

```json
{"hooks": {"SessionStart": [{"matcher": "startup|resume|clear|compact",
  "hooks": [{"type": "command", "command": "<memry> hook:session-start", "timeout": 10}]}]}}
```

`memry hook:session-start` reads the hook JSON on stdin, uses the git top-level of its `cwd`
(or the `cwd` itself outside git) as the repo root, resolves the project (see below), and prints
the memry usage protocol followed by `GET <url>/api/context?project=<project>`. The protocol asks
Claude to save memories under that project (or under another product's project when they are
about another product) and to pass the project and the repo (the root's directory name) to
`session-summary`. When you are not logged in or the request fails it
prints nothing and exits with code 0, so it never blocks a session. The hook is installed even
if MCP registration fails.

#### Project name

memry groups memories by product, not by folder, so several repos can share one project.
Declare it in a `.memry.json` file at the repo root:

```json
{"project": "memry"}
```

The value is trimmed. Without the file, or when it is unreadable, not valid JSON or has no
non-empty string `project`, the project is the root's directory name.

`settings.json` is rewritten atomically (temporary file + rename) with its permissions and
non-ASCII characters preserved. If it is not valid JSON or its `hooks` do not have the expected
shape, it is left untouched and setup prints the hook to add by hand and exits with code 1.

### Uninstall

`memry uninstall` undoes `memry setup` after asking
"Remove memry from Claude Code and delete your login? (yes/no)" (default no; `--force` skips it):

1. Revokes the stored token with `DELETE <url>/api/auth/token` (skipped when not logged in; a
   401 means it was already revoked and counts as done).
2. Removes the `memry` and legacy `db-memory` MCP servers with `claude mcp remove --scope user`
   (a server that is not registered is ignored).
3. Removes memry's SessionStart hook from Claude Code's `settings.json`, keeping every other hook
   and setting. A `SessionStart` list or `hooks` object left empty is dropped. A malformed file is
   left untouched.
4. Deletes `~/.config/memry/config.json` (or `MEMRY_CONFIG`).

Each step runs even if an earlier one fails and prints one line. It exits with code 1 if any step
failed (revoke failed with another error or an unreachable server, `claude` CLI not found, malformed settings, config not deletable) or the
confirmation was declined, and 0 otherwise, including when there was nothing to remove.
Then remove the CLI itself:

```bash
brew uninstall memry
```

| Variable       | Purpose                                                              |
| -------------- | -------------------------------------------------------------------- |
| `MEMRY_URL`    | Default server URL when `--url` is not given (default `https://api.memry.com.mx`) |
| `CLAUDE_CONFIG_DIR` | Claude Code config directory where the hook is installed (default `~/.claude`) |
| `MEMRY_CONFIG` | Alternative config file path (default `~/.config/memry/config.json`); passed on to the `headersHelper` and the SessionStart hook |
| `MEMRY_EXECUTABLE` | Shell command Claude Code runs memry with (default: the running memry executable); set by the Homebrew wrapper |

## Development

- PHP 8.3+ · [Laravel Zero](https://laravel-zero.com) · Pest
- Run the tests with `./vendor/bin/pest`
- Build the PHAR with `php memry app:build memry --build-version=<version>` (output: `builds/memry`)
