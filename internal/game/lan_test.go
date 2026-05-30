package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/lxzan/gws"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Test_initializeLanPartyPlayers(t *testing.T) {
	t.Parallel()

	_, lobby, err := CreateLobby("", "Host", "english", &EditableLobbySettings{
		MaxPlayers:       5,
		LobbyMode:        LobbyModeLanParty,
		LanPlayerCount:   5,
		LanKeyboardCount: 5,
	}, nil, ChillScoring)
	if err != nil {
		t.Fatalf("CreateLobby() error = %v", err)
	}

	if len(lobby.players) != 5 {
		t.Fatalf("LAN lobby player count = %d, want 5", len(lobby.players))
	}
	for _, player := range lobby.players {
		if !player.LanVirtual {
			t.Fatalf("player %s was not marked LAN virtual", player.Name)
		}
		if player.LanColor == "" {
			t.Fatalf("player %s has no LAN color", player.Name)
		}
	}
}

func Test_handleLanKeyboardInputSubmitsGuessForAssignedPlayer(t *testing.T) {
	t.Parallel()

	playerID := uuid.Must(uuid.NewV4())
	otherPlayerID := uuid.Must(uuid.NewV4())
	player := &Player{
		ID:                playerID,
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-1",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	otherPlayer := &Player{
		ID:                otherPlayerID,
		Name:              "Other",
		State:             Guessing,
		Connected:         true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := &Lobby{
		EditableLobbySettings: EditableLobbySettings{LobbyMode: LobbyModeLanParty, DrawingTime: 120},
		players:               []*Player{player, otherPlayer},
		State:                 Ongoing,
		CurrentWord:           "cat",
		ScoreCalculation:      ChillScoring,
		roundEndTime:          time.Now().Add(time.Minute).UnixMilli(),
		lowercaser:            cases.Lower(language.English),
		WriteObject:           noOpWriteObject,
		WritePreparedMessage:  noOpWritePreparedMessage,
	}
	lobby.resetLanInputBuffers()

	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-1", Key: "c", Action: "keydown"})
	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-1", Key: "a", Action: "keydown"})
	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-1", Key: "t", Action: "keydown"})
	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-1", Key: "Enter", Action: "keydown"})

	if player.State != Standby {
		t.Fatalf("player state = %s, want %s", player.State, Standby)
	}
	if player.Score == 0 {
		t.Fatal("expected submitted LAN guess to award score")
	}
	if got := lobby.lanBufferForPlayer(playerID).plainText(); got != "" {
		t.Fatalf("buffer after submit = %q, want empty", got)
	}
}

func Test_handleLanKeyboardInputBroadcastsIncorrectGuessAsChatMessage(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-1",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	other := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Other",
		State:             Guessing,
		Connected:         true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(player, other)

	var chatMessages []OutgoingMessage
	lobby.WritePreparedMessage = func(_ *Player, message *gws.Broadcaster) error {
		data := getUnexportedField(reflect.ValueOf(message).Elem().FieldByName("payload")).([]byte)
		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == EventTypeMessage {
			var outgoing OutgoingMessage
			if err := json.Unmarshal(event.Data, &outgoing); err != nil {
				t.Fatal(err)
			}
			chatMessages = append(chatMessages, outgoing)
		}
		return nil
	}

	for _, input := range []LanKeyboardInput{
		{KeyboardID: "kbd-1", Key: "d", Action: "keydown"},
		{KeyboardID: "kbd-1", Key: "o", Action: "keydown"},
		{KeyboardID: "kbd-1", Key: "g", Action: "keydown"},
		{KeyboardID: "kbd-1", Key: "Enter", Action: "keydown"},
	} {
		lobby.handleLanKeyboardInputUnsynchronized(input)
	}

	if len(chatMessages) != 2 {
		t.Fatalf("chat message broadcast count = %d, want 2 player deliveries", len(chatMessages))
	}
	if chatMessages[0].Author != "Guesser" || chatMessages[0].Content != "dog" {
		t.Fatalf("chat message = %#v, want Guesser dog", chatMessages[0])
	}
}

func Test_handleLanKeyboardInputBroadcastsRapidIncorrectLanGuesses(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-1",
		LanVirtual:        true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	other := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Other",
		State:             Guessing,
		Connected:         true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(player, other)

	var chatMessages []OutgoingMessage
	lobby.WritePreparedMessage = func(_ *Player, message *gws.Broadcaster) error {
		data := getUnexportedField(reflect.ValueOf(message).Elem().FieldByName("payload")).([]byte)
		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == EventTypeMessage {
			var outgoing OutgoingMessage
			if err := json.Unmarshal(event.Data, &outgoing); err != nil {
				t.Fatal(err)
			}
			chatMessages = append(chatMessages, outgoing)
		}
		return nil
	}

	for range 6 {
		for _, input := range []LanKeyboardInput{
			{KeyboardID: "kbd-1", Key: "d", Action: "keydown"},
			{KeyboardID: "kbd-1", Key: "o", Action: "keydown"},
			{KeyboardID: "kbd-1", Key: "g", Action: "keydown"},
			{KeyboardID: "kbd-1", Key: "Enter", Action: "keydown"},
		} {
			lobby.handleLanKeyboardInputUnsynchronized(input)
		}
	}

	if len(chatMessages) != 12 {
		t.Fatalf("chat message broadcast count = %d, want 12 player deliveries", len(chatMessages))
	}
}

func Test_handleLanKeyboardInputSendsCloseGuessToGuessingTerminal(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-1",
		LanVirtual:        true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	terminal := &Player{
		ID:               uuid.Must(uuid.NewV4()),
		Name:             "Terminal",
		State:            Guessing,
		Connected:        true,
		LanTerminalRoles: map[LanTerminalRole]bool{LanTerminalRoleGuessing: true},
	}
	lobby := newLanTestLobby(player, terminal)

	var closeGuessEvents []OutgoingMessage
	lobby.WriteObjectToRole = func(_ *Player, role LanTerminalRole, data any) error {
		if role != LanTerminalRoleGuessing {
			return nil
		}
		event, ok := data.(*Event)
		if !ok || event.Type != EventTypeCloseGuess {
			return nil
		}
		closeGuess, ok := event.Data.(OutgoingMessage)
		if !ok {
			t.Fatalf("close guess data type = %T, want OutgoingMessage", event.Data)
		}
		closeGuessEvents = append(closeGuessEvents, closeGuess)
		return nil
	}

	for _, input := range []LanKeyboardInput{
		{KeyboardID: "kbd-1", Key: "c", Action: "keydown"},
		{KeyboardID: "kbd-1", Key: "a", Action: "keydown"},
		{KeyboardID: "kbd-1", Key: "r", Action: "keydown"},
		{KeyboardID: "kbd-1", Key: "Enter", Action: "keydown"},
	} {
		lobby.handleLanKeyboardInputUnsynchronized(input)
	}

	if len(closeGuessEvents) != 1 {
		t.Fatalf("close guess event count = %d, want 1", len(closeGuessEvents))
	}
	if closeGuessEvents[0].Author != "Guesser" || closeGuessEvents[0].Content != "car" {
		t.Fatalf("close guess event = %#v, want Guesser car", closeGuessEvents[0])
	}
}

func Test_handleLanKeyboardInputIgnoresCurrentDrawerKeyboard(t *testing.T) {
	t.Parallel()

	drawerID := uuid.Must(uuid.NewV4())
	drawer := &Player{
		ID:                drawerID,
		Name:              "Drawer",
		State:             Drawing,
		Connected:         true,
		LanKeyboardID:     "kbd-drawer",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	guesser := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Drawing,
		Connected:         true,
		LanKeyboardID:     "kbd-guesser",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := &Lobby{
		EditableLobbySettings: EditableLobbySettings{LobbyMode: LobbyModeLanParty, DrawingTime: 120},
		players:               []*Player{drawer, guesser},
		State:                 Ongoing,
		CurrentWord:           "cat",
		ScoreCalculation:      ChillScoring,
		roundEndTime:          time.Now().Add(time.Minute).UnixMilli(),
		lowercaser:            cases.Lower(language.English),
		WriteObject:           noOpWriteObject,
		WritePreparedMessage:  noOpWritePreparedMessage,
	}
	lobby.resetLanInputBuffers()

	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-drawer", Key: "c", Action: "keydown"})
	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-drawer", Key: "a", Action: "keydown"})
	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-drawer", Key: "t", Action: "keydown"})
	lobby.handleLanKeyboardInputUnsynchronized(LanKeyboardInput{KeyboardID: "kbd-drawer", Key: "Enter", Action: "keydown"})

	if got := lobby.lanBufferForPlayer(drawerID).plainText(); got != "" {
		t.Fatalf("drawer keyboard buffer = %q, want empty", got)
	}
	if drawer.State != Drawing {
		t.Fatalf("drawer state = %s, want %s", drawer.State, Drawing)
	}

	var state *LanInputStateEvent
	lobby.Synchronized(func() {
		state = lobby.LanInputState()
	})
	if len(state.Rows) == 0 || state.Rows[0].DisabledReason != "drawing" {
		t.Fatalf("drawer disabled reason = %#v, want drawing", state.Rows)
	}
}

func Test_AssignLanKeyboardMovesDuplicateAssignment(t *testing.T) {
	t.Parallel()

	playerA := &Player{ID: uuid.Must(uuid.NewV4()), Name: "A", State: Guessing, Connected: true}
	playerB := &Player{ID: uuid.Must(uuid.NewV4()), Name: "B", State: Guessing, Connected: true}
	lobby := newLanTestLobby(playerA, playerB)

	if !lobby.AssignLanKeyboard(playerA.ID, "kbd-shared") {
		t.Fatal("expected first keyboard assignment to succeed")
	}
	if !lobby.AssignLanKeyboard(playerB.ID, "kbd-shared") {
		t.Fatal("expected reassignment to second player to succeed")
	}

	if playerA.LanKeyboardID != "" {
		t.Fatalf("player A keyboard = %q, want cleared after reassignment", playerA.LanKeyboardID)
	}
	if playerB.LanKeyboardID != "kbd-shared" {
		t.Fatalf("player B keyboard = %q, want kbd-shared", playerB.LanKeyboardID)
	}
}

func Test_AssignLanKeyboardRejectsUnknownPlayerAndAllowsClearing(t *testing.T) {
	t.Parallel()

	player := &Player{ID: uuid.Must(uuid.NewV4()), Name: "A", State: Guessing, Connected: true, LanKeyboardID: "kbd-original"}
	lobby := newLanTestLobby(player)

	if lobby.AssignLanKeyboard(uuid.Must(uuid.NewV4()), "kbd-new") {
		t.Fatal("expected unknown player assignment to fail")
	}
	if player.LanKeyboardID != "kbd-original" {
		t.Fatalf("player keyboard mutated to %q", player.LanKeyboardID)
	}
	if !lobby.AssignLanKeyboard(player.ID, "") {
		t.Fatal("expected empty keyboard assignment to clear existing assignment")
	}
	if player.LanKeyboardID != "" {
		t.Fatalf("player keyboard = %q, want cleared", player.LanKeyboardID)
	}
}

func Test_HandleLanKeyboardInputRecordsUnassignedKeyboard(t *testing.T) {
	t.Parallel()

	player := &Player{ID: uuid.Must(uuid.NewV4()), Name: "A", State: Guessing, Connected: true}
	lobby := newLanTestLobby(player)

	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-new", Key: "x", Action: "keydown"})

	var state *LanInputStateEvent
	lobby.Synchronized(func() {
		state = lobby.LanInputState()
	})
	if len(state.KnownKeyboards) != 1 || state.KnownKeyboards[0] != "kbd-new" {
		t.Fatalf("known keyboards = %#v, want [kbd-new]", state.KnownKeyboards)
	}
	if got := lobby.lanBufferForPlayer(player.ID).plainText(); got != "" {
		t.Fatalf("unassigned keyboard changed player buffer to %q", got)
	}
}

func Test_handleLanKeyboardInputSetupAutoAssignsRenamesAndReadiesPlayer(t *testing.T) {
	t.Parallel()

	playerA := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 1", State: Standby, Connected: true}
	playerB := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 2", State: Standby, Connected: true}
	playerC := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 3", State: Standby, Connected: true}
	lobby := newLanSetupTestLobby(playerA, playerB, playerC)

	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "A", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "l", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "i", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "x", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "Backspace", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "c", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "e", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "Enter", Action: "keydown"})

	if playerA.LanKeyboardID != "kbd-a" {
		t.Fatalf("player A keyboard = %q, want kbd-a", playerA.LanKeyboardID)
	}
	if playerA.Name != "Alice" {
		t.Fatalf("player A name = %q, want Alice", playerA.Name)
	}
	if playerA.State != Ready {
		t.Fatalf("player A state = %s, want %s", playerA.State, Ready)
	}
	if got := lobby.lanBufferForPlayer(playerA.ID).plainText(); got != "" {
		t.Fatalf("player A setup buffer = %q, want empty", got)
	}
	if playerB.LanKeyboardID != "" {
		t.Fatalf("player B keyboard = %q, want unassigned", playerB.LanKeyboardID)
	}
}

