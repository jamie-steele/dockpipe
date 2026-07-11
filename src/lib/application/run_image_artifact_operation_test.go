package application

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestLogImageArtifactOperationResultRendersAndMirrorsSuccessAndFailure(t *testing.T) {
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(infrastructure.EnvDockpipeEventLog, eventLog)
	stderr := captureRunTestStderr(t, func() {
		logImageArtifactOperationResult("run.image_artifact.cache", "dockpipe-test:image", nil)
		logImageArtifactOperationResult("run.image_artifact.index", "dockpipe-test:image", errors.New("index unavailable"))
	})
	if !strings.Contains(stderr, "unit=run.image_artifact.cache status=done") || !strings.Contains(stderr, "unit=run.image_artifact.index status=fail") || !strings.Contains(stderr, "error=\"index unavailable\"") {
		t.Fatalf("unexpected human operation results: %s", stderr)
	}
	events, err := infrastructure.ReadOperationEvents(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2", len(events))
	}
	if events[0].Schema != infrastructure.OperationEventSchemaV1 || events[0].Status != infrastructure.OperationStatusDone || events[0].Unit != "run.image_artifact.cache" {
		t.Fatalf("unexpected success event: %#v", events[0])
	}
	if events[1].Schema != infrastructure.OperationEventSchemaV1 || events[1].Status != infrastructure.OperationStatusFail || events[1].Unit != "run.image_artifact.index" || events[1].Error != "index unavailable" {
		t.Fatalf("unexpected failure event: %#v", events[1])
	}
}
