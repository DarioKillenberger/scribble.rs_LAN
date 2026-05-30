# Agent Guide

This repository is a Go 1.25 web game server with an embedded server-rendered frontend. Treat `internal/game` and the WebSocket event contract as the core product surface: most user-visible behavior flows through lobby state, player state, and JSON events shared with the browser client.

## Quick Start

- Read [docs/architecture.md](docs/architecture.md) before changing cross-package behavior.
- Read [docs/agentic-coding.md](docs/agentic-coding.md) before decomposing larger work across agents.
- Read [docs/testing.md](docs/testing.md) before claiming a change is complete.
- Run focused tests first, then `go test ./...`; for PR-level confidence run `go test -race -count=3 ./...`.
- Build with `go build ./cmd/scribblers`.

## Repository Map

- `cmd/scribblers/main.go`: process entry point. Loads config, wires routes, enables CORS, starts lobby cleanup, handles shutdown.
- `internal/api`: public HTTP API and WebSocket upgrade/transport handling.
- `internal/game`: lobby model, player model, game loop, event handling, scoring, words, and client-shared event structs.
- `internal/state`: process-global lobby registry and cleanup.
- `internal/frontend`: server-rendered pages, embedded templates, embedded JS/CSS/assets, localization selection.
- `internal/translations`: UI translation maps and translation tests.
- `internal/config`: environment and `.env` config loading.
- `internal/metrics`: Prometheus metrics endpoint integration.
- `tools`: one-off maintenance and conversion utilities; these are not production packages.

## Non-Negotiable Invariants

- Do not expose `game.Lobby` directly through HTTP or WebSocket APIs. It contains hidden gameplay state such as `CurrentWord`, word choices, internal timers, sessions, and sockets.
- Keep client-visible contracts in `internal/game/shared.go` unless there is a strong reason to move them.
- Synchronize lobby mutations with `Lobby.Synchronized` or the lobby mutex path already used by the caller. Avoid adding unsynchronized reads that affect decisions.
- Preserve WebSocket event ordering. Server writes use async or prepared broadcast paths consistently; changing write mode can reorder client-visible events.
- Preserve cookie/header compatibility for `usersession` and `lobby-id`; public API and SSR flows both depend on them.
- Preserve `ROOT_PATH` behavior by using `path.Join` with `cfg.RootPath` or `BasePageConfig.RootPath` patterns already in the code.
- Keep frontend resources embeddable. Files under `internal/frontend/templates` and `internal/frontend/resources` are embedded via `go:embed`.
- Do not add external build steps for the frontend unless the repo is intentionally migrated; the current frontend is plain embedded JS/CSS/templates.

## Coding Standards

- Prefer small changes that match existing package boundaries.
- Use standard library APIs and existing helpers before adding dependencies.
- Keep API parsing and validation in `internal/api/createparse.go` or the adjacent API handler when it is endpoint-specific.
- Keep reusable game rule behavior in `internal/game`, not in API or frontend packages.
- Keep state registry behavior in `internal/state`; do not add new package-level lobby registries.
- Update translation keys in `internal/translations/en_us.go` first, then mirror required keys in other translations.
- For user-facing behavior changes, update both server-side templates and browser JS if the event contract or page data changes.

## Validation Commands

Use the smallest meaningful validation while iterating:

```powershell
go test ./internal/game
go test ./internal/api
go test ./internal/frontend
go test ./...
go test -race -count=3 ./...
go build ./cmd/scribblers
```

Use the PR-level race command before declaring broad lobby, WebSocket, or state changes complete.
On this Windows machine, `-race` needs cgo and MSYS2 GCC on `PATH`; use:

```powershell
$env:PATH='C:\Program Files\Go\bin;C:\msys64\ucrt64\bin;' + $env:PATH
$env:CGO_ENABLED='1'
go test -race -count=3 ./...
```

## High-Risk Areas

- `internal/game/lobby.go`: large state machine with timers, scoring, hints, drawing history, readiness, spectating, disconnect grace, and kick logic.
- `internal/api/ws.go`: WebSocket lifecycle, session storage, deadlines, and incoming event dispatch.
- `internal/state/lobbies.go`: global process state and cleanup. Deadlocks or stale references here affect every lobby.
- `internal/frontend/lobby.js` and `internal/frontend/resources/draw.js`: browser event handling and drawing fidelity.
- Cookie behavior in `internal/api/v1.go` and SSR lobby entry in `internal/frontend/lobby.go`, especially Discord activity cookies.

## Done Definition

A change is done when:

- The implementation matches the existing package boundaries.
- New or changed behavior has focused tests where practical.
- Relevant docs are updated when commands, contracts, config, or architecture change.
- `go test ./...` passes at minimum.
- Race tests are run or explicitly called out as not run for changes touching lobby state, WebSockets, timers, or cleanup.
