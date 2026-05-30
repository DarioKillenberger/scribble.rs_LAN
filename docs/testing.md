# Testing

The repository uses Go tests across production packages. GitHub Actions validate on Windows, Linux, and macOS.

## Common Commands

Focused package tests:

```powershell
go test ./internal/api
go test ./internal/game
go test ./internal/frontend
go test ./internal/state
go test ./internal/translations
```

Full local test run:

```powershell
go test ./...
```

Race-sensitive confidence run:

```powershell
go test -race -count=3 ./...
```

Build:

```powershell
go build ./cmd/scribblers
```

Production-style build with version injection:

```powershell
go build -trimpath -ldflags "-w -s -X 'github.com/scribble-rs/scribble.rs/internal/version.Version=$(git describe --tags --dirty)'" -o scribblers ./cmd/scribblers
```

On PowerShell, replace the `$(...)` expression with an explicit version string if needed.

## Existing Test Ownership

- `internal/api/createparse_test.go`: request parsing and validation helpers.
- `internal/frontend/lobby_test.go`: lobby SSR behavior.
- `internal/frontend/templating_test.go`: template rendering and embedded frontend data.
- `internal/game/data_test.go`: shared game data helpers.
- `internal/game/lobby_test.go`: lobby state, scoring, turns, disconnects, guesses, and related game rules.
- `internal/game/words_test.go`: word list and word selection behavior.
- `internal/state/lobbies_test.go`: global lobby registry and cleanup behavior.
- `internal/translations/translations_test.go`: translation key consistency.

## Risk-Based Expectations

Run `go test ./...` for every code change.

Run `go test -race -count=3 ./...` for changes touching:

- `internal/game/lobby.go`
- `internal/game/data.go`
- `internal/api/ws.go`
- `internal/state/lobbies.go`
- timers, goroutines, WebSocket writes, or cleanup logic

Run focused frontend/template tests for changes touching:

- `internal/frontend/*.go`
- `internal/frontend/templates/*`
- embedded JS/CSS/resource references

Run translation tests for changes touching:

- `internal/translations/*`
- user-facing translated template or JS keys

## Manual Smoke Test

For behavior that spans browser and socket code:

1. Run `go run ./cmd/scribblers`.
2. Open `http://localhost:8080`.
3. Create a lobby.
4. Open a second browser or private window and join the lobby.
5. Start a game, choose a word, draw a line, send a guess, and confirm turn advancement.

Use this after automated tests when changing event contracts, canvas behavior, cookies, or lobby entry flows.

