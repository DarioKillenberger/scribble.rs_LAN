package game

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofrs/uuid/v5"
)

const lanInputRevealDuration = 100 * time.Millisecond

var lanPlayerColors = []string{
	"#ef130b",
	"#00cc00",
	"#00b2ff",
	"#ff7100",
	"#a300ba",
	"#e8a200",
	"#231fd3",
	"#d37caa",
}

type lanInputChar struct {
	value  rune
	hidden bool
}

type lanInputBuffer struct {
	chars []lanInputChar
}

func normalizeLobbyMode(mode LobbyMode) LobbyMode {
	if mode == LobbyModeLanParty {
		return LobbyModeLanParty
	}
	return LobbyModeClassic
}

func (lobby *Lobby) initializeLanPartyPlayers() {
	lobby.LobbyMode = normalizeLobbyMode(lobby.LobbyMode)
	if lobby.LobbyMode != LobbyModeLanParty {
		return
	}

	if lobby.LanPlayerCount < 2 {
		lobby.LanPlayerCount = lobby.MaxPlayers
	}
	if lobby.LanKeyboardCount < 1 {
		lobby.LanKeyboardCount = lobby.LanPlayerCount
	}
	if lobby.MaxPlayers < lobby.LanPlayerCount {
		lobby.MaxPlayers = lobby.LanPlayerCount
	}
	lobby.lanInputBuffers = make(map[uuid.UUID]*lanInputBuffer, lobby.LanPlayerCount)
	lobby.lanKnownKeyboards = make(map[string]time.Time)
	if lobby.LanControlToken == "" {
		lobby.LanControlToken = uuid.Must(uuid.NewV4()).String()
	}

	for index, player := range lobby.players {
		player.LanVirtual = true
		player.LanColor = lanColorForIndex(index)
		player.Connected = true
		if strings.TrimSpace(player.Name) == "" {
			player.Name = lanPlayerName(index)
		}
	}

	for len(lobby.players) < lobby.LanPlayerCount {
		index := len(lobby.players)
		player := &Player{
			Name:              lanPlayerName(index),
			ID:                uuid.Must(uuid.NewV4()),
			userSession:       uuid.Must(uuid.NewV4()),
			votedForKick:      make(map[uuid.UUID]bool),
			messageTimestamps: NewRing[time.Time](5),
			State:             Standby,
			Connected:         true,
			LanVirtual:        true,
			LanColor:          lanColorForIndex(index),
		}
		lobby.players = append(lobby.players, player)
	}
}

func lanPlayerName(index int) string {
	return fmt.Sprintf("Player %d", index+1)
}

func lanColorForIndex(index int) string {
	return lanPlayerColors[index%len(lanPlayerColors)]
}

func (lobby *Lobby) canUseLanController(player *Player) bool {
	return lobby.LobbyMode == LobbyModeLanParty &&
		player != nil &&
		(player.hasLanTerminalRole(LanTerminalRoleDrawing) ||
			player.hasLanTerminalRole(LanTerminalRoleGuessing) ||
			player.ID == lobby.OwnerID)
}

func (lobby *Lobby) canDrawAsTerminal(player *Player) bool {
	return lobby.LobbyMode == LobbyModeLanParty &&
		player != nil &&
		player.hasLanTerminalRole(LanTerminalRoleDrawing) &&
		lobby.Drawer() != nil &&
		lobby.State == Ongoing
}

func (lobby *Lobby) setLanTerminalRole(player *Player, role LanTerminalRole) {
	if lobby.LobbyMode != LobbyModeLanParty || player == nil {
		return
	}
	switch role {
	case LanTerminalRoleDrawing, LanTerminalRoleGuessing:
		if player.LanTerminalRoles == nil {
			player.LanTerminalRoles = make(map[LanTerminalRole]bool)
		}
		player.LanTerminalRoles[role] = true
		player.LanTerminalRole = role
	default:
		player.LanTerminalRole = LanTerminalRoleNone
	}
	lobby.writeObjectToLanRole(player, role, Event{Type: EventTypeReady, Data: generateReadyDataForLanRole(lobby, player, role)})
	if role == LanTerminalRoleDrawing && lobby.State == Ongoing && lobby.CurrentWord == "" {
		lobby.SendYourTurnEventToRole(player, LanTerminalRoleDrawing)
	}
	if role == LanTerminalRoleGuessing {
		lobby.writeObjectToLanRole(player, LanTerminalRoleGuessing, Event{Type: EventTypeLanInputState, Data: lobby.LanInputState()})
	}
}

func (player *Player) hasLanTerminalRole(role LanTerminalRole) bool {
	return player.LanTerminalRole == role || player.LanTerminalRoles[role]
}

