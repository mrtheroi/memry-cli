# Contributing to memry CLI

This guide is for developers and maintainers of the memry CLI. For installing and using memry,
see the [README](README.md).

The CLI is the command-line client for [memry](https://github.com/mrtheroi/memry-server), a
private remote memory MCP server for AI agents. It is written in Go and distributed as a single
static binary: through the `mrtheroi/homebrew-tap` Homebrew tap, `go install` and the GitHub
release archives.

## Development setup

1. Install Go (the version in [`go.mod`](go.mod)) and [Docker](https://www.docker.com) for the
   linters.
2. Run the checks before opening a pull request:

   ```bash
   go vet ./...
   go test -race ./...
   docker run --rm -v "$PWD":/app -w /app golangci/golangci-lint:v2.14.0 golangci-lint run ./...
   scripts/update-formula_test.sh
   ```

   Windows CI runs `go vet` and `go test ./...` without `-race`, which needs cgo.

   CI runs the same checks (see [`.github/workflows/go.yml`](.github/workflows/go.yml), which pins
   the golangci-lint version) and builds the release archives with
   `goreleaser release --snapshot --clean --skip=publish,sign` to validate
   [`.goreleaser.yaml`](.goreleaser.yaml). If you change a workflow, check it with
   [actionlint](https://github.com/rhysd/actionlint):
   `docker run --rm -v "$PWD":/repo -w /repo rhysd/actionlint:latest`.

3. Run the CLI from the source tree with `go run ./cmd/memry <command>`. It edits the config file
   and agent configs of the `HOME` it runs with, so try it with a throwaway one, for example
   `HOME="$(mktemp -d)" go run ./cmd/memry setup --url http://localhost:8000`.

`go test ./cmd/memry -run '^$' -bench McpStartup` measures `memry mcp` from process start to its
first reply.

Record every user-visible change under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md), following
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

### Tests first

Every change is test-driven: write one failing test for the next behavior, see it fail for the
right reason, write the least code that makes it pass, then refactor with the tests green. A bug
fix starts with a test that reproduces the bug. Tests never touch your real home, agent configs,
`claude` CLI or memry server: they point `HOME`, `CLAUDE_CONFIG_DIR`, `CODEX_HOME` and
`XDG_CONFIG_HOME` at `t.TempDir()` (set both `USERPROFILE` and `HOME`: Windows reads `USERPROFILE`
first, Unix `HOME`), fake the `claude` CLI and the agents, and serve the memry API
from `httptest`. Build expected paths with `filepath` and JSON-escaped strings, never with hard-coded
separators, so the same test passes on Windows. A behavior that cannot be tested is called out in the pull request: what is not
covered and why.

### Code layout

| Package               | Responsibility                                                              |
| --------------------- | --------------------------------------------------------------------------- |
| `cmd/memry`           | Entry point; the version is set with `-ldflags "-X main.version=<version>"` (`dev` otherwise) |
| `internal/commands`   | Cobra commands: the root command, `--version`, `setup`, `uninstall`, `delete-account` and the hidden ones |
| `internal/config`     | `config.json` path, load and save                                           |
| `internal/fsx`        | Atomic file writes                                                          |
| `internal/client`     | The HTTP client: no redirects, 2xx-only success, timeout, User-Agent        |
| `internal/flags`      | Symfony-like options: the `setup` resolver, verbosity, argument checks      |
| `internal/setup`      | `memry setup`: the email and token logins, then the agent selection and wiring |
| `internal/uninstall`  | `memry uninstall` and `memry delete-account`: remove memry from this machine |
| `internal/prompt`     | Terminal questions (hidden, yes/no) and the agent multiselect               |
| `internal/email`      | Email validation matching PHP's `FILTER_VALIDATE_EMAIL`                     |
| `internal/agents`     | The agent adapters (Claude Code, Codex, OpenCode, Antigravity, Windsurf)    |
| `internal/agentfiles` | Safe edits of agent files: rules blocks, JSON/TOML MCP configs, settings    |
| `internal/executable` | The memry command agents run (`MEMRY_EXECUTABLE`, `MEMRY_CONFIG`)           |
| `internal/mcp`        | `memry mcp`, the stdio↔HTTP MCP proxy, and `memry mcp-headers`              |
| `internal/hook`       | `memry hook:session-start`, Claude Code's SessionStart context hook         |
| `internal/phpjson`    | JSON validation and encoding matching PHP's `json_validate`/`json_encode`   |
| `internal/console`    | Output rendered like Symfony Console (the error block)                      |

The proxy reads a server reply of at most 16 MiB (`mcp.maxReplyBytes`) and answers a longer one
with a JSON-RPC `-32603` error; the hook reads a context of at most 1 MiB and prints nothing past
it; setup reads responses of at most 1 MiB.

### The PHP CLI behind the Go code

Up to 0.7.0 the CLI was written in PHP (Laravel Zero). 1.0.0 is a Go rewrite that keeps its config
file, commands, flags, messages and exit codes, so some packages reproduce PHP behavior on purpose
(`phpjson`, `email`, the Symfony-like options and prompts). The last commit with the PHP CLI is
[`f819d83`](https://github.com/mrtheroi/memry-cli/tree/f819d83d1fd8447ae1d6023ac7f2ce27becd06b0). Test comments such as
"Ported from tests/Unit/RulesFileTest.php" name a Pest test at that commit.

Some tests compare against golden files that PHP wrote, in `testdata/`. The goldens are committed,
so the tests need no PHP. Each one has the script that generated it next to it:

- `internal/email/testdata/php_filter_validate_email.php`,
  `internal/phpjson/testdata/php_json_encode.php` and `internal/phpjson/testdata/php_json_pretty.php`
  need only PHP.
- `internal/agentfiles/testdata/php_agentfiles.php` runs the PHP CLI's own classes: check out
  `f819d83`, run `composer install`, then run it from `go/internal/agentfiles/testdata`, where it
  lived then.

Regenerate a golden only to add cases.

## Commands

```bash
memry setup                                   # asks for your email, then the 6-digit login code
memry setup --email you@example.com           # skip the email prompt
memry setup --url https://your-memry.example  # use another memry server
memry setup --url <url> --token               # self-hosted: log in with a token from the server admin (MEMRY_TOKEN or hidden prompt)
memry setup --agents=claude-code,codex        # skip the agent prompt (comma-separated keys)
memry uninstall                               # undo setup (asks for confirmation; --force skips it)
memry delete-account                          # delete the account and all its memories, then undo setup
```

Hidden commands are run by AI agents, not by users: `memry mcp-headers` (the Claude Code MCP
`headersHelper`), `memry hook:session-start` (the Claude Code SessionStart hook) and `memry mcp`
(a local stdio MCP server for agents that cannot send the token themselves).

## Environment variables

| Variable            | Purpose                                                                                                                  |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `MEMRY_URL`         | Default server URL when `--url` is not given (default `https://api.memry.com.mx`).                                       |
| `MEMRY_CONFIG`      | Alternative config file path (default `~/.config/memry/config.json`, `%USERPROFILE%\.config\memry\config.json` on Windows). Passed on to the `headersHelper` (Unix), the Windows `memry mcp` server and the SessionStart hook. The global `--config <path>` option (also `--config=<path>`) on `setup`, `uninstall`, `delete-account`, `mcp`, `mcp-headers` and `hook:session-start` wins over it, per process. |
| `MEMRY_TOKEN`       | Token `memry setup --url <url> --token` uses when `--token` has no value, instead of the hidden prompt (also without interaction). Trimmed; ignored when blank or when `--token` is not given. `--token=<value>` wins over it. Removed from setup's environment once read, so subprocesses never inherit it. |
| `MEMRY_EXECUTABLE`  | Path of the memry executable agents run (default: the running memry executable). Set by the Homebrew wrapper. Agents that take a command and its arguments separately get it as the command, so it must be a plain path. On Windows it may only contain letters, digits and `: / \ . _ -` (written with forward slashes in the hook), and a config path with `` " $ ` % ! ``, a line break or a typographic quote is refused. |
| `CLAUDE_CONFIG_DIR` | Claude Code config directory where the hook is installed (default `~/.claude`).                                         |
| `CODEX_HOME`        | Codex home directory, when it is an absolute path (default `~/.codex`), as Codex resolves it.                           |
| `XDG_CONFIG_HOME`   | Base of the OpenCode config directory (default `~/.config`), as OpenCode resolves it.                                   |

The Homebrew formula installs the binary behind a wrapper that sets `MEMRY_EXECUTABLE` to the stable
`$(brew --prefix)/opt/memry/bin/memry` path, so the commands `memry setup` writes into Claude Code
keep working after `brew upgrade`. A custom `MEMRY_CONFIG` is prefixed to that command as well (on Windows, `--config "<path>"` follows it),
and passed to the other agents through the environment of their MCP server entry.

## How `memry setup` works

Without `--url`, `memry setup` uses `https://api.memry.com.mx` (or `MEMRY_URL`). A given `--url` must
be an `http://` or `https://` address with a host and no query or fragment; anything else prints
"Invalid server address given with --url. Use an http:// or https:// URL.". `--url`, `--email` and
`--agents` given without a value print "The --<option> option needs a value.". Both fail with code 1
before asking or sending anything. (`--token` without a value is different: it asks for the token.)

1. **Login.** It emails a one-time login code, exchanges it for an API token, and saves
   `{"url": "...", "token": "..."}` to `~/.config/memry/config.json` (permissions 0600). Other
   keys already in that file are kept.

   With `--token` (self-hosted servers, where the admin creates the token), there is no email
   login: setup takes the token from `--token=<value>` (which leaves it in the shell history and the
   process list), else from a non-blank `MEMRY_TOKEN` (meant for scripts, also without interaction),
   else asks for it with a hidden prompt; `MEMRY_TOKEN` is only read with `--token`, and is then removed
   from setup's environment so no subprocess setup runs inherits it. It trims it and checks it with `GET <url>/api/context?project=memry`
   (`Authorization: Bearer <token>`, `Accept: application/json`, redirects not followed) before
   saving it the same way. Only a 2xx answer accepts the token: a 401 prints "The token was rejected by <url>.", any other
   answer (including a redirect) prints the server's message, and
   an unreachable server prints "Could not reach the memry server at <url>."; each exits with code 1
   without saving anything. `--token` without `--url` fails before asking for the token or sending
   anything, so an admin token never reaches the public server. An empty token, `--token` without a value and without `MEMRY_TOKEN` when not interactive, or
   `--email` together with `--token` (either with or without a value), fails before sending anything. The token is never printed.

   A config file that exists but cannot be read fails setup without overwriting it (one that is
   not valid JSON is overwritten). When setup stops after an email login without saving the new
   token (the agent question is cancelled, or the config cannot be read or written), it revokes
   that token, which no one could use otherwise. A token given with `--token` is never revoked.
2. **Previous token.** If the file already held a token from an earlier login, it revokes it with
   `DELETE <previous url>/api/auth/token`, so only the new token stays valid. A failed revoke (for
   example a token that is already revoked or an unreachable server) only prints a warning; it
   never fails setup.
3. **Agents.** It asks which agents to wire memry into (see [Agents](#agents)) and saves the
   selection as `"agents": ["claude-code"]` in `config.json`. The default is the saved selection,
   or else the installed agents; without an interactive terminal (or with `--no-interaction`) the
   default is used without asking. `--agents=<key>,<key>` skips the prompt; an unknown key fails
   with the list of valid keys before logging in. Setup then runs each selected agent's install
   and each previously saved but deselected agent's uninstall, prints their messages and a summary
   line per agent, and exits with code 1 if any of them failed. An empty selection only prints a
   warning. Each agent is selected once, in display order. A login saved without `agents` (setups
   of 0.4.0 and earlier, which only wired Claude Code) counts as Claude Code wired, so deselecting
   it unwires it. A deselected agent memry could not be removed from is saved in
   `agents_to_remove`, and later setups (and `memry uninstall`) retry the removal until it
   succeeds; selecting it again installs it instead. For Claude Code, install is steps 4 and 5; for the other agents, see
   [Other agents](#other-agents).
4. **MCP server.** It registers the `memry` MCP server in Claude Code (user scope), replacing any
   existing entry and removing the legacy `db-memory` entry of earlier versions. The server uses a
   `headersHelper`: Claude Code runs `memry mcp-headers`, which prints
   `{"Authorization":"Bearer <token>"}` from `config.json`. The token lives only in `config.json`
   and is never stored in Claude Code's configuration. If the `claude` CLI is not found or the
   registration fails, setup keeps the login, prints the manual command (see
   [Troubleshooting](#troubleshooting)) and exits with code 1.
5. **SessionStart hook.** It adds memry's hook to Claude Code's user settings
   (`$CLAUDE_CONFIG_DIR/settings.json`, default `~/.claude/settings.json`), replacing any earlier
   memry hook and keeping every other setting and hook. The hook is installed even if MCP
   registration fails. `settings.json` is rewritten atomically (temporary file + rename) with its
   permissions and non-ASCII characters preserved. If it is not valid JSON or its `hooks` do not
   have the expected shape, it is left untouched and setup prints the hook to add by hand and exits
   with code 1.

### Agents

Each supported agent is an adapter in `internal/agents` implementing the `Agent` interface:

| Method               | Purpose                                                                     |
| -------------------- | --------------------------------------------------------------------------- |
| `Key()`              | Stable key used by `--agents` and the `agents` config key, e.g. `claude-code`. |
| `Name()`             | Name shown to the user, e.g. `Claude Code`.                                 |
| `IsInstalled()`      | Detection, used to label the prompt and pick the default selection.         |
| `Install(url)`       | Wires memry into the agent.                                                 |
| `Uninstall()`        | Removes memry from the agent.                                               |

`Install` and `Uninstall` return a `Result`: whether it succeeded and the lines to show (style
`info`, `warn`, `error` or `line`). Adapters never write to the console, so the commands decide how
to print them. Every subprocess goes through an injected `Runner` (60 second timeout), and every
path derives from the injected `Getenv`, so tests run against a temporary home. `agents.New` lists
the supported agents in display order: Claude Code, Codex, OpenCode, Antigravity, Windsurf.

### Other agents

Codex, OpenCode, Antigravity and Windsurf are `configFileAgent`s (`internal/agents/configfile.go`). They have no SessionStart
hook, so install does two things, and uninstall reverses exactly those:

1. **MCP server.** It sets the `memry` entry in the agent's config file to a stdio server running
   `<memry> mcp` (see [How `memry mcp` works](#how-memry-mcp-works)), with a custom `MEMRY_CONFIG`
   in the entry's environment. The token and URL are never written: `memry mcp` reads them from
   `config.json`. The file and its directories are created when missing; every other key, server,
   comment and non-ASCII character is kept; a re-install replaces only memry's entry; writes are
   atomic (temporary file + rename, permissions kept). A file memry cannot edit safely is left
   untouched, and install prints the entry to add by hand and fails. Uninstall removes only the
   `memry` entry (and a server list left empty); config files are never deleted.
2. **Instructions.** It writes the memry protocol (`internal/agents/protocol.go`) into the agent's
   global instructions file as a block between `<!-- memry:start -->` and `<!-- memry:end -->`,
   appended after a blank line, or replaced in place when it is already there. Nothing else in the
   file changes. The protocol tells the agent to call `get-context` at the start of every session
   with the project name (the `project` in `.memry.json`, else the repo or folder name), to use
   `search-memory` and `get-memory`, to save with `save-memory` (with a `topic_key` for evolving
   topics) and to call `session-summary` before ending. Uninstall removes the block and the blank
   line before it, and deletes the file when nothing else is left in it. If the markers are broken
   (one without the other, or more than one block), the file is left untouched and the step fails.

Install writes the instructions even when the MCP server cannot be registered, and uninstall runs
both steps even when one fails.

| Agent       | Config file (format)                                                              | Instructions file                               | Detected when                                        |
| ----------- | --------------------------------------------------------------------------------- | ----------------------------------------------- | ---------------------------------------------------- |
| Codex       | `$CODEX_HOME/config.toml`, default `~/.codex/config.toml` (TOML, `[mcp_servers.memry]` with `command`, `args`, `env`) | `$CODEX_HOME/AGENTS.md`                         | `codex` is on the `PATH`, or its home directory exists |
| OpenCode    | `~/.config/opencode/opencode.json`, or an existing `opencode.jsonc` when there is no `opencode.json` (JSON, `mcp.memry` with `"type": "local"`, `command` array, `enabled`, `environment`) | `~/.config/opencode/AGENTS.md`                  | `opencode` is on the `PATH`, or its config directory exists |
| Antigravity | `~/.gemini/config/mcp_config.json` (JSON, `mcpServers.memry` with `command`, `args`, `env`) | `~/.gemini/config/GEMINI.md`                    | `~/.gemini/antigravity` or `~/.gemini/config` exists |
| Windsurf    | `~/.codeium/windsurf/mcp_config.json` (JSON, `mcpServers.memry` with `command`, `args`, `env`) | `~/.codeium/windsurf/memories/global_rules.md` | `~/.codeium/windsurf` exists                          |

OpenCode's directory is `$XDG_CONFIG_HOME/opencode` when `XDG_CONFIG_HOME` is set. Antigravity
also reads `~/.gemini/GEMINI.md`, but so does Gemini CLI, which has no memry server; memry uses the
Antigravity-only `~/.gemini/config/GEMINI.md` instead. Windsurf limits its global rules to 6,000
characters, so the protocol stays short.

Notes on the formats:

- **JSON** files are decoded as objects, so `{}` stays `{}`, and rewritten pretty-printed with
  unescaped slashes and Unicode. JSON with comments (such as a `.jsonc` file that uses them) cannot
  be parsed and is left untouched.
- **TOML** (Codex) has no parser dependency. `agentfiles.TOMLConfig` scans the file line by line, following
  strings (including multi-line ones), arrays and inline tables so a `[` inside a value is never
  taken for a table header, and edits only the lines of `[mcp_servers.memry]` and its subtables;
  every other byte stays the same. It refuses to edit (leaving the file untouched) when the scan
  finds something it cannot follow (an unterminated string or array, a line that is not a header
  or `key = value`) or memry (or the whole `mcp_servers` table) defined through dotted keys or an
  inline table.

To add an agent:

1. Write a failing test, then a type in `internal/agents` implementing `Agent`. Resolve its config
   files from `Getenv("HOME")` (or an agent-specific environment variable) so tests can point them
   at a temporary directory.
2. Add it to the list in `agents.New`.
3. Document it in the README and the changelog.

Command tests replace the registry with fake agents (see `fakeAgent` in
`internal/setup/helpers_test.go`) and script the questions through the `Prompter` interface.

### SessionStart hook

`memry hook:session-start` reads the hook JSON on stdin, uses the git top-level of its `cwd` (or
the `cwd` itself outside git) as the repo root, resolves the project, and prints the memry usage
protocol followed by `GET <url>/api/context?project=<project>`. The protocol asks Claude to save
memories under that project (or under another product's project when they are about another
product) and to pass the project and the repo (the root's directory name) to `session-summary`.
When the user is not logged in or the request fails, it prints nothing and exits with code 0, so
it never blocks a session.

### Windows notes

On Windows, setup registers Claude Code's server as a stdio `memry mcp` command (no
`headersHelper`) and the hook runs `memry hook:session-start` by name, so `memry` must be on the
`PATH`. Known accepted risk: when Claude Code runs hooks with CMD (no Git Bash), CMD searches the
current directory before the `PATH`. The binary is not Authenticode-signed.

### Project name

memry groups memories by product, not by folder, so several repos can share one project. A repo
declares it in a `.memry.json` file at its root:

```json
{"project": "memry"}
```

The value is trimmed. Without the file, or when it is unreadable, not valid JSON or has no
non-empty string `project`, the project is the root's directory name.

## How `memry uninstall` works

`memry uninstall` asks "Remove memry from your agents and delete your login? (yes/no)" (default
no; `--force` skips it), then:

1. Runs the uninstall of every agent saved in `config.json` (`agents`, and `agents_to_remove`
   from a failed removal), or of Claude Code when there is neither key (setups of 0.4.0 and
   earlier only wired Claude Code). For Claude Code:
   - it removes the `memry` and legacy `db-memory` MCP servers with
     `claude mcp remove --scope user`. A server that is not registered (the output says "No MCP
     server found") is ignored; any other failure fails the step. Without the `claude` CLI the
     removal is skipped with a warning, since there is no Claude Code to run the server;
   - it removes memry's SessionStart hook from Claude Code's `settings.json`, keeping every other
     hook and setting. A `SessionStart` list or `hooks` object left empty is dropped. A malformed
     file is left untouched.

   For the other agents, it removes the `memry` entry and the instructions block (see
   [Other agents](#other-agents)); a file memry cannot edit is left untouched with a warning.
2. Revokes the stored token with `DELETE <url>/api/auth/token` (skipped when not logged in; a 401
   means it was already revoked and counts as done).
3. Deletes `~/.config/memry/config.json` (or `MEMRY_CONFIG`).

When memry cannot be removed from an agent, uninstall stops after step 1 and keeps what a retry
needs: it neither revokes the token nor deletes the config; it saves the agents still to clean up
in `agents_to_remove`, with an empty `agents` selection, prints "Kept the login and the agents
still to clean up in <path>." and "Fix the problems above, then run `memry uninstall` again.", and
exits with code 1. A config file that exists but cannot be read fails uninstall before anything is
removed.

Otherwise steps 2 and 3 run even if the revoke fails, each printing one line. It exits with code 1 if
any step failed (revoke failed with another error or an unreachable server, config not deletable)
or the confirmation was declined, and 0 otherwise, including when there was nothing to remove. It
ends with a hint to run `brew uninstall memry` (on Windows, `winget uninstall memry` for a WinGet
install, else the path of the `memry.exe` to delete).

## How `memry delete-account` works

1. **Login.** Without a `url` and `token` in the config file it prints "You are not logged in to
   memry." and exits with code 1.
2. **Confirmation.** It warns that this deletes the account and all its memories on the server and
   cannot be undone, then asks "Type your account email to confirm". The answer is trimmed and
   lowercased; an invalid address prints "Enter a valid email address." and asks again, and an
   empty answer aborts with "Aborted; nothing was deleted." (exit code 1). There is no `--force`:
   deleting an account always requires typing the email. The email is not stored locally, so the
   server checks that it matches the account.
3. **Deletion.** It sends `DELETE <url>/api/account` with the stored token and `{"email": "..."}`.
   On a 204 the server has deleted the account with its memories, prompts, tokens and login codes,
   and it prints "Deleted your memry account and all its memories.". A 422 (email does not match),
   401 (login no longer valid; run `memry setup`), other error or unreachable server prints an
   error and exits with code 1 without removing anything locally.
4. **Local cleanup.** After a deletion it runs steps 1 and 3 of `memry uninstall` (the saved
   agents, config file); there is no token left to revoke. When memry cannot be removed from an
   agent, it keeps only `agents_to_remove` and an empty `agents` selection (no login) in the config file and asks you to run
   `memry uninstall` again, exiting with code 1. Otherwise it exits with code 1 if deleting the
   config file failed. It ends with the same removal hint as `memry uninstall`.

## How `memry mcp` works

`memry mcp` is a stdio MCP server that proxies every message to the memry server, so an agent
(Codex, OpenCode, Antigravity, Windsurf...) can launch it as a local MCP server and the token never
appears in the agent's config files.

- **Framing.** Newline-delimited JSON-RPC: it reads one message per line from stdin until EOF,
  then exits with code 0. Blank lines are ignored. Each reply is written to stdout as one line of
  JSON followed by `\n` and flushed right away. Nothing else is ever written to stdout.
- **Forwarding.** Each line is sent as is with `POST <url>/mcp/memory`, `Authorization: Bearer
  <token>`, `Content-Type: application/json` and `Accept: application/json, text/event-stream`,
  with a 30 second timeout. The config file is read again for every message, so a new
  `memry setup` takes effect without restarting the agent. The server is stateless: there is no
  `Mcp-Session-Id`, and a notification is answered with an empty 202, which prints nothing.
  JSON-RPC errors the server answers with a 4xx or 5xx status are forwarded unchanged.
- **Stateless protocol (2026-07-28).** When a request carries
  `params._meta["io.modelcontextprotocol/protocolVersion"]`, it also sends the `MCP-Protocol-Version`,
  `Mcp-Method` and (for `tools/call`, `prompts/get` and `resources/read`) `Mcp-Name` headers the
  server requires to match the body. Requests of the initialize handshake protocols get none.
- **Errors.** They are JSON-RPC errors for the request's `id`; a notification never gets one. The
  command keeps reading after every error, so the agent shows the message instead of losing the
  server.

| Case                                                | Code     | Message                                                   |
| --------------------------------------------------- | -------- | --------------------------------------------------------- |
| No `url` or `token` in the config file              | `-32000` | Not logged in to memry. Run `memry setup`.                |
| The server answers 401                              | `-32000` | Your memry login is no longer valid. Run `memry setup`.   |
| Other error status without a JSON-RPC body          | `-32603` | The memry server returned HTTP `<status>`.                |
| The server cannot be reached or times out           | `-32603` | Could not reach the memry server.                         |
| The server answers with something that is not JSON  | `-32603` | The memry server returned an invalid response.            |
| A line on stdin is not JSON (`id` is `null`)        | `-32700` | Parse error                                               |

To try it by hand (the replies are the only output; the second line prints nothing):

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"manual","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | memry mcp
```

## Troubleshooting

If `memry setup` cannot register the MCP server, register it by hand (`<memry>` is the value of
`MEMRY_EXECUTABLE`, or the path of the memry executable):

```bash
claude mcp add-json --scope user memry '{"type":"http","url":"<url>/mcp/memory","headersHelper":"<memry> mcp-headers"}'
```

If it cannot install the hook, fix `settings.json` and add this group to its
`hooks.SessionStart` array:

```json
{"hooks": {"SessionStart": [{"matcher": "startup|resume|clear|compact",
  "hooks": [{"type": "command", "command": "<memry> hook:session-start", "timeout": 10}]}]}}
```

For the other agents, setup prints the exact entry to add when it cannot edit a config file. For
example, for Codex:

```toml
[mcp_servers.memry]
command = "<memry>"
args = ["mcp"]
```

When the memry markers in an instructions file are broken, remove the leftover
`<!-- memry:start -->` / `<!-- memry:end -->` lines (and anything between them) and run
`memry setup` again.

`memry setup --no-interaction` needs `--email` (or `--token=<value>`); with `--email` it cannot read the login code either, so it exits
with code 1 after the code is sent. Run `memry setup` in a terminal to log in.

## Release process

Releases are built by GoReleaser ([`.goreleaser.yaml`](.goreleaser.yaml)) in the
[Release workflow](.github/workflows/release.yml) when a `vX.Y.Z` tag is pushed. It needs no
secret beyond the workflow's own `GITHUB_TOKEN`: the checksums are signed keyless with cosign,
through the workflow's GitHub OIDC identity.

1. Open a release pull request that moves the `[Unreleased]` entries in
   [CHANGELOG.md](CHANGELOG.md) under a new `## [X.Y.Z] - YYYY-MM-DD` heading, and merge it
   (`chore: release X.Y.Z`). That section becomes the GitHub release notes; the workflow fails
   without it.
2. Tag the merge commit on `main` and push the tag:

   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

3. The workflow runs `go vet`, `go test -race` and golangci-lint, then GoReleaser publishes the
   release: `memry_X.Y.Z_<os>_<arch>.tar.gz` for darwin and linux, and `memry_X.Y.Z_windows_<arch>.zip`, on
   amd64 and arm64 (the binary, `LICENSE` and `README.md`), an SBOM per archive, `checksums.txt` and its cosign bundle
   `checksums.txt.sigstore.json`. To check the signature:

   ```bash
   cosign verify-blob --bundle checksums.txt.sigstore.json \
     --certificate-identity-regexp '^https://github.com/mrtheroi/memry-cli/' \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
   ```

4. Render the Homebrew formula from the release's checksums and open a pull request with it in
   [mrtheroi/homebrew-tap](https://github.com/mrtheroi/homebrew-tap):

   ```bash
   scripts/update-formula.sh X.Y.Z > ../homebrew-tap/Formula/memry.rb
   ```

   The template is [`packaging/homebrew/memry.rb.tmpl`](packaging/homebrew/memry.rb.tmpl): one
   archive per OS and architecture, the binary in `libexec`, and a `bin/memry` wrapper that sets
   `MEMRY_EXECUTABLE` to the stable opt path (see [Environment variables](#environment-variables)).
   GoReleaser does not push to the tap, which would need a token for another repository.

   GoReleaser also opens a pull request with the `Memry.Memry` manifest from the
   `mrtheroi/winget-pkgs` fork against `microsoft/winget-pkgs`, when the `WINGET_GITHUB_TOKEN`
   secret is set (release candidates are skipped).

5. Once the tap is merged, check the upgrade end to end:

   ```bash
   brew update && brew upgrade memry
   memry --version
   ```
