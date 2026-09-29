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
