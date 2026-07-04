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
		"dry-run: would upload",
		`s3://dockpipe-mirror/artifact.tar.gz`,
		`endpoint="https://account.r2.cloudflarestorage.com"`,
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

	err := cmdReleaseUpload([]string{file, "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "set --bucket") {
		t.Fatalf("expected missing bucket error, got %v", err)
	}
}
