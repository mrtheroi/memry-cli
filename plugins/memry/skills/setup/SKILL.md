---
name: setup
description: Install and connect memry, the persistent memory for AI coding agents. Use when the user wants to set up memry, asks why memry tools are missing, or wants Claude Code to remember context between sessions.
---

# Set up memry

memry stores decisions, bug fixes and discoveries on a memry server and gives them back to the agent at the start of the next session. The `memry` CLI does the setup: it logs the user in and registers memry in each agent they choose. This plugin does not register memry itself, so the agent never ends up with the server twice.

Do not try to diagnose an existing installation, and do not read Claude Code's settings files: they can hold credentials. `memry setup` can be run again at any time, and it repairs a partial or broken setup.

Follow these steps in order and stop at the first one that needs the user.

1. **Check whether the CLI is installed.** Run `command -v memry` (on Windows, `where.exe memry`; in PowerShell, `where` without `.exe` is a different command).
   - On Windows, if the first path it prints is inside the current project folder, stop: CMD runs a `memry.exe` from the current folder before the one on the `PATH`. Tell the user not to run it, and to run the remaining steps from their home folder instead.
   - If it is missing, give the user the install command for their system and wait until they confirm it ran:
     - macOS or Linux with Homebrew: `brew install mrtheroi/tap/memry`
     - Any system with Go: `go install github.com/mrtheroi/memry-cli/cmd/memry@latest`. Go puts the binary in `$(go env GOBIN)` or `$(go env GOPATH)/bin`, which is not on the `PATH` by default; the user must add that folder to their `PATH` and open a new terminal.
     - Otherwise, download a release from https://github.com/mrtheroi/memry-cli/releases and follow https://github.com/mrtheroi/memry-cli#install.
   - Then run the lookup again. Continue only when it finds `memry`.

2. **Log in and connect.** `memry setup` is interactive: it asks for the user's email, sends a 6-digit code and asks which agents to connect. Do not run it yourself. Ask the user whether they set memry up before with a custom config file (`--config`), then ask them to run it in their terminal:
   - memry Cloud: `memry setup`
   - A self-hosted memry server, with a token from their admin: `memry setup --url https://their-memry-server --token`
   - With a custom config file, add the same `--config <path>` they used before, so the repair keeps that file.

3. **Start a new session.** Setup registers the `memry` MCP server and a hook that loads the user's memories when a session starts. Both take effect in the next Claude Code session. If memry's tools are still missing afterwards, the user can check `/mcp` for another server named `memry` in the project, and whether hooks are disabled in their settings.

To remove memry later, the user runs `memry uninstall`. `memry delete-account` erases their account and memories.

More: https://www.memry.com.mx
