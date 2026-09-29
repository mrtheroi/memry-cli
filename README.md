<p align="center">
  <img src="art/memry-logo.png" alt="memry" width="400">
</p>

# memry

memry gives [Claude Code](https://docs.anthropic.com/en/docs/claude-code) a persistent memory
that carries over between sessions and projects. Decisions, bug fixes and discoveries saved in
one session are available in the next one, so you do not have to explain the same context again.

## Requirements

- macOS or Linux
- [Homebrew](https://brew.sh)
- The [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI (`claude`) on your `PATH`

## Install

```bash
brew install mrtheroi/tap/memry
memry setup
```

`memry setup` asks for your email, sends you a 6-digit login code and connects memry to
Claude Code. Start a new Claude Code session afterwards to use it.

## How it works

- **Login by email code.** You sign in with a one-time code sent to your email; there is no
  password to remember.
- **Claude Code integration.** Setup registers a `memry` MCP server in Claude Code, which gives
  Claude the tools to save and search memories, and adds a hook that runs when a session starts.
- **Context at session start.** Each new, resumed or compacted session loads the recent memories
  of the current project, so Claude picks up where you left off.
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

Your login and Claude Code configuration keep working after an upgrade.

## Uninstall

```bash
memry uninstall
brew uninstall memry
```

`memry uninstall` asks for confirmation, then removes memry from Claude Code and deletes your
local login. Use `memry uninstall --force` to skip the confirmation.

## Your data

Your memories are stored on the memry server and tied to your account. `memry uninstall` revokes
the token on this machine, so it can no longer access them.

## License

memry is released under the MIT license. See [CHANGELOG.md](CHANGELOG.md) for release notes and
[CONTRIBUTING.md](CONTRIBUTING.md) for development and troubleshooting.
