//go:build !windows

package main

import "os/exec"

func applyProcessAttrs(cmd *exec.Cmd) {
}
