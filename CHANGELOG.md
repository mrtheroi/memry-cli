# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-09-30

### Added

- `memry mcp` is a hidden stdio MCP server for agents that launch local MCP servers. It reads newline-delimited JSON-RPC messages from stdin until EOF and forwards each one to `<url>/mcp/memory` with the stored token (`Authorization: Bearer`, 30 second timeout), printing each reply as one line of JSON on stdout and nothing for notifications, so the token never appears in the agent's config. Requests of the stateless 2026-07-28 protocol also get the `MCP-Protocol-Version`, `Mcp-Method` and `Mcp-Name` headers. Errors are JSON-RPC errors and never stop it: not logged in or a 401 (`-32000`, "Run `memry setup`."), another HTTP error, unreachable server or invalid reply (`-32603`), and a stdin line that is not JSON (`-32700`, `id` null). The config file is read for every message, so a new login applies without restarting the agent.
- `memry setup` asks "Which agents do you use?" after logging in (a multiselect of every supported agent; agents not found on this machine are labelled "(not installed)" but can still be selected). The default is the selection saved by the previous setup, or else the installed agents; it is also used without asking when the session is not interactive. `--agents=claude-code,...` skips the prompt; an unknown key prints "Unknown agent "<key>". Valid agents: ..." and exits with code 1 before logging in. The selection is saved as `agents` in `config.json`. Setup wires memry into every selected agent and removes it from agents saved earlier but no longer selected, continuing when one fails, then prints a summary line per agent; it exits with code 1 if any agent failed. Selecting no agent keeps the login, prints a warning and exits with code 0.
- `memry setup` supports four more agents, in this order after Claude Code: Codex (`codex`), OpenCode (`opencode`), Antigravity (`antigravity`) and Windsurf (`windsurf`). For each one it registers a `memry` MCP server that runs `memry mcp` over stdio (with a custom `MEMRY_CONFIG` in the server's environment; no token or URL in the agent's files) and adds the memry instructions, which tell the agent to load the project context with `get-context` at the start of every session and to save and summarize its work, to the agent's global instructions between `<!-- memry:start -->` and `<!-- memry:end -->` markers. Codex: `$CODEX_HOME/config.toml` (default `~/.codex`) and `AGENTS.md`. OpenCode: `~/.config/opencode/opencode.json` (or an existing `opencode.jsonc`; `$XDG_CONFIG_HOME` is honored) and `AGENTS.md`. Antigravity: `~/.gemini/config/mcp_config.json` and `~/.gemini/config/GEMINI.md`. Windsurf: `~/.codeium/windsurf/mcp_config.json` and `memories/global_rules.md`. Files and directories are created when missing, every other setting, server and instruction is kept, a re-install replaces only memry's entries, and writes are atomic with permissions kept. A config file memry cannot edit safely (invalid JSON, JSON with comments, TOML it cannot follow) or instructions with broken markers are left untouched, and setup prints what to add by hand and fails for that agent. `memry uninstall` removes only memry's entry and instructions block (deleting an instructions file left empty). An agent counts as installed when its CLI is on the `PATH` (Codex, OpenCode) or its config directory exists.

### Changed

- `memry uninstall` and `memry delete-account` remove memry only from the agents saved in `config.json`, or from Claude Code when the config has no `agents` (setups of 0.4.0 and earlier).
- `memry uninstall` asks "Remove memry from your agents and delete your login?" instead of naming Claude Code, and its description says the same.

### Fixed

- `memry setup --no-interaction` without `--email` no longer loops forever: it prints "Pass --email when running without interaction." and exits with code 1 before sending anything. When no login code can be read, it prints "No login code given. Run `memry setup` interactively to enter the code from the email." and exits with code 1 instead of asking again forever.

## [0.4.0] - 2026-09-29

### Added

- `memry delete-account` permanently deletes the memry account and all its memories. It requires being logged in ("You are not logged in to memry." otherwise), warns that the deletion cannot be undone and asks the user to type their account email to confirm (asking again for an invalid address; an empty answer aborts with exit code 1; there is no `--force`). It sends `DELETE <url>/api/account` with the stored token and the normalized email. On success it prints "Deleted your memry account and all its memories." and removes memry from this machine like `memry uninstall` (MCP servers, SessionStart hook, config file), without revoking the already deleted token. A 422 ("The email does not match your memry account."), 401 ("Your memry login is no longer valid. Run `memry setup` and try again."), other error or unreachable server ("Could not delete your memry account. Try again later.") exits with code 1 and removes nothing locally.

## [0.3.0] - 2026-09-29

### Added

- `memry uninstall` undoes `memry setup`: it revokes the stored token (`DELETE <url>/api/auth/token`; a 401 prints "The memry token was already revoked." and counts as success), removes the `memry` and legacy `db-memory` MCP servers from Claude Code, removes only memry's SessionStart hook from Claude Code's `settings.json` (dropping a `SessionStart` list or `hooks` object left empty, leaving a malformed file untouched), and deletes the config file. It asks "Remove memry from Claude Code and delete your login? (yes/no)" first (default no; `--force` skips it). Every step runs even when an earlier one fails; it exits with code 1 when a step failed or the confirmation was declined, and ends with the hint "Run `brew uninstall memry` to remove the CLI.".