func Test_LanInputStateSetupExposesUnmaskedDraftNames(t *testing.T) {
	t.Parallel()

	playerA := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 1", State: Standby, Connected: true}
	playerB := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 2", State: Standby, Connected: true}
	lobby := newLanSetupTestLobby(playerA, playerB)

	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "A", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "l", Action: "keydown"})
	time.Sleep(lanInputRevealDuration + 25*time.Millisecond)

	var state *LanInputStateEvent
	lobby.Synchronized(func() {
		state = lobby.LanInputState()
	})
	if got := state.Rows[0].DraftName; got != "Al" {
		t.Fatalf("draft name = %q, want Al", got)
	}
	if got := state.Rows[0].MaskedText; got != "**" {
		t.Fatalf("masked text = %q, want **", got)
	}
}

func Test_handleLanKeyboardInputSetupKeepsSimultaneousKeyboardNamesSeparate(t *testing.T) {
	t.Parallel()

	playerA := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 1", State: Standby, Connected: true}
	playerB := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 2", State: Standby, Connected: true}
	playerC := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Player 3", State: Standby, Connected: true}
	lobby := newLanSetupTestLobby(playerA, playerB, playerC)

	events := []LanKeyboardInput{
		{KeyboardID: "kbd-a", Key: "A", Action: "keydown"},
		{KeyboardID: "kbd-b", Key: "B", Action: "keydown"},
		{KeyboardID: "kbd-a", Key: "n", Action: "keydown"},
		{KeyboardID: "kbd-b", Key: "o", Action: "keydown"},
		{KeyboardID: "kbd-a", Key: "n", Action: "keydown"},
		{KeyboardID: "kbd-b", Key: "b", Action: "keydown"},
		{KeyboardID: "kbd-b", Key: "Enter", Action: "keydown"},
		{KeyboardID: "kbd-a", Key: "Enter", Action: "keydown"},
	}
	for _, event := range events {
		lobby.HandleLanKeyboardInput(event)
	}

	if playerA.LanKeyboardID != "kbd-a" || playerA.Name != "Ann" || playerA.State != Ready {
		t.Fatalf("player A = keyboard %q name %q state %s, want kbd-a Ann ready", playerA.LanKeyboardID, playerA.Name, playerA.State)
	}
	if playerB.LanKeyboardID != "kbd-b" || playerB.Name != "Bob" || playerB.State != Ready {
		t.Fatalf("player B = keyboard %q name %q state %s, want kbd-b Bob ready", playerB.LanKeyboardID, playerB.Name, playerB.State)
	}
}

