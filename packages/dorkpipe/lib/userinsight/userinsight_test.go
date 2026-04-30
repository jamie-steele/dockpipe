package userinsight

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnqueueProcessExportFlow(t *testing.T) {
	workdir := t.TempDir()

	id1, err := Enqueue(workdir, "convention: use gofmt for Go.", "unknown", "", "", "")
	if err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if id1 == "" {
		t.Fatal("enqueue 1 returned empty id")
	}
	id2, err := Enqueue(workdir, "SOC2 review will cover secret storage.", "unknown", "", "", "")
	if err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}

	insightsPath := filepath.Join(workdir, "bin", ".dockpipe", "analysis", "insights.json")
	if err := os.MkdirAll(filepath.Dir(insightsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(insightsPath, []byte("null\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Process(workdir)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if res.NewCount != 2 {
		t.Fatalf("expected 2 new insights, got %d", res.NewCount)
	}

	var doc InsightsDoc
	body, err := os.ReadFile(insightsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Insights) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(doc.Insights))
	}
	if doc.Insights[0].ID != "insight-"+id1 {
		t.Fatalf("unexpected first insight id %q", doc.Insights[0].ID)
	}
	if doc.Insights[1].QueueItemID != id2 {
		t.Fatalf("unexpected second queue id %q", doc.Insights[1].QueueItemID)
	}
	if doc.Insights[0].Category != "convention" || doc.Insights[0].Status != "accepted" {
		t.Fatalf("unexpected first insight classification: %+v", doc.Insights[0])
	}
	if doc.Insights[1].Category != "compliance" || doc.Insights[1].Status != "pending" {
		t.Fatalf("unexpected second insight classification: %+v", doc.Insights[1])
	}

	catDir, err := ExportByCategory(workdir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var exported []Insight
	body, err = os.ReadFile(filepath.Join(catDir, "convention.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &exported); err != nil {
		t.Fatal(err)
	}
	if len(exported) != 1 {
		t.Fatalf("expected 1 convention insight, got %d", len(exported))
	}
}

func TestInsightLifecycleMutations(t *testing.T) {
	workdir := t.TempDir()
	if _, err := Enqueue(workdir, "architecture: keep packages isolated.", "unknown", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Enqueue(workdir, "future: revisit orchestration later.", "unknown", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Process(workdir); err != nil {
		t.Fatal(err)
	}

	insightsPath := filepath.Join(workdir, "bin", ".dockpipe", "analysis", "insights.json")
	var doc InsightsDoc
	body, err := os.ReadFile(insightsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Insights) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(doc.Insights))
	}
	newID := doc.Insights[0].ID
	oldID := doc.Insights[1].ID

	status, err := Review(workdir, "reject", newID, "needs human confirmation")
	if err != nil {
		t.Fatal(err)
	}
	if status != "rejected" {
		t.Fatalf("expected rejected, got %q", status)
	}
	if err := MarkStale(workdir, newID); err != nil {
		t.Fatal(err)
	}
	if err := Supersede(workdir, newID, oldID); err != nil {
		t.Fatal(err)
	}

	body, err = os.ReadFile(insightsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	var gotNew, gotOld *Insight
	for i := range doc.Insights {
		switch doc.Insights[i].ID {
		case newID:
			gotNew = &doc.Insights[i]
		case oldID:
			gotOld = &doc.Insights[i]
		}
	}
	if gotNew == nil || gotOld == nil {
		t.Fatalf("missing mutated insights: %+v", doc.Insights)
	}
	if gotNew.RejectionCause != "needs human confirmation" || !gotNew.Stale {
		t.Fatalf("unexpected new insight state: %+v", *gotNew)
	}
	if gotNew.Supersedes == nil || *gotNew.Supersedes != oldID {
		t.Fatalf("unexpected supersedes link: %+v", *gotNew)
	}
	if gotOld.Status != "superseded" || !gotOld.Stale {
		t.Fatalf("unexpected old insight state: %+v", *gotOld)
	}
}
