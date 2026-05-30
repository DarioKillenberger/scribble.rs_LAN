//go:build windows

package main

import (
	"github.com/scribble-rs/scribble.rs/internal/game"
	"github.com/scribble-rs/scribble.rs/internal/laninput"
)

func runNativeCapture(endpoint, token, userSession string, listOnly bool) error {
	if !listOnly {
		startInputPoster(endpoint, token, userSession)
	}
	return laninput.Capture(func(input game.LanKeyboardInput) {
		enqueueInput(keyboardInput{
			KeyboardID: input.KeyboardID,
			Key:        input.Key,
			Action:     input.Action,
		})
	}, listOnly)
}
