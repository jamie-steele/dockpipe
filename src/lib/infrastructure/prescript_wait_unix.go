//go:build !windows

package infrastructure

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// waitHostScriptWithSignalForward forwards terminal-style signals to the bash child so host scripts
// (e.g. cursor-dev) can trap and run docker stop. Without this, only the dockpipe process receives
// SIGTERM from `kill <pid>` and the child keeps running. Linux also uses PR_SET_PDEATHSIG (see
// prescript_runhost_linux.go) when the parent exits without a graceful signal path.
func waitHostScriptWithSignalForward(cmd *exec.Cmd) error {
	sigCh := make(chan os.Signal, 8)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				if cmd.Process != nil {
					_ = cmd.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	signal.Stop(sigCh)
	return err
}