func Test_generateReadyDataIncludesLanControlTokenOnlyForOwner(t *testing.T) {
	t.Parallel()

	owner := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Owner", State: Standby, Connected: true}
	other := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Other", State: Standby, Connected: true}
	lobby := newLanTestLobby(owner, other)
	lobby.OwnerID = owner.ID
	lobby.LanControlToken = "secret-token"

	if got := generateReadyData(lobby, owner).LanControlToken; got != "secret-token" {
		t.Fatalf("owner LAN control token = %q, want secret-token", got)
	}
	if got := generateReadyData(lobby, other).LanControlToken; got != "" {
		t.Fatalf("non-owner LAN control token = %q, want empty", got)
	}
}

func Test_generateReadyDataForLanRoleSeparatesDrawingAndGuessingHints(t *testing.T) {
	t.Parallel()

	drawer := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Drawer", State: Drawing, Connected: true}
	lobby := newLanTestLobby(drawer)
	lobby.wordHints = []*WordHint{{Underline: true}, {Underline: true}}
	lobby.wordHintsShown = []*WordHint{{Character: 'o', Underline: true}, {Character: 'x', Underline: true}}

	guessingReady := generateReadyDataForLanRole(lobby, drawer, LanTerminalRoleGuessing)
	if guessingReady.WordHints[0].Character != 0 || guessingReady.AllowDrawing {
		t.Fatalf("guessing terminal got revealed/drawing data: %#v allow=%t", guessingReady.WordHints, guessingReady.AllowDrawing)
	}

	drawingReady := generateReadyDataForLanRole(lobby, drawer, LanTerminalRoleDrawing)
	if drawingReady.WordHints[0].Character != 'o' || !drawingReady.AllowDrawing {
		t.Fatalf("drawing terminal got hidden/non-drawing data: %#v allow=%t", drawingReady.WordHints, drawingReady.AllowDrawing)
	}
}

