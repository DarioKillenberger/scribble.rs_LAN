package api

import (
	json "encoding/json"
	"net/http"

	"github.com/scribble-rs/scribble.rs/internal/game"
	"github.com/scribble-rs/scribble.rs/internal/state"
)

type LanSetupData struct {
	LanControlToken string                   `json:"lanControlToken"`
	LanInputState   *game.LanInputStateEvent `json:"lanInputState"`
}

func (handler *V1Handler) getLanSetup(writer http.ResponseWriter, request *http.Request) {
	lobby, ok := handler.authorizeLanOwnerCookie(writer, request)
	if !ok {
		return
	}

	lobby.Synchronized(func() {
		_, _ = marshalToHTTPWriter(&LanSetupData{
			LanControlToken: lobby.LanControlToken,
			LanInputState:   lobby.LanInputState(),
		}, writer)
	})
}

func (handler *V1Handler) postLanInput(writer http.ResponseWriter, request *http.Request) {
	lobby, ok := handler.authorizeLanOwner(writer, request)
	if !ok {
		return
	}

	var input game.LanKeyboardInput
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	lobby.HandleLanKeyboardInput(input)
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *V1Handler) postLanAssignment(writer http.ResponseWriter, request *http.Request) {
	lobby, ok := handler.authorizeLanOwner(writer, request)
	if !ok {
		return
	}

	var assignment game.LanKeyboardAssignment
	if err := json.NewDecoder(request.Body).Decode(&assignment); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	if !lobby.AssignLanKeyboard(assignment.PlayerID, assignment.KeyboardID) {
		http.Error(writer, "invalid LAN keyboard assignment", http.StatusBadRequest)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (handler *V1Handler) authorizeLanOwner(writer http.ResponseWriter, request *http.Request) (*game.Lobby, bool) {
	lobby := state.GetLobby(GetLobbyId(request))
	if !validateLanLobby(writer, lobby) {
		return nil, false
	}

	if token := request.Header.Get("Lan-Control-Token"); token != "" && token == lobby.LanControlToken {
		return lobby, true
	}

	return handler.authorizeLanOwnerCookieForLobby(writer, request, lobby)
}

func (handler *V1Handler) authorizeLanOwnerCookie(writer http.ResponseWriter, request *http.Request) (*game.Lobby, bool) {
	lobby := state.GetLobby(GetLobbyId(request))
	if !validateLanLobby(writer, lobby) {
		return nil, false
	}

	return handler.authorizeLanOwnerCookieForLobby(writer, request, lobby)
}

func validateLanLobby(writer http.ResponseWriter, lobby *game.Lobby) bool {
	if lobby == nil {
		http.Error(writer, ErrLobbyNotExistent.Error(), http.StatusNotFound)
		return false
	}
	if lobby.LobbyMode != game.LobbyModeLanParty {
		http.Error(writer, "lobby is not in LAN-party mode", http.StatusBadRequest)
		return false
	}
	return true
}

func (handler *V1Handler) authorizeLanOwnerCookieForLobby(writer http.ResponseWriter, request *http.Request, lobby *game.Lobby) (*game.Lobby, bool) {
	userSession, err := GetUserSession(request)
	if err != nil {
		http.Error(writer, "no valid usersession supplied", http.StatusBadRequest)
		return nil, false
	}

	owner := lobby.GetOwner()
	if owner == nil || owner.GetUserSession() != userSession {
		http.Error(writer, "only the lobby owner can control LAN-party input", http.StatusForbidden)
		return nil, false
	}

	return lobby, true
}
