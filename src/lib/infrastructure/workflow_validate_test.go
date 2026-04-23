package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkflowYAMLPath_relativeFromSubdir(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(wfDir, "config.yml")
	if err := os.WriteFile(cfg, []byte("name: demo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	t.Chdir(sub)

	got, err := ResolveWorkflowYAMLPath("workflows/demo/config.yml")
	if err != nil {
		t.Fatalf("ResolveWorkflowYAMLPath: %v", err)
	}
	want, err := filepath.Abs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveWorkflowYAMLPath_defaultSingleWorkflow(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows", "solo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(wfDir, "config.yml")
	if err := os.WriteFile(cfg, []byte("name: solo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	t.Chdir(root)

	got, err := ResolveWorkflowYAMLPath("")
	if err != nil {
		t.Fatalf("ResolveWorkflowYAMLPath: %v", err)
	}
	want, err := filepath.Abs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidateResolvedWorkflowYAML_AcceptsAsyncGroup(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: async-demo
steps:
  - group:
      mode: async
      tasks:
        - id: a
          cmd: echo a
          outputs: bin/.dockpipe/a.env
        - id: b
          cmd: echo b
          outputs: bin/.dockpipe/b.env
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedWorkflowYAML(cfg); err != nil {
		t.Fatalf("expected async group to validate, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsUnknownStepKey(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-step
steps:
  - cmd: echo hi
    runtimee: dockerimage
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "additionalProperties") {
		t.Fatalf("expected schema additionalProperties error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsHostBuiltinWithoutHostKind(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-host-builtin
compose:
  file: assets/compose/docker-compose.yml
steps:
  - host_builtin: compose_up
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "requires kind: host") {
		t.Fatalf("expected host_builtin kind error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsWorkflowWithoutPackage(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: bad-nesting
steps:
  - workflow: child-name
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "requires package: <namespace>") {
		t.Fatalf("expected packaged workflow package error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsTopLevelSingleFlowFieldsWithSteps(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: mixed-shape
run: scripts/setup.sh
steps:
  - cmd: echo hi
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "top-level run") {
		t.Fatalf("expected top-level run with steps error, got %v", err)
	}
}

func TestValidateResolvedWorkflowYAML_RejectsPlainIsBlockingFalse(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "config.yml")
	yml := `name: mixed-async
steps:
  - id: a
    cmd: echo hi
    is_blocking: false
`
	if err := os.WriteFile(cfg, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "is_blocking: false is no longer supported on plain steps") {
		t.Fatalf("expected plain is_blocking false error, got %v", err)
	}
}
