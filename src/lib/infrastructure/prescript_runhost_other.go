//go:build !linux

package infrastructure

import "os/exec"

func setRunHostProcAttrs(cmd *exec.Cmd) {}
