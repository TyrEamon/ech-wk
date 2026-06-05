//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func applyProcessAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
