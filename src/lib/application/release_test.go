package application

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestResolveReleaseEndpointPrefersExplicitEnv(t *testing.T) {
	t.Setenv(envR2Endpoint, "https://explicit.example")
	t.Setenv(envAWSEndpointS3, "https://ignored.example")
	t.Setenv(envCloudflareAcct, "ignored-account")

	if got := resolveReleaseEndpoint(); got != "https://explicit.example" {
		t.Fatalf("resolveReleaseEndpoint() = %q want explicit endpoint", got)
	}
}

func TestResolveReleaseEndpointFallsBackToCloudflareAccount(t *testing.T) {
	t.Setenv(envR2Endpoint, "")
	t.Setenv(envAWSEndpointS3, "")
	t.Setenv(envCloudflareAcct, "1234567890abcdef1234567890abcdef")
	t.Setenv(envR2AccountID, "")

	got := resolveReleaseEndpoint()
	want := "https://1234567890abcdef1234567890abcdef.r2.cloudflarestorage.com"
	if got != want {
		t.Fatalf("resolveReleaseEndpoint() = %q want %q", got, want)
	}
}

func TestCmdReleaseUploadDryRunUsesEnvBucketAndEndpoint(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "artifact.tar.gz")
	if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envReleaseBucket, "dockpipe-mirror")
	t.Setenv(envR2Endpoint, "https://account.r2.cloudflarestorage.com")
	t.Setenv(envAWSRegion, "")

	stderr, err := captureResultStderr(t, func() error {
		return cmdReleaseUpload([]string{file, "--dry-run"})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=release.upload.preflight",
		"status=done",
		"unit=release.upload",
		"status=done",
		"result=dry_run",
		`remote=s3://dockpipe-mirror/artifact.tar.gz`,
		`endpoint=https://account.r2.cloudflarestorage.com`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdReleaseUploadDryRunMirrorsOperationEvents(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "artifact.tar.gz")
	if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	eventLog := filepath.Join(dir, "events.jsonl")
	t.Setenv(infrastructure.EnvDockpipeEventLog, eventLog)
	t.Setenv(envReleaseBucket, "dockpipe-mirror")
	t.Setenv(envR2Endpoint, "https://account.r2.cloudflarestorage.com")
	t.Setenv(envAWSRegion, "")

	if _, err := captureResultStderr(t, func() error {
		return cmdReleaseUpload([]string{file, "--dry-run"})
	}); err != nil {
		t.Fatal(err)
	}
	events, err := infrastructure.ReadOperationEvents(eventLog)
	if err != nil {
		t.Fatalf("ReadOperationEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 operation events, got %d: %#v", len(events), events)
	}
	preflight := events[0]
	if preflight.Schema != infrastructure.OperationEventSchemaV1 || preflight.Type != infrastructure.OperationEventKind {
		t.Fatalf("unexpected preflight event envelope: %#v", preflight)
	}
	if preflight.Unit != "release.upload.preflight" || preflight.Status != infrastructure.OperationStatusDone {
		t.Fatalf("unexpected preflight event status: %#v", preflight)
	}
	for key, want := range map[string]string{
		"bucket": "dockpipe-mirror",
		"remote": "s3://dockpipe-mirror/artifact.tar.gz",
		"result": "dry_run",
		"region": "auto",
	} {
		if got := preflight.IDs[key]; got != want {
			t.Fatalf("preflight event ID %s = %q want %q (event: %#v)", key, got, want, preflight)
		}
	}
	upload := events[1]
	if upload.Unit != "release.upload" || upload.Status != infrastructure.OperationStatusDone {
		t.Fatalf("unexpected upload event status: %#v", upload)
	}
	if got := upload.IDs["result"]; got != "dry_run" {
		t.Fatalf("upload event result = %q want dry_run (event: %#v)", got, upload)
	}
}

func TestCmdReleaseUploadRequiresBucket(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "artifact.tar.gz")
	if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envReleaseBucket, "")
	t.Setenv(envR2Bucket, "")

	stderr, err := captureResultStderr(t, func() error {
		return cmdReleaseUpload([]string{file, "--dry-run"})
	})
	if err == nil || !strings.Contains(err.Error(), "set --bucket") {
		t.Fatalf("expected missing bucket error, got %v", err)
	}
	for _, want := range []string{
		"unit=release.upload.preflight",
		"status=fail",
		"result=missing_bucket",
		"error=",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected missing bucket stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdReleaseUploadMissingFileEmitsPreflightResult(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "missing.tar.gz")
	t.Setenv(envReleaseBucket, "dockpipe-mirror")

	stderr, err := captureResultStderr(t, func() error {
		return cmdReleaseUpload([]string{file, "--dry-run"})
	})
	if err == nil || !strings.Contains(err.Error(), "missing.tar.gz") {
		t.Fatalf("expected missing file error, got %v", err)
	}
	for _, want := range []string{
		"unit=release.upload.preflight",
		"status=fail",
		"result=missing_file",
		"error=",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected missing file stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdReleaseUploadMissingAWSCLIEmitsOperationResult(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "artifact.tar.gz")
	if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(dir, "empty-path")
	if err := os.MkdirAll(emptyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", emptyPath)
	t.Setenv(envReleaseBucket, "dockpipe-mirror")
	t.Setenv(envR2Endpoint, "")
	t.Setenv(envAWSRegion, "")

	stderr, err := captureResultStderr(t, func() error {
		return cmdReleaseUpload([]string{file})
	})
	if err == nil || !strings.Contains(err.Error(), "aws CLI not found") {
		t.Fatalf("expected missing aws CLI error, got %v", err)
	}
	for _, want := range []string{
		"unit=release.upload.preflight",
		"status=fail",
		"result=missing_aws_cli",
		"remote=s3://dockpipe-mirror/artifact.tar.gz",
		"error=",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdReleaseUploadPreflightFailuresMirrorOperationEvents(t *testing.T) {
	tests := []struct {
		name      string
		prepare   func(t *testing.T, dir string) ([]string, map[string]string)
		errorText string
	}{
		{
			name: "directory",
			prepare: func(t *testing.T, dir string) ([]string, map[string]string) {
				t.Setenv(envReleaseBucket, "dockpipe-mirror")
				return []string{dir, "--dry-run"}, map[string]string{
					"local_path": filepath.ToSlash(filepath.Clean(dir)), "result": "non_regular_file",
				}
			},
			errorText: "must be a regular file",
		},
		{
			name: "empty normalized key",
			prepare: func(t *testing.T, dir string) ([]string, map[string]string) {
				file := filepath.Join(dir, "artifact.tar.gz")
				if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Setenv(envReleaseBucket, "dockpipe-mirror")
				return []string{file, "--key", "/", "--dry-run"}, map[string]string{
					"local_path": filepath.ToSlash(file), "bucket": "dockpipe-mirror", "key": "", "remote": "s3://dockpipe-mirror/", "region": "us-east-1", "result": "missing_key",
				}
			},
			errorText: "object key must not be empty",
		},
		{
			name: "missing file",
			prepare: func(t *testing.T, dir string) ([]string, map[string]string) {
				file := filepath.Join(dir, "missing.tar.gz")
				t.Setenv(envReleaseBucket, "dockpipe-mirror")
				return []string{file, "--dry-run"}, map[string]string{
					"local_path": filepath.ToSlash(file), "result": "missing_file",
				}
			},
			errorText: "missing.tar.gz",
		},
		{
			name: "missing bucket",
			prepare: func(t *testing.T, dir string) ([]string, map[string]string) {
				file := filepath.Join(dir, "artifact.tar.gz")
				if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Setenv(envReleaseBucket, "")
				t.Setenv(envR2Bucket, "")
				return []string{file, "--dry-run"}, map[string]string{
					"local_path": filepath.ToSlash(file), "result": "missing_bucket",
				}
			},
			errorText: "set --bucket",
		},
		{
			name: "missing aws cli",
			prepare: func(t *testing.T, dir string) ([]string, map[string]string) {
				file := filepath.Join(dir, "artifact.tar.gz")
				if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
					t.Fatal(err)
				}
				emptyPath := filepath.Join(dir, "empty-path")
				if err := os.MkdirAll(emptyPath, 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", emptyPath)
				t.Setenv(envReleaseBucket, "dockpipe-mirror")
				return []string{file}, map[string]string{
					"local_path": filepath.ToSlash(file), "bucket": "dockpipe-mirror", "key": "artifact.tar.gz", "remote": "s3://dockpipe-mirror/artifact.tar.gz", "region": "us-east-1", "result": "missing_aws_cli",
				}
			},
			errorText: "aws CLI not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			eventLog := filepath.Join(dir, "events.jsonl")
			t.Setenv(infrastructure.EnvDockpipeEventLog, eventLog)
			t.Setenv(envR2Endpoint, "")
			t.Setenv(envAWSEndpointS3, "")
			t.Setenv(envCloudflareAcct, "")
			t.Setenv(envR2AccountID, "")
			t.Setenv(envAWSRegion, "")

			args, wantIDs := tt.prepare(t, dir)
			if _, err := captureResultStderr(t, func() error { return cmdReleaseUpload(args) }); err == nil || !strings.Contains(err.Error(), tt.errorText) {
				t.Fatalf("expected error containing %q, got %v", tt.errorText, err)
			}
			events, err := infrastructure.ReadOperationEvents(eventLog)
			if err != nil {
				t.Fatalf("ReadOperationEvents: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("expected one preflight event and no release.upload event, got %#v", events)
			}
			event := events[0]
			if event.Schema != infrastructure.OperationEventSchemaV1 || event.Type != infrastructure.OperationEventKind || event.Unit != "release.upload.preflight" || event.Status != infrastructure.OperationStatusFail {
				t.Fatalf("unexpected preflight failure event: %#v", event)
			}
			if !reflect.DeepEqual(event.IDs, wantIDs) {
				t.Fatalf("event IDs = %#v want %#v", event.IDs, wantIDs)
			}
			if !strings.Contains(event.Error, tt.errorText) {
				t.Fatalf("event error = %q should contain %q", event.Error, tt.errorText)
			}
		})
	}
}
