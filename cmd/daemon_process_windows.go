//go:build windows

package cmd

import (
	"os/exec"
	"syscall"
)

func configureDaemonProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200, HideWindow: true}
}
