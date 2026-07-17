package application

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure"
)

const resultUsageText = `dockpipe result — emit a canonical operation-result line

Usage:
  dockpipe result --unit <name> --status <start|progress|done|fail> [flags]

Flags:
  --message <text>       Short human summary
  --duration-ms <ms>     Completed/progress duration in milliseconds
  --id <key=value>       Identifier to attach; repeatable
  --error <text>         Failure text for status=fail
  --event-log <path>     Mirror to an operation-event JSONL path for this call

When --event-log is omitted, DOCKPIPE_EVENT_LOG controls JSONL mirroring.
`

func cmdResult(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(resultUsageText)
		return nil
	}
	result := infrastructure.OperationResult{
		DurationMs: -1,
		IDs:        map[string]string{},
	}
	eventLog := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--unit":
			if i+1 >= len(args) {
				return fmt.Errorf("dockpipe result: --unit requires a value")
			}
			result.Unit = strings.TrimSpace(args[i+1])
			i++
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("dockpipe result: --status requires a value")
			}
			result.Status = strings.TrimSpace(args[i+1])
			i++
		case "--message":
			if i+1 >= len(args) {
				return fmt.Errorf("dockpipe result: --message requires a value")
			}
			result.Message = strings.TrimSpace(args[i+1])
			i++
		case "--duration-ms":
			if i+1 >= len(args) {
				return fmt.Errorf("dockpipe result: --duration-ms requires a value")
			}
			duration, err := strconv.ParseInt(strings.TrimSpace(args[i+1]), 10, 64)
			if err != nil || duration < 0 {
				return fmt.Errorf("dockpipe result: --duration-ms must be a non-negative integer")
			}
			result.DurationMs = duration
			i++
		case "--id":
			if i+1 >= len(args) {
				return fmt.Errorf("dockpipe result: --id requires key=value")
			}
			key, value, ok := strings.Cut(args[i+1], "=")
			if !ok || strings.TrimSpace(key) == "" {
				return fmt.Errorf("dockpipe result: --id requires key=value")
			}
			result.IDs[strings.TrimSpace(key)] = strings.TrimSpace(value)
			i++
		case "--error":
			if i+1 >= len(args) {
				return fmt.Errorf("dockpipe result: --error requires a value")
			}
			result.Error = strings.TrimSpace(args[i+1])
			i++
		case "--event-log":
			if i+1 >= len(args) {
				return fmt.Errorf("dockpipe result: --event-log requires a path")
			}
			eventLog = strings.TrimSpace(args[i+1])
			i++
		default:
			return fmt.Errorf("dockpipe result: unexpected argument %q", args[i])
		}
	}
	if result.Unit == "" {
		return fmt.Errorf("dockpipe result: --unit is required")
	}
	if !validOperationStatus(result.Status) {
		return fmt.Errorf("dockpipe result: --status must be start, progress, done, or fail")
	}
	now := time.Now().UTC()
	if result.Status == infrastructure.OperationStatusStart {
		result.StartedAt = now
	} else {
		result.FinishedAt = now
	}
	if len(result.IDs) == 0 {
		result.IDs = nil
	}

	restore := setEventLogForResult(eventLog)
	defer restore()
	infrastructure.LogOperationResult(os.Stderr, result)
	return nil
}

func validOperationStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case infrastructure.OperationStatusStart, infrastructure.OperationStatusProgress, infrastructure.OperationStatusDone, infrastructure.OperationStatusFail:
		return true
	default:
		return false
	}
}

func setEventLogForResult(eventLog string) func() {
	eventLog = strings.TrimSpace(eventLog)
	if eventLog == "" {
		return func() {}
	}
	old, hadOld := os.LookupEnv(infrastructure.EnvDockpipeEventLog)
	_ = os.Setenv(infrastructure.EnvDockpipeEventLog, eventLog)
	return func() {
		if hadOld {
			_ = os.Setenv(infrastructure.EnvDockpipeEventLog, old)
			return
		}
		_ = os.Unsetenv(infrastructure.EnvDockpipeEventLog)
	}
}