func Test_broadcastWordChosenSendsRoleSpecificHintsToSamePlayerTerminals(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:               uuid.Must(uuid.NewV4()),
		Name:             "Owner",
		State:            Drawing,
		Connected:        true,
		LanTerminalRoles: map[LanTerminalRole]bool{LanTerminalRoleDrawing: true, LanTerminalRoleGuessing: true},
	}
	lobby := newLanTestLobby(player)
	lobby.wordHints = []*WordHint{{Underline: true}}
	lobby.wordHintsShown = []*WordHint{{Character: 'x', Underline: true}}

	eventsByRole := map[LanTerminalRole]*Event{}
	lobby.WriteObjectToRole = func(_ *Player, role LanTerminalRole, data any) error {
		event, ok := data.(*Event)
		if !ok {
			t.Fatalf("data type = %T, want *Event", data)
		}
		eventsByRole[role] = event
		return nil
	}

	lobby.broadcastWordChosen(1000)

	guessing := eventsByRole[LanTerminalRoleGuessing].Data.(*WordChosen)
	if guessing.Hints[0].Character != 0 {
		t.Fatalf("guessing hint character = %q, want hidden", guessing.Hints[0].Character)
	}
	drawing := eventsByRole[LanTerminalRoleDrawing].Data.(*WordChosen)
	if drawing.Hints[0].Character != 'x' {
		t.Fatalf("drawing hint character = %q, want x", drawing.Hints[0].Character)
	}
}