### Changed

- The default server URL is now `https://api.memry.com.mx` instead of `https://db-mcp-production-n8vlvz.laravel.cloud`. Existing logins keep the URL stored in `config.json`.

- `memry setup` registers memry in Claude Code as the user-scope `memry` MCP server instead of `db-memory`. It removes the legacy `db-memory` entry and any existing `memry` entry (ignoring failures of either) before adding `memry` and verifying it with `claude mcp get memry`. The tool names in Claude Code change from `mcp__db-memory__*` to `mcp__memry__*`.
- The SessionStart protocol text references the `memry` MCP tools.

## [0.2.1] - 2026-09-29

### Fixed

- `memry setup` validates the email before sending any request, instead of crashing with a Guzzle `json_encode error: Malformed UTF-8 characters` when it contained bytes that are not valid UTF-8. An invalid `--email` prints "Invalid email address given with --email." and exits with code 1 without contacting the server; an invalid email typed at the prompt prints "Enter a valid email address." and asks again.
- `memry setup` asks for the login code again ("Enter the 6-digit code from the email.") until it has 6 digits, so a malformed code no longer crashes the same way.

## [0.2.0] - 2026-09-29

### Added

- `memry setup` revokes the token of the previous login (`DELETE /api/auth/token` on the server stored with it) after the new credentials are saved, printing "Revoked the previous memry token.". It is skipped when there was no previous token or the server returned the same one, and a failed revoke (401, server error, unreachable server) only prints a warning and never changes the exit code.

## [0.1.0] - 2026-09-28

### Added

- `memry setup` command: log in with an email one-time code and save the server URL and token to `~/.config/memry/config.json` (or `$MEMRY_CONFIG`), readable by the owner only.
- `--url` and `--email` options for `memry setup`; the default server URL can be overridden with `MEMRY_URL`.
- `memry setup` exits with code 1 and writes nothing when a request fails, including an unreachable server, rate limiting, unexpected server errors and a response without a token.
- `memry setup` registers memry in Claude Code as the user-scope `db-memory` MCP server (`claude mcp add-json`, replacing any existing entry, then verified with `claude mcp get`). The server uses a `headersHelper` that runs `memry mcp-headers`, so the token is stored only in the config file and never passed to Claude Code.
- `memry setup` exits with code 1 and prints the manual `claude mcp add-json` command when the Claude Code CLI is not on `PATH` or registration fails; the login is kept.
- Hidden `memry mcp-headers` command: prints `{"Authorization":"Bearer <token>"}` from the config file for Claude Code's `headersHelper`, or exits with code 1 and nothing on stdout when not logged in.
- A custom `MEMRY_CONFIG` used during `memry setup` is passed on to the registered `headersHelper`, so `memry mcp-headers` reads the same config file.
- Hidden `memry hook:session-start` command: a Claude Code `SessionStart` hook that reads the hook JSON on stdin, resolves the project from the git top-level of its `cwd` (or the directory name), and prints the memry usage protocol followed by `GET /api/context` for that project. It prints nothing and always exits with code 0 when not logged in or the request fails, so it never blocks a session.
- `memry setup` installs that hook in Claude Code's user settings (`$CLAUDE_CONFIG_DIR/settings.json`, default `~/.claude/settings.json`) for the `startup|resume|clear|compact` sources, replacing any earlier memry hook and keeping every other setting and hook. It is installed even when MCP registration fails. When the settings file is not valid JSON it is left untouched and setup prints the hook to add by hand and exits with code 1.
- The Claude Code settings file is rewritten atomically (temporary file + rename), keeping its permissions and non-ASCII characters; a file whose `hooks` do not have the expected shape is also left untouched.
- A repo can declare its memry project in a `.memry.json` file at its root (`{"project": "memry"}`), so several repos of one product share a project. `memry hook:session-start` reads it from the git top-level (or the `cwd` outside git), trims the value and falls back to the directory name when the file is missing, unreadable, not valid JSON or has no non-empty string `project`.
- The SessionStart protocol asks for the repo (directory name) in `session-summary` and to save memories about another product under that product's project.
- This repo declares the `memry` project in `.memry.json`.
- `MEMRY_EXECUTABLE` sets the command Claude Code uses to run memry (for the `headersHelper` and the SessionStart hook), so a stable path survives upgrades of a versioned install such as Homebrew's. A custom `MEMRY_CONFIG` is prefixed to it as well.
- memry is built as a single-file PHAR with `php memry app:build memry --build-version=<version>` into `builds/` (ignored by git) and can be installed with `brew install mrtheroi/tap/memry`.
