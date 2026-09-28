# memry CLI

Command-line client for [memry](https://github.com/mrtheroi/db-mcp), a private remote memory MCP server for AI agents.

`memry setup` will log in from the terminal with an email one-time code and wire memry into Claude Code (MCP server and SessionStart hook).

## Usage

```bash
memry setup                                   # asks for your email, then the 6-digit login code
memry setup --email you@example.com           # skip the email prompt
memry setup --url https://your-memry.example  # use another memry server
```

`memry setup` emails you a one-time login code, exchanges it for an API token, and saves
`{"url": "...", "token": "..."}` to `~/.config/memry/config.json` (permissions 0600).
Other keys already in that file are kept.

| Variable       | Purpose                                                              |
| -------------- | -------------------------------------------------------------------- |
| `MEMRY_URL`    | Default server URL when `--url` is not given                         |
| `MEMRY_CONFIG` | Alternative config file path (default `~/.config/memry/config.json`) |

## Development

- PHP 8.3+ · [Laravel Zero](https://laravel-zero.com) · Pest
- Run the tests with `./vendor/bin/pest`
