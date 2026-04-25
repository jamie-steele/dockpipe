//go:build linux

package infrastructure

import (
	"os/exec"
	"syscall"
)

// setRunHostProcAttrs configures the bash child so it receives SIGTERM when the dockpipe parent
// exits (PR_SET_PDEATHSIG). Otherwise a kill(1) on dockpipe leaves the child running (e.g. docker wait)
// and detached containers stay up.
func setRunHostProcAttrs(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