func Test_LanPartyDrawingEventsAreEchoedToSamePlayerGuessingTerminal(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Owner",
		State:             Drawing,
		Connected:         true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
		LanTerminalRoles:  map[LanTerminalRole]bool{LanTerminalRoleDrawing: true, LanTerminalRoleGuessing: true},
	}
	lobby := newLanTestLobby(player)

	writeCount := 0
	lobby.WritePreparedMessage = func(_ *Player, _ *gws.Broadcaster) error {
		writeCount++
		return nil
	}

	err := lobby.HandleEvent(EventTypeLine, []byte(`{"data":{"x":1,"y":2,"x2":3,"y2":4,"color":0,"width":8}}`), player)
	if err != nil {
		t.Fatalf("line event failed: %v", err)
	}
	if writeCount != 1 {
		t.Fatalf("LAN drawing broadcast count = %d, want 1", writeCount)
	}
}

func Test_LanPartyStartWaitsForDrawingTerminalHintConfirmation(t *testing.T) {
	t.Parallel()

	owner := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Owner", State: Ready, Connected: true}
	other := &Player{ID: uuid.Must(uuid.NewV4()), Name: "Other", State: Ready, Connected: true}
	lobby := newLanSetupTestLobby(owner, other)
	lobby.OwnerID = owner.ID
	lobby.words = []string{"cat", "dog", "tree"}
	lobby.WordsPerTurn = 2
	lobby.Rounds = 1

	sawStartPending := false
	lobby.WritePreparedMessage = func(_ *Player, message *gws.Broadcaster) error {
		data := getUnexportedField(reflect.ValueOf(message).Elem().FieldByName("payload")).([]byte)
		var event EventTypeOnly
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == EventTypeLanStartPending {
			sawStartPending = true
		}
		if event.Type == EventTypeNextTurn {
			t.Fatal("next turn was sent before LAN start confirmation")
		}
		return nil
	}

	lobby.startGame()
	if !sawStartPending {
		t.Fatal("expected LAN start-pending event")
	}
	if !lobby.lanStartPending || lobby.State != Unstarted {
		t.Fatalf("pending=%t state=%s, want pending unstarted", lobby.lanStartPending, lobby.State)
	}

	lobby.WritePreparedMessage = noOpWritePreparedMessage
	if err := lobby.HandleEvent(EventTypeLanStartConfirm, nil, owner); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if lobby.lanStartPending || lobby.State != Ongoing {
		t.Fatalf("pending=%t state=%s, want started ongoing", lobby.lanStartPending, lobby.State)
	}
}

