package laninput

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/scribble-rs/scribble.rs/internal/game"
)

var (
	activeLobby atomic.Pointer[game.Lobby]
	startOnce   sync.Once
)

// ActivateLobby makes lobby the single local target for native LAN keyboard
// input. The native capture loop is process-wide and starts on first use.
func ActivateLobby(lobby *game.Lobby) {
	if lobby == nil || lobby.LobbyMode != game.LobbyModeLanParty {
		return
	}
	activeLobby.Store(lobby)
	startOnce.Do(func() {
		go func() {
			log.Println("starting local LAN keyboard capture")
			if err := Capture(handleCapturedInput, false); err != nil {
				log.Printf("local LAN keyboard capture unavailable: %v", err)
			}
		}()
	})
}

func handleCapturedInput(input game.LanKeyboardInput) {
	lobby := activeLobby.Load()
	if lobby == nil {
		return
	}
	lobby.HandleLanKeyboardInput(input)
}
