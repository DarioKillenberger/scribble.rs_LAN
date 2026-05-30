//go:build !windows

package main

import "errors"

func runNativeCapture(_, _, _ string, _ bool) error {
	return errors.New("native keyboard capture is currently implemented only on Windows; rerun with -stdin for protocol testing")
}
