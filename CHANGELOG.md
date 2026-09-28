# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `memry setup` command: log in with an email one-time code and save the server URL and token to `~/.config/memry/config.json` (or `$MEMRY_CONFIG`), readable by the owner only.
- `--url` and `--email` options for `memry setup`; the default server URL can be overridden with `MEMRY_URL`.
- `memry setup` exits with code 1 and writes nothing when a request fails, including an unreachable server, rate limiting, unexpected server errors and a response without a token.
