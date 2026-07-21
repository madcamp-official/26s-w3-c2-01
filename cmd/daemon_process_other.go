//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

func configureDaemonProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
