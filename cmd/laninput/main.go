package main

import (
	"flag"
	"log"
	"strings"
)

func main() {
	server := flag.String("server", "http://localhost:8080", "Scribble.rs server URL")
	lobbyID := flag.String("lobby", "", "Lobby ID")
	token := flag.String("token", "", "LAN control token shown in the lobby setup dialog")
	userSession := flag.String("usersession", "", "Owner usersession cookie value; fallback for development")
	stdinMode := flag.Bool("stdin", false, "read newline-delimited keyboard JSON from stdin instead of native capture")
	listOnly := flag.Bool("list", false, "list keyboard devices and exit")
	flag.Parse()

	if !*listOnly && (*lobbyID == "" || (*token == "" && *userSession == "")) {
		log.Fatal("-lobby and either -token or -usersession are required")
	}
	if *token != "" && looksLikeJWT(*token) {
		log.Fatal("-token must be the LAN control token from the LAN Setup dialog, not a browser auth/session token")
	}

	endpoint := strings.TrimRight(*server, "/") + "/v1/lobby/" + *lobbyID + "/lan/input"
	if *stdinMode {
		runStdinBridge(endpoint, *token, *userSession)
		return
	}

	if err := runNativeCapture(endpoint, *token, *userSession, *listOnly); err != nil {
		log.Fatal(err)
	}
}

func looksLikeJWT(value string) bool {
	parts := strings.Split(value, ".")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}
