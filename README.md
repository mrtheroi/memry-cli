<p align="center">
  <img src="art/memry-logo.png" alt="memry" width="400">
</p>

# memry

memry gives your AI coding agents a persistent memory that carries over between sessions and
projects. Decisions, bug fixes and discoveries saved in
one session are available in the next one, so you do not have to explain the same context again.

## Requirements

- macOS or Linux
- [Homebrew](https://brew.sh)
- At least one [supported agent](#supported-agents). For Claude Code, its CLI (`claude`) must be on
  your `PATH`.

## Install

```bash
brew install mrtheroi/tap/memry
memry setup
```

`memry setup` asks for your email, sends you a 6-digit login code, then asks which agents you use
and connects memry to each one you select (the ones found on your machine are preselected; press
space to select, enter to confirm). Start a new session in your agent afterwards to use it.

To choose the agents without the prompt, for example in a script, pass their keys:

```bash
memry setup --agents=claude-code,codex
```

Run `memry setup` again to change the selection; memry is removed from agents you deselect.

### Self-hosted

On a self-hosted memry server, your admin creates a token for you instead of the email login.
Connect with:

```bash
memry setup --url https://memry.company.internal --token
```

`--url` is required with `--token`, so the token is never sent to the public server. Setup asks for
the token without showing it, checks it with the server before saving it, then asks
which agents you use as usual. For scripts, `--token=<token>` passes it without the prompt, but the
token then lands in your shell history.

## Supported agents

| Agent                                                            | Key           | What `memry setup` adds                                                                         |
| ---------------------------------------------------------------- | ------------- | ----------------------------------------------------------------------------------------------- |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code)    | `claude-code` | The `memry` MCP server (user scope) and a hook that loads your memories when a session starts.  |
| [Codex](https://developers.openai.com/codex)                     | `codex`       | The `memry` MCP server in `~/.codex/config.toml` and memry instructions in `~/.codex/AGENTS.md`. |
| [OpenCode](https://opencode.ai)                                  | `opencode`    | The `memry` MCP server in `~/.config/opencode/opencode.json` and memry instructions in `~/.config/opencode/AGENTS.md`. |
| [Antigravity](https://antigravity.google)                        | `antigravity` | The `memry` MCP server in `~/.gemini/config/mcp_config.json` and memry instructions in `~/.gemini/config/GEMINI.md`. |
| [Windsurf](https://windsurf.com)                                 | `windsurf`    | The `memry` MCP server in `~/.codeium/windsurf/mcp_config.json` and memry instructions in its global rules (`~/.codeium/windsurf/memories/global_rules.md`). |

Setup only adds memry's own entries: everything else in those files is kept, and `memry uninstall`
removes just what setup added. Your login token is never written to an agent's files.

## How it works

- **Login by email code.** You sign in with a one-time code sent to your email; there is no
  password to remember.
- **Agent integration.** Setup registers a `memry` MCP server in each agent you select, which
  gives the agent the tools to save and search memories.
- **Context at session start.** Each new session loads the recent memories of the current
  project, so your agent picks up where you left off. Claude Code loads them with a hook (also on
  resumed and compacted sessions); the other agents are instructed to load them as their first
  step.
- **Projects.** By default, a repository's project is its directory name. To group several
  repositories into one project (for example, the backend and frontend of one product), add a
  `.memry.json` file at the root of each repository:

  ```json
  {"project": "my-product"}
  ```

## Update

```bash
brew upgrade memry
```

Your login and agent configuration keep working after an upgrade.

## Uninstall

```bash
memry uninstall
brew uninstall memry
```

`memry uninstall` asks for confirmation, then removes memry from your agents and deletes your
local login. Use `memry uninstall --force` to skip the confirmation.

### Delete your account

```bash
memry delete-account
```

`memry delete-account` permanently deletes your memry account and all its memories on the server,
then removes memry from your agents and deletes your local login, like `memry uninstall`. It cannot
be undone, so it asks you to type your account email to confirm.

## Your data

Your memories are stored on the memry server and tied to your account. `memry uninstall` revokes
the token on this machine, so it can no longer access them. `memry delete-account` deletes them.
See the [privacy policy](PRIVACY.md) ([en español](PRIVACY.es.md)) for what memry stores and who handles it.

## License

memry is released under the [MIT license](LICENSE). See [CHANGELOG.md](CHANGELOG.md) for release notes and
[CONTRIBUTING.md](CONTRIBUTING.md) for development and troubleshooting.