func Test_selectWordBroadcastsLanInputStateToEnableGuessers(t *testing.T) {
	t.Parallel()

	drawer := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Drawer",
		State:             Drawing,
		Connected:         true,
		LanKeyboardID:     "kbd-drawer",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	guesser := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-guesser",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(drawer, guesser)
	lobby.CurrentWord = ""
	lobby.wordChoice = []string{"cat"}
	lobby.preSelectedWord = 0
	lobby.DrawingTime = 120

	var lanStates []*LanInputStateEvent
	lobby.WritePreparedMessage = func(_ *Player, message *gws.Broadcaster) error {
		data := getUnexportedField(reflect.ValueOf(message).Elem().FieldByName("payload")).([]byte)
		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == EventTypeLanInputState {
			var state LanInputStateEvent
			if err := json.Unmarshal(event.Data, &state); err != nil {
				t.Fatal(err)
			}
			lanStates = append(lanStates, &state)
		}
		return nil
	}

	if err := lobby.selectWord(0); err != nil {
		t.Fatalf("select word failed: %v", err)
	}
	if len(lanStates) == 0 {
		t.Fatal("expected LAN input state after word selection")
	}
	last := lanStates[len(lanStates)-1]
	var guesserRow *LanInputRow
	for _, row := range last.Rows {
		if row.PlayerID == guesser.ID {
			guesserRow = row
			break
		}
	}
	if guesserRow == nil {
		t.Fatal("guesser row missing")
	}
	if guesserRow.Locked || guesserRow.DisabledReason != "" {
		t.Fatalf("guesser row locked=%t disabled=%q, want enabled", guesserRow.Locked, guesserRow.DisabledReason)
	}
}

func Test_DrawingTerminalCanChooseWordForLaterLanDrawer(t *testing.T) {
	t.Parallel()

	terminalOwner := &Player{
		ID:               uuid.Must(uuid.NewV4()),
		Name:             "Owner",
		State:            Guessing,
		Connected:        true,
		LanTerminalRoles: map[LanTerminalRole]bool{LanTerminalRoleDrawing: true},
	}
	drawer := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Second Drawer",
		State:             Drawing,
		Connected:         true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(terminalOwner, drawer)
	lobby.CurrentWord = ""
	lobby.wordChoice = []string{"cat", "dog"}
	lobby.preSelectedWord = 0
	lobby.DrawingTime = 120

	if err := lobby.HandleEvent(EventTypeChooseWord, []byte(`{"data":1}`), terminalOwner); err != nil {
		t.Fatalf("choose word failed: %v", err)
	}
	if lobby.CurrentWord != "dog" {
		t.Fatalf("current word = %q, want dog", lobby.CurrentWord)
	}
}

func Test_resetLanInputBuffersIgnoresOldMaskTimers(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-1",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(player)

	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-1", Key: "x", Action: "keydown"})
	lobby.Synchronized(func() {
		lobby.resetLanInputBuffers()
		lobby.lanBufferForPlayer(player.ID).chars = append(lobby.lanBufferForPlayer(player.ID).chars, lanInputChar{value: 'y'})
	})
	time.Sleep(lanInputRevealDuration + 25*time.Millisecond)

	var masked string
	lobby.Synchronized(func() {
		masked = lobby.LanInputState().Rows[0].MaskedText
	})
	if masked != "y" {
		t.Fatalf("masked text after stale timer = %q, want y", masked)
	}
}

