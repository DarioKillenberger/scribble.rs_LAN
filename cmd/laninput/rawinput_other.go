//go:build !windows

package main

import (
	"errors"

	"github.com/scribble-rs/scribble.rs/internal/laninput"
)

func runNativeCapture(_, _, _ string, _ bool) error {
	if err := laninput.Capture(nil, true); err != nil {
		return errors.New(err.Error() + "; rerun with -stdin for protocol testing")
	}
	return nil
}
