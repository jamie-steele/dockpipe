package infrastructure

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	EnvDockpipeEventLog = "DOCKPIPE_EVENT_LOG"

	OperationEventSchemaV1 = "dockpipe.operation_event.v1"
	OperationEventKind     = "operation_result"

	OperationEventIndexSchemaV1 = "dockpipe.operation_event_index.v1"
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

type OperationEventIndex struct {
	Schema       string                    `json:"schema"`
	Source       string                    `json:"source"`
	GeneratedAt  string                    `json:"generated_at"`
	EventCount   int                       `json:"event_count"`
	StatusCounts map[string]int            `json:"status_counts,omitempty"`
	Units        []OperationEventUnitIndex `json:"units,omitempty"`
}

type OperationEventUnitIndex struct {
	Unit            string         `json:"unit"`
	Count           int            `json:"count"`
	StatusCounts    map[string]int `json:"status_counts,omitempty"`
	FirstTimestamp  string         `json:"first_ts,omitempty"`
	LastTimestamp   string         `json:"last_ts,omitempty"`
	LastStatus      string         `json:"last_status,omitempty"`
	LastError       string         `json:"last_error,omitempty"`
	TotalDurationMs int64          `json:"total_duration_ms,omitempty"`
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

func BuildOperationEventIndex(path string) (OperationEventIndex, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return OperationEventIndex{}, fmt.Errorf("operation event log path is empty")
	}
	events, err := ReadOperationEvents(path)
	if err != nil {
		return OperationEventIndex{}, err
	}
	index := OperationEventIndex{
		Schema:       OperationEventIndexSchemaV1,
		Source:       path,
		GeneratedAt:  timeNowDockerFn().UTC().Format(time.RFC3339Nano),
		EventCount:   len(events),
		StatusCounts: map[string]int{},
	}
	byUnit := map[string]*OperationEventUnitIndex{}
	for _, event := range events {
		status := strings.TrimSpace(event.Status)
		if status == "" {
			status = "unknown"
		}
		index.StatusCounts[status]++
		unit := strings.TrimSpace(event.Unit)
		if unit == "" {
			unit = "unknown"
		}
		row := byUnit[unit]
		if row == nil {
			row = &OperationEventUnitIndex{
				Unit:         unit,
				StatusCounts: map[string]int{},
			}
			byUnit[unit] = row
		}
		row.Count++
		row.StatusCounts[status]++
		if row.FirstTimestamp == "" {
			row.FirstTimestamp = strings.TrimSpace(event.Timestamp)
		}
		if ts := strings.TrimSpace(event.Timestamp); ts != "" {
			row.LastTimestamp = ts
		}
		row.LastStatus = status
		if errText := strings.TrimSpace(event.Error); errText != "" {
			row.LastError = errText
		}
		if event.DurationMs != nil {
			row.TotalDurationMs += *event.DurationMs
		}
	}
	units := make([]string, 0, len(byUnit))
	for unit := range byUnit {
		units = append(units, unit)
	}
	sort.Strings(units)
	for _, unit := range units {
		index.Units = append(index.Units, *byUnit[unit])
	}
	return index, nil
}

func WriteOperationEventIndex(path string, index OperationEventIndex) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("operation event index path is empty")
	}
	if index.Schema == "" {
		index.Schema = OperationEventIndexSchemaV1
	}
	if index.GeneratedAt == "" {
		index.GeneratedAt = timeNowDockerFn().UTC().Format(time.RFC3339Nano)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func appendConfiguredOperationEvent(result OperationResult) {
	path := strings.TrimSpace(os.Getenv(EnvDockpipeEventLog))
	if path == "" {
		return
	}
	_ = AppendOperationEvent(path, OperationEventFromResult(result))
}
