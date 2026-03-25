//go:build windows

package infrastructure

import (
	"os"
	"os/exec"
	"os/signal"
)

// waitHostScriptWithSignalForward forwards Ctrl+C (Interrupt) to the bash child on Windows.
func waitHostScriptWithSignalForward(cmd *exec.Cmd) error {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, os.Interrupt)
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