func (lobby *Lobby) resetLanInputBuffers() {
	if lobby.LobbyMode != LobbyModeLanParty {
		return
	}
	lobby.lanInputGeneration++
	lobby.lanInputBuffers = make(map[uuid.UUID]*lanInputBuffer, len(lobby.players))
}

func (lobby *Lobby) LanInputState() *LanInputStateEvent {
	rows := make([]*LanInputRow, 0, len(lobby.players))
	drawer := lobby.Drawer()
	for _, player := range lobby.players {
		if player.State == Spectating {
			continue
		}
		buffer := lobby.lanBufferForPlayer(player.ID)
		isDrawer := drawer != nil && drawer.ID == player.ID
		disabledReason := ""
		locked := player.State != Guessing || lobby.CurrentWord == ""
		if lobby.State != Ongoing {
			locked = player.State != Standby
			if player.State == Ready {
				disabledReason = "ready"
			} else if player.State == Spectating {
				disabledReason = string(player.State)
			}
		} else if isDrawer {
			disabledReason = "drawing"
		} else if lobby.CurrentWord == "" {
			disabledReason = "waiting"
		} else if player.State != Guessing {
			disabledReason = string(player.State)
		}
		row := &LanInputRow{
			PlayerID:       player.ID,
			PlayerName:     player.Name,
			Color:          player.LanColor,
			KeyboardID:     player.LanKeyboardID,
			MaskedText:     buffer.maskedText(),
			DraftName:      lobby.lanDraftNameForPlayer(player, buffer),
			Locked:         locked,
			DisabledReason: disabledReason,
			Drawing:        isDrawer,
		}
		rows = append(rows, row)
	}
	return &LanInputStateEvent{Rows: rows, KnownKeyboards: lobby.knownLanKeyboards()}
}

func (lobby *Lobby) lanDraftNameForPlayer(player *Player, buffer *lanInputBuffer) string {
	if lobby.State == Ongoing || player.State != Standby {
		return ""
	}
	return buffer.plainText()
}

func (lobby *Lobby) writeObjectToLanRole(player *Player, role LanTerminalRole, data any) {
	if lobby.WriteObjectToRole != nil {
		_ = lobby.WriteObjectToRole(player, role, data)
		return
	}
	_ = lobby.WriteObject(player, data)
}

func (lobby *Lobby) lanBufferForPlayer(playerID uuid.UUID) *lanInputBuffer {
	if lobby.lanInputBuffers == nil {
		lobby.lanInputBuffers = make(map[uuid.UUID]*lanInputBuffer)
	}
	buffer := lobby.lanInputBuffers[playerID]
	if buffer == nil {
		buffer = &lanInputBuffer{}
		lobby.lanInputBuffers[playerID] = buffer
	}
	return buffer
}

func (buffer *lanInputBuffer) maskedText() string {
	var builder strings.Builder
	for _, char := range buffer.chars {
		if char.hidden {
			builder.WriteRune('*')
		} else {
			builder.WriteRune(char.value)
		}
	}
	return builder.String()
}

func (buffer *lanInputBuffer) plainText() string {
	var builder strings.Builder
	for _, char := range buffer.chars {
		builder.WriteRune(char.value)
	}
	return builder.String()
}

func (lobby *Lobby) HandleLanKeyboardInput(input LanKeyboardInput) {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	lobby.handleLanKeyboardInputUnsynchronized(input)
}

func (lobby *Lobby) handleLanKeyboardInputUnsynchronized(input LanKeyboardInput) {
	if lobby.LobbyMode != LobbyModeLanParty || input.KeyboardID == "" || input.Action == "keyup" {
		return
	}
	lobby.recordLanKeyboardUnsynchronized(input.KeyboardID)
	player := lobby.getLanPlayerByKeyboard(input.KeyboardID)
	if player == nil && lobby.State != Ongoing {
		player = lobby.assignNextLanSetupKeyboardUnsynchronized(input.KeyboardID)
	}
	if lobby.State != Ongoing {
		lobby.handleLanSetupKeyboardInputUnsynchronized(player, input)
		return
	}
	if player == nil || player.State != Guessing || lobby.CurrentWord == "" {
		lobby.broadcastLanInputState()
		return
	}

	buffer := lobby.lanBufferForPlayer(player.ID)
	switch input.Key {
	case "Enter":
		message := buffer.plainText()
		buffer.chars = nil
		lobby.broadcastLanInputState()
		handleMessage(message, player, lobby)
		lobby.broadcastLanInputState()
	case "Backspace":
		if len(buffer.chars) > 0 {
			buffer.chars = buffer.chars[:len(buffer.chars)-1]
			lobby.broadcastLanInputState()
		}
	default:
		runes := []rune(input.Key)
		if len(runes) != 1 {
			return
		}
		buffer.chars = append(buffer.chars, lanInputChar{value: runes[0]})
		charIndex := len(buffer.chars) - 1
		generation := lobby.lanInputGeneration
		lobby.broadcastLanInputState()
		go lobby.hideLanInputCharLater(player.ID, charIndex, generation)
	}
}

