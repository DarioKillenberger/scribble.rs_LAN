//go:build !windows

package laninput

import (
	"errors"

	"github.com/scribble-rs/scribble.rs/internal/game"
)

func captureNative(_ func(game.LanKeyboardInput), _ bool) error {
	return errors.New("native keyboard capture is currently implemented only on Windows")
}
