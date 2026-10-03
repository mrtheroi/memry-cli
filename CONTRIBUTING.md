# Contributing to memry CLI

This guide is for developers and maintainers of the memry CLI. For installing and using memry,
see the [README](README.md).

The CLI is the command-line client for [memry](https://github.com/mrtheroi/memry-server), a
private remote memory MCP server for AI agents. It is built with PHP 8.3+,
[Laravel Zero](https://laravel-zero.com) and Pest, and is distributed as a single-file PHAR
through the `mrtheroi/homebrew-tap` Homebrew tap.

## Development setup

1. Install PHP 8.3 or later (for example with [Laravel Herd](https://herd.laravel.com)) and
   [Composer](https://getcomposer.org).
2. Install the dependencies:

   ```bash
   composer install
   ```

3. Run the tests and the code style fixer before opening a pull request:

   ```bash
   ./vendor/bin/pest
   ./vendor/bin/pint
   ```

4. Run the CLI from the source tree with `php memry <command>`, for example `php memry setup`.

Record every user-visible change under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md), following
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Commands

```bash
memry setup                                   # asks for your email, then the 6-digit login code
memry setup --email you@example.com           # skip the email prompt
memry setup --url https://your-memry.example  # use another memry server
memry setup --url <url> --token               # self-hosted: log in with a token from the server admin (hidden prompt)
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
| `MEMRY_CONFIG`      | Alternative config file path (default `~/.config/memry/config.json`). Passed on to the `headersHelper` and the SessionStart hook. |
| `MEMRY_EXECUTABLE`  | Path of the memry executable agents run (default: the running memry executable). Set by the Homebrew wrapper. Agents that take a command and its arguments separately get it as the command, so it must be a plain path. |
| `CLAUDE_CONFIG_DIR` | Claude Code config directory where the hook is installed (default `~/.claude`).                                         |
| `CODEX_HOME`        | Codex home directory, when it is an absolute path (default `~/.codex`), as Codex resolves it.                           |
| `XDG_CONFIG_HOME`   | Base of the OpenCode config directory (default `~/.config`), as OpenCode resolves it.                                   |

The Homebrew formula installs the PHAR behind a wrapper that sets `MEMRY_EXECUTABLE` to the stable
`$(brew --prefix)/opt/memry/bin/memry` path, so the commands `memry setup` writes into Claude Code
keep working after `brew upgrade`. A custom `MEMRY_CONFIG` is prefixed to that command as well,
and passed to the other agents through the environment of their MCP server entry.

## How `memry setup` works

Without `--url`, `memry setup` uses `https://api.memry.com.mx` (or `MEMRY_URL`).

1. **Login.** It emails a one-time login code, exchanges it for an API token, and saves
   `{"url": "...", "token": "..."}` to `~/.config/memry/config.json` (permissions 0600). Other
   keys already in that file are kept.

   With `--token` (self-hosted servers, where the admin creates the token), there is no email
   login: setup asks for the token with a hidden prompt (or takes it from `--token=<value>`, which
   leaves it in the shell history and is meant for scripts only), trims it and checks it with `GET <url>/api/context?project=memry`
   (`Authorization: Bearer <token>`, `Accept: application/json`) before saving it the same way.
   Only a 2xx answer accepts the token: a 401 prints "The token was rejected by <url>.", any other
   answer (including a redirect) prints the server's message, and
   an unreachable server prints "Could not reach the memry server at <url>."; each exits with code 1
   without saving anything. `--token` without `--url` fails before asking for the token or sending
   anything, so an admin token never reaches the public server. An empty token, `--token` without a value when not interactive, or
   `--email` together with `--token` (either with or without a value), fails before sending anything. The token is never printed.
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
   warning. For Claude Code, install is steps 4 and 5; for the other agents, see
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

Each supported agent is an adapter in `app/Agents` implementing the `Agent` contract:

| Method          | Purpose                                                                          |
| --------------- | -------------------------------------------------------------------------------- |
| `key()`         | Stable key used by `--agents` and the `agents` config key, e.g. `claude-code`.  |
| `name()`        | Name shown to the user, e.g. `Claude Code`.                                      |
| `isInstalled()` | Detection, used to label the prompt and pick the default selection.              |
| `install($url)` | Wires memry into the agent.                                                      |
| `uninstall()`   | Removes memry from the agent.                                                    |

`install()` and `uninstall()` return an `AgentResult`: whether it succeeded and the lines to show
(`[style, text]`, style `info`, `warn`, `error` or `line`). Adapters never write to the console, so
the commands decide how to print them. `AgentRegistry` lists the supported agents in display
order: Claude Code, Codex, OpenCode, Antigravity, Windsurf.

### Other agents

Codex, OpenCode, Antigravity and Windsurf extend `ConfigFileAgent`. They have no SessionStart
hook, so install does two things, and uninstall reverses exactly those:

1. **MCP server.** It sets the `memry` entry in the agent's config file to a stdio server running
   `<memry> mcp` (see [How `memry mcp` works](#how-memry-mcp-works)), with a custom `MEMRY_CONFIG`
   in the entry's environment. The token and URL are never written: `memry mcp` reads them from
   `config.json`. The file and its directories are created when missing; every other key, server,
   comment and non-ASCII character is kept; a re-install replaces only memry's entry; writes are
   atomic (temporary file + rename, permissions kept). A file memry cannot edit safely is left
   untouched, and install prints the entry to add by hand and fails. Uninstall removes only the
   `memry` entry (and a server list left empty); config files are never deleted.
2. **Instructions.** It writes the memry protocol (`app/Agents/Protocol.php`) into the agent's
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
- **TOML** (Codex) has no parser dependency. `TomlMcpConfig` scans the file line by line, following
  strings (including multi-line ones), arrays and inline tables so a `[` inside a value is never
  taken for a table header, and edits only the lines of `[mcp_servers.memry]` and its subtables;
  every other byte stays the same. It refuses to edit (leaving the file untouched) when the scan
  finds something it cannot follow (an unterminated string or array, a line that is not a header
  or `key = value`) or memry (or the whole `mcp_servers` table) defined through dotted keys or an
  inline table.

To add an agent:

1. Write a failing test, then an `app/Agents/<Name>Agent.php` class implementing `Agent`. Resolve
   its config files from `HOME` (or an agent-specific environment variable) so tests can point
   them at a temporary directory.
2. Add it to the default list in `AgentRegistry`.
3. Document it in the README and the changelog.

Command tests replace the registry with `fakeAgents(new FakeAgent(...), ...)` (see
`tests/Fakes/FakeAgent.php`). `tests/TestCase.php` sets the environment to `testing`, so Laravel
Prompts questions like the agent multiselect fall back to console questions that tests answer
with `expectsChoice()`. It also points `HOME` at an empty temporary directory and unsets
`CODEX_HOME` and `XDG_CONFIG_HOME` for every feature test, so no test can reach your real agent
configs.

### SessionStart hook

`memry hook:session-start` reads the hook JSON on stdin, uses the git top-level of its `cwd` (or
the `cwd` itself outside git) as the repo root, resolves the project, and prints the memry usage
protocol followed by `GET <url>/api/context?project=<project>`. The protocol asks Claude to save
memories under that project (or under another product's project when they are about another
product) and to pass the project and the repo (the root's directory name) to `session-summary`.
When the user is not logged in or the request fails, it prints nothing and exits with code 0, so
it never blocks a session.

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

1. Revokes the stored token with `DELETE <url>/api/auth/token` (skipped when not logged in; a 401
   means it was already revoked and counts as done).
2. Runs the uninstall of every agent saved in `config.json`, or of Claude Code when there is no
   `agents` key (setups of 0.4.0 and earlier only wired Claude Code). For Claude Code:
   - it removes the `memry` and legacy `db-memory` MCP servers with
     `claude mcp remove --scope user` (a server that is not registered is ignored);
   - it removes memry's SessionStart hook from Claude Code's `settings.json`, keeping every other
     hook and setting. A `SessionStart` list or `hooks` object left empty is dropped. A malformed
     file is left untouched.

   For the other agents, it removes the `memry` entry and the instructions block (see
   [Other agents](#other-agents)); a file memry cannot edit is left untouched with a warning.
3. Deletes `~/.config/memry/config.json` (or `MEMRY_CONFIG`).

Each step runs even if an earlier one fails and prints one line. It exits with code 1 if any step
failed (revoke failed with another error or an unreachable server, `claude` CLI not found,
malformed settings or agent files, config not deletable) or the confirmation was declined, and 0 otherwise,
including when there was nothing to remove. It ends with a hint to run `brew uninstall memry`.

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
4. **Local cleanup.** After a deletion it runs steps 2 and 3 of `memry uninstall` (the saved
   agents, config file); there is no token left to revoke. Each step runs even if an
   earlier one fails, and it exits with code 1 if any step failed. It ends with a hint to run
   `brew uninstall memry`.

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
  | php memry mcp
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

## Building the PHAR

`app:build` packs `vendor/` as it is (see `box.json`), so install without dev dependencies first,
or the PHAR ships Pest, Pint and the rest of the dev tooling:

```bash
composer install --no-dev -o
php memry app:build memry --build-version=X.Y.Z   # output: builds/memry (ignored by git)
composer install                                  # restore the dev dependencies
```

## Release process

1. Open a release pull request that moves the `[Unreleased]` entries in
   [CHANGELOG.md](CHANGELOG.md) under a new `## [X.Y.Z] - YYYY-MM-DD` heading, and merge it
   (`chore: release X.Y.Z`).
2. On the updated `main`, build the PHAR as described in [Building the PHAR](#building-the-phar).
3. Tag the release and publish the PHAR as a GitHub release asset:

   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   gh release create vX.Y.Z 'builds/memry#memry.phar'
   ```

4. In [mrtheroi/homebrew-tap](https://github.com/mrtheroi/homebrew-tap), update `url` and `sha256`
   in `Formula/memry.rb` to the new asset (`shasum -a 256 builds/memry`).
5. Check the upgrade end to end:

   ```bash
   brew update && brew upgrade memry
   memry --version
   ```
