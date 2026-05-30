# Architecture

Scribble.rs is a single Go HTTP process that serves a pictionary-style game. The backend owns lobby state and game rules; the official browser client is server-rendered and receives embedded static assets.

## Process Startup

`cmd/scribblers/main.go` is the entry point:

1. Load config from `.env` and environment variables via `internal/config`.
2. Create an `http.ServeMux`.
3. Wrap registered routes with CORS settings.
4. Register `/health`.
5. Register public API and WebSocket endpoints through `internal/api`.
6. Register SSR and static frontend routes through `internal/frontend`.
7. Start global lobby cleanup through `internal/state` when configured.
8. On SIGINT or SIGTERM, broadcast lobby shutdown and stop CPU profiling if enabled.

Routes are registered with Go's method-aware pattern syntax, for example `GET /v1/lobby/{lobby_id}/ws`.

## Package Responsibilities

`internal/config` loads runtime configuration. Defaults live in `config.Default`; environment values override `.env` values.

`internal/api` owns the public HTTP API, request parsing, cookies, WebSocket upgrade, socket session binding, and transport-level serialization.

`internal/game` owns game rules and lobby internals: players, rounds, scoring, drawing events, guesses, hints, readiness, spectating, disconnect grace, kick votes, and the shared JSON event structs used by clients.

`internal/state` owns the process-global lobby list, public lobby lookup, stats aggregation, lobby cleanup, and graceful shutdown traversal.

`internal/frontend` owns server-rendered pages, embedded assets, template data, cache busting, CSP headers, Discord activity handling, and locale selection.

`internal/translations` owns localized UI strings. `en_us.go` is the source translation to use when adding keys.

`internal/metrics` registers Prometheus metrics under the API route tree.

## Request Flow

Home page:

```text
GET / -> frontend.indexPageHandler -> templates/index.html
```

Create lobby from browser:

```text
POST /lobby
  -> frontend.ssrCreateLobby
  -> api parse helpers
  -> game.CreateLobby
  -> api.SetGameplayCookies
  -> state.AddLobby
  -> redirect to /lobby/{id}
```

Create lobby from API:

```text
POST /v1/lobby
  -> api.postLobby
  -> api parse helpers
  -> game.CreateLobby
  -> api.SetGameplayCookies
  -> state.AddLobby
  -> JSON LobbyData
```

Join or re-enter lobby page:

```text
GET /lobby/{lobby_id}
  -> frontend.ssrEnterLobby
  -> state.GetLobby
  -> api.GetPlayer or game.JoinPlayer
  -> api.SetGameplayCookies
  -> templates/lobby.html
```

Connect gameplay socket:

```text
GET /v1/lobby/{lobby_id}/ws
  -> api.websocketUpgrade
  -> api.GetUserSession
  -> state.GetLobby
  -> lobby.GetPlayerBySession
  -> gws.Upgrade
  -> lobby.OnPlayerConnectUnsynchronized
  -> socket.ReadLoop
```

Incoming socket event:

```text
gws.OnMessage
  -> api.handleIncommingEvent
  -> unmarshal EventTypeOnly
  -> lobby.HandleEvent
  -> mutate lobby and broadcast events
```

## Lobby Lifecycle

A lobby is created by `game.CreateLobby`, then added to global state only after cookies and response data can be written successfully. The lobby stores callback functions for transport writes:

- `Lobby.WriteObject`
- `Lobby.WritePreparedMessage`

Those callbacks are supplied by `internal/api`, which keeps the game package independent from HTTP handler code while still letting game logic broadcast to sockets.

During play, `advanceLobby` and `advanceLobbyPredefineDrawer` determine the next drawer, reset per-turn state, send `next-turn` or `game-over`, and start a ticker for turn timing. `tickLogic` reveals hints, handles timeout, and advances early for disconnected drawers or guessers.

Disconnected players keep their slot for `slotReservationTime`. Empty lobbies are removed by `internal/state` after the configured cleanup threshold.

## Shared Client Contract

The JSON event contract is centered in `internal/game/shared.go`.

Incoming-only events include:

- `start`
- `toggle-readiness`
- `toggle-spectate`
- `request-drawing`
- `choose-word`
- `undo`

Outgoing-only events include:

- `ready`
- `your-turn`
- `next-turn`
- `word-chosen`
- `update-wordhint`
- `update-players`
- `correct-guess`
- `close-guess`
- `drawing`
- `game-over`
- `shutdown`

Bidirectional events include:

- `message`
- `line`
- `fill`
- `clear-drawing-board`
- `kick-vote`
- `name-change`

When changing event payloads, update server structs, browser handling, tests, and documentation together.

## Concurrency Model

There are two important locks:

- `state.globalStateMutex` protects the global lobby slice.
- `game.Lobby.mutex` protects per-lobby mutable game state.

Avoid holding the global state lock while doing long-running per-lobby work. Existing state cleanup and shutdown paths intentionally keep their locking simple; inspect them before adding nested locking.

Use `Lobby.Synchronized` for multi-step lobby operations from API/frontend packages. Inside `internal/game`, follow the existing pattern in `HandleEvent`, `tickLogic`, and disconnect handling.

## Frontend Model

The frontend does not use a bundler. Go embeds:

- `internal/frontend/templates/*`
- `internal/frontend/resources/*`
- `internal/frontend/index.js`
- `internal/frontend/lobby.js`

`internal/frontend/index.go` templates JS and computes cache-busting hashes. `internal/frontend/http.go` serves embedded resources with long cache headers and CSP. Canvas drawing implementation lives in `internal/frontend/resources/draw.js`.

## Deployment Notes

Dockerfiles exist for Linux, Windows, and Fly.io. GitHub Actions run tests on Windows, Linux, and macOS. Push builds use `go test -v -race ./...`; pull requests use `go test -v -race -count=3 ./...`.

