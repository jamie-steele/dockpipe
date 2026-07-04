package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestApplyProcessEventLogEnvSetsAndRestores(t *testing.T) {
	old, hadOld := os.LookupEnv(infrastructure.EnvDockpipeEventLog)
	t.Cleanup(func() {
		if hadOld {
			os.Setenv(infrastructure.EnvDockpipeEventLog, old)
		} else {
			os.Unsetenv(infrastructure.EnvDockpipeEventLog)
		}
	})
	os.Unsetenv(infrastructure.EnvDockpipeEventLog)
	path := filepath.Join(t.TempDir(), "events.jsonl")
	restore := applyProcessEventLogEnv(map[string]string{
		infrastructure.EnvDockpipeEventLog: path,
	})
	if got := os.Getenv(infrastructure.EnvDockpipeEventLog); got != path {
		t.Fatalf("process event log = %q want %q", got, path)
	}
	restore()
	if got := os.Getenv(infrastructure.EnvDockpipeEventLog); got != "" {
		t.Fatalf("process event log should be unset after restore, got %q", got)
	}
}

func TestApplyProcessEventLogEnvNoopsWhenUnset(t *testing.T) {
	old, hadOld := os.LookupEnv(infrastructure.EnvDockpipeEventLog)
	t.Cleanup(func() {
		if hadOld {
			os.Setenv(infrastructure.EnvDockpipeEventLog, old)
		} else {
			os.Unsetenv(infrastructure.EnvDockpipeEventLog)
		}
	})
	os.Setenv(infrastructure.EnvDockpipeEventLog, "existing")
	restore := applyProcessEventLogEnv(map[string]string{})
	restore()
	if got := os.Getenv(infrastructure.EnvDockpipeEventLog); got != "existing" {
		t.Fatalf("process event log should remain existing, got %q", got)
	}
}
