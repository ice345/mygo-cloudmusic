//go:build !windows

package plugin

import "os/exec"

// hideConsole starts a program without a console window of its own, which
// only Windows has.
func hideConsole(*exec.Cmd) {}
