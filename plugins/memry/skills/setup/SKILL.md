---
name: setup
description: Install and connect memry, the persistent memory for AI coding agents. Use when the user wants to set up memry, asks why memry tools are missing, or wants Claude Code to remember context between sessions.
---

# Set up memry

memry stores decisions, bug fixes and discoveries on a memry server and gives them back to the agent at the start of the next session. The `memry` CLI does the setup: it logs the user in and registers memry in each agent they choose. This plugin does not register memry itself, so the agent never ends up with the server twice.

Follow these steps in order and stop at the first one that needs the user.

1. **Check whether memry is already connected.** Run `claude mcp get memry`. If it succeeds, memry is set up: tell the user to start a new session if its tools are not loaded yet, and stop.

2. **Check whether the CLI is installed.** Run `command -v memry` (on Windows, `where memry`).
   - If it is missing, give the user the install command for their system and wait until they confirm it ran:
     - macOS or Linux with Homebrew: `brew install mrtheroi/tap/memry`
     - Any system with Go: `go install github.com/mrtheroi/memry-cli/cmd/memry@latest`
     - Otherwise, download a release from https://github.com/mrtheroi/memry-cli/releases and follow https://github.com/mrtheroi/memry-cli#install.

3. **Log in and connect.** `memry setup` is interactive: it asks for the user's email, sends a 6-digit code and asks which agents to connect. Do not run it yourself. Ask the user to run it in their terminal:
   - memry Cloud: `memry setup`
   - A self-hosted memry server, with a token from their admin: `memry setup --url https://their-memry-server --token`

4. **Start a new session.** Setup registers the `memry` MCP server and a hook that loads the user's memories when a session starts. Both take effect in the next Claude Code session.

To remove memry later, the user runs `memry uninstall`. `memry delete-account` erases their account and memories.

More: https://www.memry.com.mx
