# memry CLI in Go

This directory holds the Go rewrite of the memry CLI. It is meant to be a drop-in replacement
for the PHP CLI in the repository root: the same config file, environment variables, commands,
flags, messages and exit codes, so users only need to `brew upgrade`.

The PHP CLI keeps shipping until a final cutover. Its Pest tests in `../tests` are the behavioral
spec: a Go test that ports one names it in a comment.

| Package             | Responsibility                                                              |
| ------------------- | --------------------------------------------------------------------------- |
| `cmd/memry`         | Entry point; the version is set with `-ldflags "-X main.version=<version>"` |
| `internal/commands` | Cobra commands: the root command, `--version` and `setup`                   |
| `internal/config`   | `config.json` path, load and save, in the PHP CLI's format                  |
| `internal/fsx`      | Atomic file writes                                                          |
| `internal/client`   | The HTTP client: no redirects, 2xx-only success, timeout, User-Agent        |
| `internal/flags`    | The pure resolver of `memry setup` options, interaction and verbosity       |
| `internal/setup`    | `memry setup`: the email and token logins (agent wiring is not ported yet)  |
| `internal/prompt`   | Terminal questions read like Symfony's, with a hidden prompt                |
| `internal/email`    | Email validation matching PHP's `FILTER_VALIDATE_EMAIL`                     |
| `internal/agents`   | The agent registry interface; a stub of the PHP agent keys for now          |

## Running the tests

With Go (the version in `go.mod`):

```bash
cd go
go vet ./...
go test -race ./...
```

CI also runs [golangci-lint](https://golangci-lint.run) (see `.github/workflows/go.yml` for the
pinned version). To run it locally with Docker:

```bash
docker run --rm -v "$PWD":/app -w /app golangci/golangci-lint:v2.14.0 golangci-lint run ./...
```
