//go:build windows

package plugin

import (
	"os/exec"
	"syscall"
)

// hideConsole starts a program without a console window of its own.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
