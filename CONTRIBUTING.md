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
memry uninstall                               # undo setup (asks for confirmation; --force skips it)
memry delete-account                          # delete the account and all its memories, then undo setup
```

Two hidden commands are run by Claude Code, not by users: `memry mcp-headers` (the MCP
`headersHelper`) and `memry hook:session-start` (the SessionStart hook).

## Environment variables

| Variable            | Purpose                                                                                                                  |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `MEMRY_URL`         | Default server URL when `--url` is not given (default `https://api.memry.com.mx`).                                       |
| `MEMRY_CONFIG`      | Alternative config file path (default `~/.config/memry/config.json`). Passed on to the `headersHelper` and the SessionStart hook. |
| `MEMRY_EXECUTABLE`  | Shell command Claude Code runs memry with (default: the running memry executable). Set by the Homebrew wrapper.         |
| `CLAUDE_CONFIG_DIR` | Claude Code config directory where the hook is installed (default `~/.claude`).                                         |

The Homebrew formula installs the PHAR behind a wrapper that sets `MEMRY_EXECUTABLE` to the stable
`$(brew --prefix)/opt/memry/bin/memry` path, so the commands `memry setup` writes into Claude Code
keep working after `brew upgrade`. A custom `MEMRY_CONFIG` is prefixed to that command as well.

## How `memry setup` works

Without `--url`, `memry setup` uses `https://api.memry.com.mx` (or `MEMRY_URL`).

1. **Login.** It emails a one-time login code, exchanges it for an API token, and saves
   `{"url": "...", "token": "..."}` to `~/.config/memry/config.json` (permissions 0600). Other
   keys already in that file are kept.
2. **Previous token.** If the file already held a token from an earlier login, it revokes it with
   `DELETE <previous url>/api/auth/token`, so only the new token stays valid. A failed revoke (for
   example a token that is already revoked or an unreachable server) only prints a warning; it
   never fails setup.
3. **MCP server.** It registers the `memry` MCP server in Claude Code (user scope), replacing any
   existing entry and removing the legacy `db-memory` entry of earlier versions. The server uses a
   `headersHelper`: Claude Code runs `memry mcp-headers`, which prints
   `{"Authorization":"Bearer <token>"}` from `config.json`. The token lives only in `config.json`
   and is never stored in Claude Code's configuration. If the `claude` CLI is not found or the
   registration fails, setup keeps the login, prints the manual command (see
   [Troubleshooting](#troubleshooting)) and exits with code 1.
4. **SessionStart hook.** It adds memry's hook to Claude Code's user settings
   (`$CLAUDE_CONFIG_DIR/settings.json`, default `~/.claude/settings.json`), replacing any earlier
   memry hook and keeping every other setting and hook. The hook is installed even if MCP
   registration fails. `settings.json` is rewritten atomically (temporary file + rename) with its
   permissions and non-ASCII characters preserved. If it is not valid JSON or its `hooks` do not
   have the expected shape, it is left untouched and setup prints the hook to add by hand and exits
   with code 1.

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

`memry uninstall` asks "Remove memry from Claude Code and delete your login? (yes/no)" (default
no; `--force` skips it), then:

1. Revokes the stored token with `DELETE <url>/api/auth/token` (skipped when not logged in; a 401
   means it was already revoked and counts as done).
2. Removes the `memry` and legacy `db-memory` MCP servers with `claude mcp remove --scope user`
   (a server that is not registered is ignored).
3. Removes memry's SessionStart hook from Claude Code's `settings.json`, keeping every other hook
   and setting. A `SessionStart` list or `hooks` object left empty is dropped. A malformed file is
   left untouched.
4. Deletes `~/.config/memry/config.json` (or `MEMRY_CONFIG`).

Each step runs even if an earlier one fails and prints one line. It exits with code 1 if any step
failed (revoke failed with another error or an unreachable server, `claude` CLI not found,
malformed settings, config not deletable) or the confirmation was declined, and 0 otherwise,
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
4. **Local cleanup.** After a deletion it runs steps 2 to 4 of `memry uninstall` (MCP servers,
   SessionStart hook, config file); there is no token left to revoke. Each step runs even if an
   earlier one fails, and it exits with code 1 if any step failed. It ends with a hint to run
   `brew uninstall memry`.

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
