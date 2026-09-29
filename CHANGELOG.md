# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
