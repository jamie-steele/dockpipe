package infrastructure

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostRunsDir(t *testing.T) {
	got := HostRunsDir("/tmp/proj")
	want := filepath.Join("/tmp/proj", DockpipeDirRel, "runs")
	if got != want {
		t.Fatalf("HostRunsDir: got %q want %q", got, want)
	}
}

func TestBeginHostRun_emptyWorkdir(t *testing.T) {
	env := []string{"FOO=bar"}
	rid, rf, out, err := BeginHostRun("", env)
	if err != nil {
		t.Fatal(err)
	}
	if rid != "" || rf != "" {
		t.Fatalf("expected no run id/file for empty workdir, got %q %q", rid, rf)
	}
	if len(out) != len(env) {
		t.Fatalf("env should be unchanged, got %d vs %d", len(out), len(env))
	}
}

func TestListHostRuns_emptyDir(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := ListHostRuns(dir, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No host runs") {
		t.Fatalf("expected empty message, got %q", buf.String())
	}
}

func TestWriteHostRunRecord_listRoundTrip(t *testing.T) {
	dir := t.TempDir()
	runsDir := HostRunsDir(dir)
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runID := "a1b2c3d4"
	runFile := filepath.Join(runsDir, runID+".json")
	if err := WriteHostRunRecord(runFile, runID, 4242, dir, "hello.sh"); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(runsDir, runID+".container")
	if err := os.WriteFile(sidecar, []byte("my-container\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ListHostRuns(dir, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, runID) || !strings.Contains(out, "4242") {
		t.Fatalf("expected id and pid in output: %q", out)
	}
	if !strings.Contains(out, "my-container") {
		t.Fatalf("expected container sidecar merged: %q", out)
	}
	RemoveHostRunArtifacts(runFile)
	if _, err := os.Stat(runFile); !os.IsNotExist(err) {
		t.Fatalf("json should be removed: %v", err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("container sidecar should be removed: %v", err)
	}
}
