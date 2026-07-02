package infrastructure

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	OperationStatusStart    = "start"
	OperationStatusProgress = "progress"
	OperationStatusDone     = "done"
	OperationStatusFail     = "fail"
)

type OperationResult struct {
	Unit       string
	Status     string
	Message    string
	StartedAt  time.Time
	FinishedAt time.Time
	DurationMs int64
	IDs        map[string]string
	Error      string
}

type OperationOptions struct {
	Spinner       bool
	ProgressEvery time.Duration
}

func RunOperation(stderr *os.File, unit, spinnerMessage string, ids map[string]string, fn func() error) error {
	_, err := RunOperationWithResult(stderr, unit, spinnerMessage, ids, fn)
	return err
}

func RunOperationWithOptions(stderr *os.File, unit, spinnerMessage string, ids map[string]string, options OperationOptions, fn func() error) error {
	_, err := RunOperationWithResultOptions(stderr, unit, spinnerMessage, ids, options, fn)
	return err
}

func LogOperationResult(stderr *os.File, result OperationResult) {
	logOperationResult(stderr, result)
}

func RunOperationWithResult(stderr *os.File, unit, spinnerMessage string, ids map[string]string, fn func() error) (OperationResult, error) {
	return RunOperationWithResultOptions(stderr, unit, spinnerMessage, ids, OperationOptions{Spinner: true}, fn)
}

func RunOperationWithResultOptions(stderr *os.File, unit, spinnerMessage string, ids map[string]string, options OperationOptions, fn func() error) (OperationResult, error) {
	if stderr == nil {
		stderr = os.Stderr
	}
	startedAt := timeNowDockerFn()
	stopSpinner := func() {}
	stopProgress := func() {}
	if options.Spinner {
		if fd, ok := fdInt(stderr); ok && isTerminalDockerFn(fd) {
			if msg := strings.TrimSpace(spinnerMessage); msg != "" {
				stopSpinner = StartLineSpinner(stderr, msg)
			}
		} else {
			logOperationResult(stderr, OperationResult{
				Unit:      unit,
				Status:    OperationStatusStart,
				Message:   strings.TrimSpace(spinnerMessage),
				StartedAt: startedAt,
				IDs:       copyOperationIDs(ids),
			})
		}
	} else {
		if msg := strings.TrimSpace(spinnerMessage); msg != "" {
			logOperationResult(stderr, OperationResult{
				Unit:      unit,
				Status:    OperationStatusStart,
				Message:   msg,
				StartedAt: startedAt,
				IDs:       copyOperationIDs(ids),
			})
		}
		if options.ProgressEvery > 0 {
			stopProgress = startOperationProgressLogger(stderr, unit, strings.TrimSpace(spinnerMessage), copyOperationIDs(ids), startedAt, options.ProgressEvery)
		}
	}
	err := fn()
	stopSpinner()
	stopProgress()
	finishedAt := timeNowDockerFn()
	result := OperationResult{
		Unit:       unit,
		Status:     OperationStatusDone,
		Message:    strings.TrimSpace(spinnerMessage),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		DurationMs: finishedAt.Sub(startedAt).Milliseconds(),
		IDs:        copyOperationIDs(ids),
	}
	if err != nil {
		result.Status = OperationStatusFail
		result.Error = strings.TrimSpace(err.Error())
	}
	logOperationResult(stderr, result)
	return result, err
}

func startOperationProgressLogger(stderr *os.File, unit, message string, ids map[string]string, startedAt time.Time, every time.Duration) func() {
	if every <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				now := timeNowDockerFn()
				logOperationResult(stderr, OperationResult{
					Unit:       unit,
					Status:     OperationStatusProgress,
					Message:    message,
					StartedAt:  startedAt,
					FinishedAt: now,
					DurationMs: now.Sub(startedAt).Milliseconds(),
					IDs:        copyOperationIDs(ids),
				})
			}
		}
	}()
	return func() {
		once.Do(func() { close(done) })
	}
}

func logOperationResult(stderr *os.File, result OperationResult) {
	if stderr == nil {
		stderr = os.Stderr
	}
	fields := []string{
		"[dockpipe]",
		"ts=" + operationResultTimestamp(result),
		"unit=" + result.Unit,
		"status=" + result.Status,
	}
	if result.Status != OperationStatusStart && result.DurationMs >= 0 {
		fields = append(fields, "duration_ms="+strconv.FormatInt(result.DurationMs, 10))
	}
	for _, key := range sortedOperationIDKeys(result.IDs) {
		value := strings.TrimSpace(result.IDs[key])
		if value == "" {
			continue
		}
		fields = append(fields, key+"="+value)
	}
	if result.Status == OperationStatusFail && result.Error != "" {
		fields = append(fields, fmt.Sprintf("error=%q", result.Error))
	}
	fmt.Fprintln(stderr, strings.Join(fields, " "))
}

func copyOperationIDs(ids map[string]string) map[string]string {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]string, len(ids))
	for key, value := range ids {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sortedOperationIDKeys(ids map[string]string) []string {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, 0, len(ids))
	for key := range ids {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func OperationEventFields(result OperationResult) map[string]string {
	fields := map[string]string{
		"type":   "operation." + strings.TrimSpace(result.Status),
		"unit":   strings.TrimSpace(result.Unit),
		"status": strings.TrimSpace(result.Status),
	}
	if !result.StartedAt.IsZero() {
		fields["started_at"] = result.StartedAt.UTC().Format(time.RFC3339)
	}
	if !result.FinishedAt.IsZero() {
		fields["finished_at"] = result.FinishedAt.UTC().Format(time.RFC3339)
	}
	if result.DurationMs >= 0 && result.Status != OperationStatusStart {
		fields["duration_ms"] = strconv.FormatInt(result.DurationMs, 10)
	}
	if strings.TrimSpace(result.Error) != "" {
		fields["error"] = strings.TrimSpace(result.Error)
	}
	for key, value := range result.IDs {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		fields[key] = value
	}
	return fields
}

func operationResultTimestamp(result OperationResult) string {
	if !result.FinishedAt.IsZero() && result.Status != OperationStatusStart {
		return result.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if !result.StartedAt.IsZero() {
		return result.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	return timeNowDockerFn().UTC().Format(time.RFC3339Nano)
}
