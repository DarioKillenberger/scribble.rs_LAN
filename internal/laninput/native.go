package laninput

import "github.com/scribble-rs/scribble.rs/internal/game"

// Capture blocks while capturing native keyboard input and passes every
// captured keyboard event to handleInput. If listOnly is true, captured input is
// logged instead and handleInput may be nil.
func Capture(handleInput func(game.LanKeyboardInput), listOnly bool) error {
	return captureNative(handleInput, listOnly)
}