func (lobby *Lobby) assignNextLanSetupKeyboardUnsynchronized(keyboardID string) *Player {
	for _, player := range lobby.players {
		if player.State == Spectating || player.LanKeyboardID != "" {
			continue
		}
		player.LanKeyboardID = keyboardID
		return player
	}
	return nil
}

func (lobby *Lobby) handleLanSetupKeyboardInputUnsynchronized(player *Player, input LanKeyboardInput) {
	if player == nil || player.State != Standby {
		lobby.broadcastLanInputState()
		return
	}

	buffer := lobby.lanBufferForPlayer(player.ID)
	switch input.Key {
	case "Enter":
		name := strings.TrimSpace(buffer.plainText())
		buffer.chars = nil
		if name != "" {
			handleNameChangeEvent(player, lobby, name)
		}
		player.State = Ready
		if lobby.readyToStart() {
			lobby.startGame()
			return
		}
		lobby.Broadcast(&Event{Type: EventTypeUpdatePlayers, Data: lobby.players})
		lobby.broadcastLanInputState()
	case "Backspace":
		if len(buffer.chars) > 0 {
			buffer.chars = buffer.chars[:len(buffer.chars)-1]
		}
		lobby.broadcastLanInputState()
	default:
		runes := []rune(input.Key)
		if len(runes) != 1 || utf8.RuneCountInString(buffer.plainText()) >= MaxPlayerNameLength {
			lobby.broadcastLanInputState()
			return
		}
		buffer.chars = append(buffer.chars, lanInputChar{value: runes[0]})
		charIndex := len(buffer.chars) - 1
		generation := lobby.lanInputGeneration
		lobby.broadcastLanInputState()
		go lobby.hideLanInputCharLater(player.ID, charIndex, generation)
	}
}

func (lobby *Lobby) recordLanKeyboardUnsynchronized(keyboardID string) {
	if lobby.lanKnownKeyboards == nil {
		lobby.lanKnownKeyboards = make(map[string]time.Time)
	}
	lobby.lanKnownKeyboards[keyboardID] = time.Now()
}

func (lobby *Lobby) knownLanKeyboards() []string {
	keyboards := make([]string, 0, len(lobby.lanKnownKeyboards))
	for keyboardID := range lobby.lanKnownKeyboards {
		keyboards = append(keyboards, keyboardID)
	}
	for _, player := range lobby.players {
		if player.LanKeyboardID != "" {
			keyboards = append(keyboards, player.LanKeyboardID)
		}
	}
	sort.Strings(keyboards)
	return compactSortedStrings(keyboards)
}

func compactSortedStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	writeIndex := 1
	for readIndex := 1; readIndex < len(values); readIndex++ {
		if values[readIndex] != values[readIndex-1] {
			values[writeIndex] = values[readIndex]
			writeIndex++
		}
	}
	return values[:writeIndex]
}

func (lobby *Lobby) hideLanInputCharLater(playerID uuid.UUID, charIndex int, generation int64) {
	time.Sleep(lanInputRevealDuration)
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	if generation != lobby.lanInputGeneration {
		return
	}
	buffer := lobby.lanBufferForPlayer(playerID)
	if charIndex >= len(buffer.chars) {
		return
	}
	buffer.chars[charIndex].hidden = true
	lobby.broadcastLanInputState()
}

func (lobby *Lobby) getLanPlayerByKeyboard(keyboardID string) *Player {
	for _, player := range lobby.players {
		if player.LanKeyboardID == keyboardID {
			return player
		}
	}
	return nil
}

func (lobby *Lobby) AssignLanKeyboard(playerID uuid.UUID, keyboardID string) bool {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	if lobby.LobbyMode != LobbyModeLanParty {
		return false
	}
	player := lobby.GetPlayerByID(playerID)
	if player == nil {
		return false
	}
	if keyboardID == "" {
		player.LanKeyboardID = ""
		lobby.Broadcast(&Event{Type: EventTypeLanAssignmentUpdate, Data: lobby.LanInputState()})
		return true
	}
	for _, otherPlayer := range lobby.players {
		if otherPlayer.ID != playerID && otherPlayer.LanKeyboardID == keyboardID {
			otherPlayer.LanKeyboardID = ""
		}
	}
	player.LanKeyboardID = keyboardID
	lobby.Broadcast(&Event{Type: EventTypeLanAssignmentUpdate, Data: lobby.LanInputState()})
	return true
}

func (lobby *Lobby) broadcastLanInputState() {
	if lobby.LobbyMode == LobbyModeLanParty {
		lobby.Broadcast(&Event{Type: EventTypeLanInputState, Data: lobby.LanInputState()})
	}
}
