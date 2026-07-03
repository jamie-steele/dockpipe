package infrastructure

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	EnvDockpipeEventLog = "DOCKPIPE_EVENT_LOG"

	OperationEventSchemaV1 = "dockpipe.operation_event.v1"
	OperationEventKind     = "operation_result"
)

type OperationEvent struct {
	Schema     string            `json:"schema"`
	Type       string            `json:"type"`
	Timestamp  string            `json:"ts"`
	Unit       string            `json:"unit"`
	Status     string            `json:"status"`
	Message    string            `json:"message,omitempty"`
	StartedAt  string            `json:"started_at,omitempty"`
	FinishedAt string            `json:"finished_at,omitempty"`
	DurationMs *int64            `json:"duration_ms,omitempty"`
	IDs        map[string]string `json:"ids,omitempty"`
	Error      string            `json:"error,omitempty"`
}

func OperationEventFromResult(result OperationResult) OperationEvent {
	event := OperationEvent{
		Schema:    OperationEventSchemaV1,
		Type:      OperationEventKind,
		Timestamp: operationResultTimestamp(result),
		Unit:      strings.TrimSpace(result.Unit),
		Status:    strings.TrimSpace(result.Status),
		Message:   strings.TrimSpace(result.Message),
		IDs:       copyOperationIDs(result.IDs),
		Error:     strings.TrimSpace(result.Error),
	}
	if !result.StartedAt.IsZero() {
		event.StartedAt = result.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if !result.FinishedAt.IsZero() {
		event.FinishedAt = result.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if result.DurationMs >= 0 && result.Status != OperationStatusStart {
		duration := result.DurationMs
		event.DurationMs = &duration
	}
	return event
}

func AppendOperationEvent(path string, event OperationEvent) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if event.Schema == "" {
		event.Schema = OperationEventSchemaV1
	}
	if event.Type == "" {
		event.Type = OperationEventKind
	}
	if event.Timestamp == "" {
		event.Timestamp = timeNowDockerFn().UTC().Format(time.RFC3339Nano)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(enc, '\n')); err != nil {
		return err
	}
	return nil
}

func ReadOperationEvents(path string) ([]OperationEvent, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("operation event log path is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []OperationEvent
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event OperationEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("read operation event %s:%d: %w", path, lineNo, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func appendConfiguredOperationEvent(result OperationResult) {
	path := strings.TrimSpace(os.Getenv(EnvDockpipeEventLog))
	if path == "" {
		return
	}
	_ = AppendOperationEvent(path, OperationEventFromResult(result))
}
