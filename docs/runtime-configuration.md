# Runtime Configuration

Configuration is loaded by `internal/config.Load`.

1. Read `.env` from the working directory if present.
2. Overlay process environment variables.
3. Parse into `config.Config` with defaults from `config.Default`.
4. Normalize `RootURL`, `CanonicalURL`, and `RootPath`.

## Important Settings

- `PORT`: HTTP port. Default `8080`.
- `NETWORK_ADDRESS`: bind address. Empty means all interfaces.
- `ROOT_PATH`: path prefix for hosting under a subpath.
- `ROOT_URL`: public scheme and host used for metadata.
- `CANONICAL_URL`: canonical public URL. Defaults to `ROOT_URL`.
- `ALLOW_INDEXING`: controls index/noindex behavior on the home page.
- `SERVE_DIRECTORIES`: optional map of extra directories served by the frontend handler.
- `CPU_PROFILE_PATH`: enables CPU profiling output.
- `CORS_ALLOWED_ORIGINS`: API CORS origins. Default `*`.
- `CORS_ALLOW_CREDENTIALS`: API CORS credential behavior.
- `LOBBY_CLEANUP_INTERVAL`: cleanup tick interval. Set `0` to disable.
- `LOBBY_CLEANUP_PLAYER_INACTIVITY_THRESHOLD`: age after which empty lobbies can be cleaned.

Lobby creation defaults use the `LOBBY_SETTING_DEFAULTS_` prefix. Bounds use `LOBBY_SETTING_BOUNDS_`.

## Config Change Checklist

- Add the field to `internal/config/config.go`.
- Add a default in `config.Default` when appropriate.
- Document the variable in this file and `README.md` if user-facing.
- Add or update tests if parsing, normalization, or default behavior changes.
- Consider whether SSR defaults and API parsing both need the new setting.

