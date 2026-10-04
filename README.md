# Junior Assistant

A local terminal assistant for macOS, written in Go. Find files and folders, track project or personal tasks and bugs, and see what needs attention today. Runtime is offline: no AI, telemetry, cloud integration, or background service.

## Build and use

Install Go 1.26 or newer. The first build downloads dependencies; the built executable runs offline and needs no separate SQLite installation.

```sh
go build -o bin/assistant ./cmd/assistant
./bin/assistant --help
./bin/assistant project add junior-assistant "$PWD"
./bin/assistant task add "Try the assistant" --planned "$(date +%F)"
./bin/assistant today
./bin/assistant find README
```

Optionally install the executable into Go's binary directory (normally `~/go/bin`); add that directory to your PATH to use `assistant` directly:

```sh
go install ./cmd/assistant
```

See the [command reference](docs/commands.md) for scopes, search exclusions, backup/restore, output contracts, and exit codes.

## Verify

```sh
go test -race ./...
go vet ./...
go build -o bin/assistant ./cmd/assistant
python3 scripts/smoke.py
```

The optional Python smoke check uses macOS `sandbox-exec` to deny network access and keeps all state in disposable directories. [Acceptance evidence](docs/v1-acceptance.md) records the completed checks.

## Design and tracking

- [Version-one specification](docs/v1-spec.md)
- [GitHub implementation plan](docs/v1-issues.md)
- [Domain glossary](GLOSSARY.md)
- [Architecture decision](docs/adr/0001-local-command-assistant.md)

GitHub Issues track development of this repository. The installed assistant does not access GitHub Issues or other external services.
