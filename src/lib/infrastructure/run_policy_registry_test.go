package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteRunPolicyRecord(t *testing.T) {
	dir := t.TempDir()
	rec := &RunPolicyRecord{
		WorkflowName:          "secure",
		ImageRef:              "dockpipe-codex",
		ImageArtifactDecision: "using cached image artifact codex",
		PolicyFingerprint:     "sha256:policy",
		PolicySummary:         "runtime policy: network=offline",
		NetworkMode:           "offline",
		NetworkEnforcement:    "native",
		AppliedRuleIDs:        []string{"network.mode.offline"},
	}
	path, err := WriteRunPolicyRecord(dir, rec)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != RunPolicyRecordsDir(dir) {
		t.Fatalf("unexpected run policy dir: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got RunPolicyRecord
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != RunPolicyRecordKind || got.WorkflowName != "secure" || got.ImageRef != "dockpipe-codex" {
		t.Fatalf("unexpected record: %+v", got)
	}
	if got.Workdir == "" || got.CreatedAt == "" || got.ID == "" {
		t.Fatalf("expected populated metadata: %+v", got)
	}
}

func TestListRunPolicyRecordsReadsLegacyDecisionDir(t *testing.T) {
	dir := t.TempDir()
	legacyDir := legacyRunDecisionRecordsDir(dir)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(&RunPolicyRecord{
		Schema:       1,
		Kind:         RunPolicyRecordKind,
		ID:           "a1b2c3d4",
		WorkflowName: "legacy",
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(filepath.Join(legacyDir, "a1b2c3d4.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := ListRunPolicyRecords(dir, &out, RunPolicyListOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "legacy") {
		t.Fatalf("expected legacy policy row, got:\n%s", out.String())
	}
}
