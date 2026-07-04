package application

import (
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestResolveWorkflowContainerConfigOverridesPrimaryMountAndAddsSecondaryMounts(t *testing.T) {
	hostBase := filepath.Join(string(filepath.Separator), "repo", "workflow")
	cfg := domain.WorkflowContainerConfig{
		WorkdirHost: "../consumer",
		WorkPath:    "src/app",
		Mounts: []domain.WorkflowContainerMount{
			{Host: "../shared", Guest: "/workspace/shared", Mode: "ro"},
		},
	}

	workHost, workPath, mounts, err := resolveWorkflowContainerConfig(cfg, hostBase, hostBase, "", []string{"/tmp/cache:/cache"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want := filepath.Clean(filepath.Join(hostBase, "..", "consumer")); workHost != want {
		t.Fatalf("workHost = %q want %q", workHost, want)
	}
	if workPath != "src/app" {
		t.Fatalf("workPath = %q want %q", workPath, "src/app")
	}
	if len(mounts) != 2 {
		t.Fatalf("mounts len = %d want 2 (%#v)", len(mounts), mounts)
	}
	if want := filepath.Clean(filepath.Join(hostBase, "..", "shared")) + ":/workspace/shared:ro"; mounts[0] != want {
		t.Fatalf("mount[0] = %q want %q", mounts[0], want)
	}
	if mounts[1] != "/tmp/cache:/cache" {
		t.Fatalf("mount[1] = %q want %q", mounts[1], "/tmp/cache:/cache")
	}
}

func TestMergeWorkflowContainerConfigAppendsStepMounts(t *testing.T) {
	base := domain.WorkflowContainerConfig{
		WorkdirHost: "../consumer",
		Mounts:      []domain.WorkflowContainerMount{{Host: "../shared", Guest: "/shared", Mode: "ro"}},
	}
	override := domain.WorkflowContainerConfig{
		WorkPath: "tools",
		Mounts:   []domain.WorkflowContainerMount{{Host: "../cache", Guest: "/cache", Mode: "rw"}},
	}
	got := mergeWorkflowContainerConfig(base, override)
	if got.WorkdirHost != "../consumer" {
		t.Fatalf("WorkdirHost = %q want %q", got.WorkdirHost, "../consumer")
	}
	if got.WorkPath != "tools" {
		t.Fatalf("WorkPath = %q want %q", got.WorkPath, "tools")
	}
	if len(got.Mounts) != 2 {
		t.Fatalf("Mounts len = %d want 2", len(got.Mounts))
	}
	if got.Mounts[0].Guest != "/shared" || got.Mounts[1].Guest != "/cache" {
		t.Fatalf("Mounts = %#v", got.Mounts)
	}
}