func Test_handleLanKeyboardInputBackspaceBeforeMaskTimerDoesNotRestoreCharacter(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-1",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(player)

	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-1", Key: "x", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-1", Key: "Backspace", Action: "keydown"})
	time.Sleep(lanInputRevealDuration + 25*time.Millisecond)

	if got := lobby.lanBufferForPlayer(player.ID).plainText(); got != "" {
		t.Fatalf("buffer after backspace and mask timer = %q, want empty", got)
	}
	if got := lobby.LanInputState().Rows[0].MaskedText; got != "" {
		t.Fatalf("masked text after backspace and mask timer = %q, want empty", got)
	}
}

func Test_handleLanKeyboardInputSubmitBeforeMaskTimerDoesNotRecreateBuffer(t *testing.T) {
	t.Parallel()

	player := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Guesser",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-1",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	otherPlayer := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "Other",
		State:             Guessing,
		Connected:         true,
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(player, otherPlayer)

	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-1", Key: "c", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-1", Key: "a", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-1", Key: "t", Action: "keydown"})
	lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-1", Key: "Enter", Action: "keydown"})
	time.Sleep(lanInputRevealDuration + 25*time.Millisecond)

	if player.State != Standby {
		t.Fatalf("player state = %s, want %s", player.State, Standby)
	}
	if got := lobby.lanBufferForPlayer(player.ID).plainText(); got != "" {
		t.Fatalf("buffer after submit and mask timers = %q, want empty", got)
	}
}

func Test_LanInputConcurrentTypingReassignmentAndStateRendering(t *testing.T) {
	t.Parallel()

	playerA := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "A",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-a",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	playerB := &Player{
		ID:                uuid.Must(uuid.NewV4()),
		Name:              "B",
		State:             Guessing,
		Connected:         true,
		LanKeyboardID:     "kbd-b",
		messageTimestamps: NewRing[time.Time](5),
		votedForKick:      make(map[uuid.UUID]bool),
	}
	lobby := newLanTestLobby(playerA, playerB)

	var wg sync.WaitGroup
	for i := range 25 {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "x", Action: "keydown"})
			if i%2 == 0 {
				lobby.HandleLanKeyboardInput(LanKeyboardInput{KeyboardID: "kbd-a", Key: "Backspace", Action: "keydown"})
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			_ = lobby.AssignLanKeyboard(playerB.ID, fmt.Sprintf("kbd-b-%d", i))
			_ = lobby.AssignLanKeyboard(playerB.ID, "kbd-b")
		}(i)
		go func() {
			defer wg.Done()
			lobby.mutex.Lock()
			defer lobby.mutex.Unlock()
			_ = lobby.LanInputState()
		}()
	}
	wg.Wait()
	time.Sleep(lanInputRevealDuration + 25*time.Millisecond)

	if playerA.LanKeyboardID != "kbd-a" {
		t.Fatalf("player A keyboard = %q, want kbd-a", playerA.LanKeyboardID)
	}
	if playerB.LanKeyboardID != "kbd-b" {
		t.Fatalf("player B keyboard = %q, want kbd-b", playerB.LanKeyboardID)
	}
}

func newLanTestLobby(players ...*Player) *Lobby {
	lobby := &Lobby{
		EditableLobbySettings: EditableLobbySettings{LobbyMode: LobbyModeLanParty, DrawingTime: 120},
		players:               players,
		State:                 Ongoing,
		CurrentWord:           "cat",
		ScoreCalculation:      ChillScoring,
		roundEndTime:          time.Now().Add(time.Minute).UnixMilli(),
		lowercaser:            cases.Lower(language.English),
		WriteObject:           noOpWriteObject,
		WritePreparedMessage:  noOpWritePreparedMessage,
	}
	for _, player := range players {
		if player.messageTimestamps == nil {
			player.messageTimestamps = NewRing[time.Time](5)
		}
		if player.votedForKick == nil {
			player.votedForKick = make(map[uuid.UUID]bool)
		}
	}
	lobby.resetLanInputBuffers()
	return lobby
}

func newLanSetupTestLobby(players ...*Player) *Lobby {
	lobby := newLanTestLobby(players...)
	lobby.State = Unstarted
	lobby.CurrentWord = ""
	lobby.roundEndTime = 0
	return lobby
}
