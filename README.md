# memry CLI

Command-line client for [memry](https://github.com/mrtheroi/db-mcp), a private remote memory MCP server for AI agents.

`memry setup` logs in from the terminal with an email one-time code and registers memry in Claude Code as the `db-memory` MCP server.

Requires the [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI (`claude`) on your `PATH`.

## Usage

```bash
memry setup                                   # asks for your email, then the 6-digit login code
memry setup --email you@example.com           # skip the email prompt
memry setup --url https://your-memry.example  # use another memry server
```

`memry setup` emails you a one-time login code, exchanges it for an API token, and saves
`{"url": "...", "token": "..."}` to `~/.config/memry/config.json` (permissions 0600).
Other keys already in that file are kept.

It then registers the `db-memory` MCP server in Claude Code (user scope), replacing any existing entry:

```bash
claude mcp add-json --scope user db-memory '{"type":"http","url":"<url>/mcp/memory","headersHelper":"<memry> mcp-headers"}'
```

The server uses a `headersHelper`: Claude Code runs `memry mcp-headers`, which prints
`{"Authorization":"Bearer <token>"}` from `config.json`. The token lives only in `config.json`
and is never stored in Claude Code's configuration.

If the `claude` CLI is not found or the registration fails, `memry setup` keeps your login,
prints the command above to run by hand, and exits with code 1.

| Variable       | Purpose                                                              |
| -------------- | -------------------------------------------------------------------- |
| `MEMRY_URL`    | Default server URL when `--url` is not given                         |
| `MEMRY_CONFIG` | Alternative config file path (default `~/.config/memry/config.json`); passed on to the `headersHelper` |

## Development

- PHP 8.3+ · [Laravel Zero](https://laravel-zero.com) · Pest
- Run the tests with `./vendor/bin/pest`
