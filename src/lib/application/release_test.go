package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
