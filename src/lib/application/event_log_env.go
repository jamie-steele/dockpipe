package application

import (
	"os"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

func applyProcessEventLogEnv(envMap map[string]string) func() {
	next := strings.TrimSpace(envMap[infrastructure.EnvDockpipeEventLog])
	if next == "" {
		return func() {}
	}
	previous, hadPrevious := os.LookupEnv(infrastructure.EnvDockpipeEventLog)
	_ = os.Setenv(infrastructure.EnvDockpipeEventLog, next)
	return func() {
		if hadPrevious {
			_ = os.Setenv(infrastructure.EnvDockpipeEventLog, previous)
			return
		}
		_ = os.Unsetenv(infrastructure.EnvDockpipeEventLog)
	}
}
