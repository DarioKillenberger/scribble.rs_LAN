# LAN Input Helper

`laninput` captures physical keyboard events for LAN-party mode and posts
keyboard-attributed input to a LAN lobby using the LAN control token shown in
the owner-only setup dialog.

The main `scribblers` server starts local native keyboard capture automatically
for LAN-party lobbies when `AUTO_LAN_INPUT=true` (the default). Use this helper
only for debugging, protocol testing, or non-standard setups.

On Windows, the default mode uses Raw Input and reports the Raw Input device
name as `keyboardId`.

Discover keyboard IDs:

```powershell
go run ./cmd/laninput -list
```

Run native capture:

```powershell
go run ./cmd/laninput -server http://localhost:8080 -lobby <lobby-id> -token <lan-control-token>
```

After the helper is running, press a key on each physical keyboard. The owner
setup dialog will show discovered keyboard IDs and lets you assign them to
players.
Assignments are fixed for the whole game. When the assigned player is the
current clue giver, their keyboard remains assigned but the server ignores its
input until that player's drawing turn ends.

For protocol testing, stdin mode accepts newline-delimited JSON:

```json
{"keyboardId":"kbd-1","key":"c","action":"keydown"}
{"keyboardId":"kbd-1","key":"Enter","action":"keydown"}
```

```powershell
go run ./cmd/laninput -stdin -server http://localhost:8080 -lobby <lobby-id> -token <lan-control-token>
```
