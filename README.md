<p align="center">
  <img src="art/memry-logo.png" alt="memry" width="400">
</p>

# memry

memry gives your AI coding agents a persistent memory that carries over between sessions and
projects. Decisions, bug fixes and discoveries saved in
one session are available in the next one, so you do not have to explain the same context again.

## Requirements

- macOS, Linux or Windows (amd64 or arm64)
- At least one [supported agent](#supported-agents). For Claude Code, its CLI (`claude`) must be on
  your `PATH`.

## Install

With [Homebrew](https://brew.sh):

```bash
brew install mrtheroi/tap/memry
memry setup
```

Other ways to install memry, a single binary with no other dependency:

- With Go: `go install github.com/mrtheroi/memry-cli/cmd/memry@latest`. The binary lands in
  `$(go env GOPATH)/bin`, which must be on your `PATH`.
- From a [release](https://github.com/mrtheroi/memry-cli/releases): download the
  `memry_<version>_<os>_<arch>.tar.gz` archive for your system, check it against `checksums.txt`,
  and put the `memry` binary from it on your `PATH`, for example:

  ```bash
  tar -xzf memry_1.0.0_darwin_arm64.tar.gz memry
  sudo install -m 0755 memry /usr/local/bin/memry
  ```

On Windows, see [Windows](#windows).

Your agents run memry from the path it is installed at, so outside Homebrew keep the binary where
it is, or run `memry setup` again after you move it.

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
which agents you use as usual. For scripts, set the `MEMRY_TOKEN` environment variable: with
`--token` and no value, setup uses it instead of the prompt, also with `--no-interaction`, and the
token stays out of the process list. `--token=<token>` also skips the prompt (and wins over
`MEMRY_TOKEN`), but the token then lands in your shell history and the process list.

```bash
MEMRY_TOKEN="$token" memry setup --url https://memry.company.internal --token --no-interaction
```

`MEMRY_TOKEN` is only read together with `--token`, so it never changes the email login.

Use an `https://` URL. Setup sends your token or login code to the server, and `memry mcp` sends the
saved token with every request, so a plain `http://` URL is only accepted for this machine
(`localhost`, a `127.x.x.x` address or `[::1]`). For any other `http://` server setup stops before
asking or sending anything: "Refusing to send credentials over plain HTTP to <host>. Use an https://
URL, or pass --insecure to allow it (for example on a trusted LAN).". Pass `--insecure` only on a network
you trust: the credentials then travel in clear text. The same rule applies to `MEMRY_URL`.

### Windows

Install with [WinGet](https://learn.microsoft.com/windows/package-manager/), then run setup in a
new terminal so the `memry` command is on your `PATH`:

```powershell
winget install Memry.Memry
memry setup
```

Other ways to install it:

- With Go: `go install github.com/mrtheroi/memry-cli/cmd/memry@latest`. The binary lands in
  `%GOPATH%\bin` (`go env GOPATH`), which must be on your `PATH`.
- From a [release](https://github.com/mrtheroi/memry-cli/releases): download
  `memry_<version>_windows_<arch>.zip` (`amd64` or `arm64`), check it against `checksums.txt`
  (signed with cosign, see the cosign command in the [release process](CONTRIBUTING.md#release-process)), and put
  `memry.exe` from it on your `PATH`.

Update with `winget upgrade Memry.Memry`. To uninstall, run `memry uninstall`, then
`winget uninstall memry`; for a manual install, `memry uninstall` prints the path of the
`memry.exe` to delete.

What differs from macOS and Linux:

- **Config file.** `%USERPROFILE%\.config\memry\config.json`, readable only by your user. memry
  finds your home in `USERPROFILE`, then `HOME`, then two levels above `LOCALAPPDATA`; without any,
  it stops with "Could not find your home directory; set USERPROFILE or HOME." (on every OS, an
  unset home is an error). Use [`--config`](#custom-config-file) or `MEMRY_CONFIG` to put it elsewhere.
- **Claude Code.** Setup registers memry as a local `memry mcp` server
  (`claude mcp add --transport stdio --scope user memry -- <memry.exe> mcp`) and adds the same
  SessionStart hook. Both `claude.exe` and `claude.cmd` are supported.
- **The hook runs `memry`** by its name, so `memry` must be on your `PATH`. WinGet guarantees it;
  with `go install`, add `%GOPATH%\bin` to your `PATH`. Claude Code hands its own `PATH` to the
  hook, so after installing, **restart the terminal or editor (VS Code, JetBrains) you start
  Claude Code from**; a window opened before the install does not see `memry`, and the hook
  then loads no context without any error. If you run Claude Code with CMD instead of
  Git Bash, CMD searches the current directory first, so do not start sessions in a folder that
  contains an untrusted `memry.exe`.
- **Antivirus and SmartScreen.** `memry.exe` is not Authenticode-signed, so SmartScreen or your
  antivirus may warn about it. To check a download, verify the signed `checksums.txt` as described
  in [CONTRIBUTING.md](CONTRIBUTING.md#release-process) and compare the archive's SHA-256.
- **Terminals without color support.** In a legacy console, the agent question is replaced by
  `Agents: <list> (pass --agents to choose).` and keeps the defaults; use `--agents` to choose.

### Custom config file

By default memry keeps your login in `~/.config/memry/config.json`. To use another file, set the
`MEMRY_CONFIG` environment variable or pass the global `--config <path>` (or `--config=<path>`)
option to `setup`, `uninstall` and `delete-account`:

```bash
memry setup --config ~/work/memry.json
```

The precedence is `--config`, then `MEMRY_CONFIG`, then the default; each applies to that one
process. Setup passes the file on to the commands it registers in your agents, so they read the
same login.

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

## Usage metrics (opt-in)

memry sends no telemetry. If you want the memry dashboard to chart your Claude Code token usage and
cost, turn it on explicitly:

```bash
memry usage enable    # off by default
memry usage status    # on or off, and where the metrics go (never prints the token)
memry usage disable   # turn it off again
```

`memry usage enable` first tells you what is sent, then asks your memry server for a usage token
(it can only send usage metrics) and sets Claude Code's OpenTelemetry metrics export in the `env`
of `~/.claude/settings.json` (or `$CLAUDE_CONFIG_DIR/settings.json`): `CLAUDE_CODE_ENABLE_TELEMETRY`,
`OTEL_METRICS_EXPORTER`, `OTEL_EXPORTER_OTLP_METRICS_PROTOCOL`, `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`,
`OTEL_EXPORTER_OTLP_METRICS_HEADERS`, and `OTEL_METRICS_INCLUDE_SESSION_ID`,
`OTEL_METRICS_INCLUDE_ACCOUNT_UUID` and `OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES` set to `false`.
Restart Claude Code to apply it. The settings file holds the usage token, so memry leaves it
readable by your user only.

What is sent, only to the server you logged in to:

- Claude Code has no per-metric filter, so it sends all 8 of its metrics: session count, lines of
  code, pull requests, commits, cost, token usage, code edit tool decisions and active time.
- Each metric carries your email, your organization id, an anonymous user id and the terminal type.
  Claude Code cannot leave these out. memry turns off the session id, the account uuid and the
  resource attributes.
- No prompts or code are sent: memry does not turn on Claude Code's logs or traces.
- The memry server keeps only the token and cost data points, tied to your memry account: the
  count or cost, the model, the token type, the query source and the time window. It discards the
  other metrics on receipt and does not store your email or the other attributes; it only hashes
  them, with the metric, into a series identity.

- `enable` stops, before asking for a token and whatever `--force` says, when:
  - one of those variables is exported in the environment you run it from with another value
    (Claude Code's launch environment wins over its settings, so it would override memry's);
  - `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS` or `OTEL_EXPORTER_OTLP_PROTOCOL` is
    set in the environment or the settings (they would redirect your other telemetry, and Claude
    Code adds the generic headers, often another collector's credentials, to the ones it sends to
    memry);
  - `OTEL_LOGS_EXPORTER` or `OTEL_TRACES_EXPORTER` is set (and not `none`) while you have not
    turned telemetry on yourself: `CLAUDE_CODE_ENABLE_TELEMETRY` would start those exports too.
- When one of memry's variables is set in `settings.json` to a value memry did not write, `enable`
  stops and names it; `--force` replaces it. A plain `http://` server that is not this machine
  needs `--insecure`, like `memry setup`.
- memry recognizes its own export by the metrics endpoint: the one of the server you are logged in
  to, or the one it saved under `usage_metrics` in the memry config. `disable` removes only the
  variables memry wrote and that still have memry's values, keeps the ones you had already set to
  the same value, and drops the `env` object if that leaves it empty. `memry uninstall` turns usage
  metrics off too.

## Update

```bash
brew upgrade memry
```

On Windows, run `winget upgrade Memry.Memry`. Without a package manager, run the same `go install`
again, or replace the binary with the one from a newer release. Your login and agent configuration
keep working after an upgrade.

## Uninstall

```bash
memry uninstall
brew uninstall memry
```

`memry uninstall` asks for confirmation, then removes memry from your agents and deletes your
local login. Use `memry uninstall --force` to skip the confirmation. Without Homebrew, delete the
`memry` binary instead of running `brew uninstall`. On Windows, run `winget uninstall memry`
instead (`memry uninstall` prints the hint that fits your install).

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
Usage metrics are sent only if you turn them on (see
[Usage metrics](#usage-metrics-opt-in)). See the [privacy policy](PRIVACY.md) ([en español](PRIVACY.es.md)) for what memry stores and who handles it.

## License

memry is released under the [MIT license](LICENSE). See [CHANGELOG.md](CHANGELOG.md) for release notes and
[CONTRIBUTING.md](CONTRIBUTING.md) for development and troubleshooting.
