package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseWorkflowYAMLStepRuntime checks per-step runtime field.
func TestParseWorkflowYAMLStepRuntime(t *testing.T) {
	y := `
steps:
  - runtime: code-server
    cmd: echo hi
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 || w.Steps[0].Runtime != "code-server" || w.Steps[0].RuntimeProfileName() != "code-server" {
		t.Fatalf("runtime: got %+v", w.Steps)
	}
}

// TestParseWorkflowYAMLStepResolver checks per-step resolver field.
func TestParseWorkflowYAMLStepResolver(t *testing.T) {
	y := `
steps:
  - resolver: code-server
    cmd: echo hi
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 || w.Steps[0].Resolver != "code-server" {
		t.Fatalf("resolver: got %+v", w.Steps)
	}
}

// TestParseWorkflowYAMLSteps checks multi-step YAML: two steps, per-step isolate override, and CmdLine.
func TestParseWorkflowYAMLSteps(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yml")
	content := `
name: t
isolate: alpine
steps:
  - isolate: alpine
    cmd: echo hi
  - cmd: echo bye
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	w, err := ParseWorkflowYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("steps: got %d", len(w.Steps))
	}
	if w.Steps[0].CmdLine() != "echo hi" {
		t.Fatalf("cmd0: %q", w.Steps[0].CmdLine())
	}
}

// TestParseWorkflowYAMLAsyncGroupAndID checks step id, is_blocking, and DisplayName for flat async-style steps.
func TestParseWorkflowYAMLAsyncGroupAndID(t *testing.T) {
	y := `
steps:
  - id: a
    cmd: echo a
    is_blocking: false
  - id: b
    cmd: echo b
    is_blocking: true
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("steps: %d", len(w.Steps))
	}
	if w.Steps[0].ID != "a" || w.Steps[0].IsBlocking() {
		t.Fatalf("step0: id=%q blocking=%v", w.Steps[0].ID, w.Steps[0].IsBlocking())
	}
	if w.Steps[1].ID != "b" || !w.Steps[1].IsBlocking() {
		t.Fatalf("step1: id=%q blocking=%v", w.Steps[1].ID, w.Steps[1].IsBlocking())
	}
	if w.Steps[0].DisplayName(0) != "a" || w.Steps[1].DisplayName(1) != "b" {
		t.Fatalf("DisplayName: %q %q", w.Steps[0].DisplayName(0), w.Steps[1].DisplayName(1))
	}
}

// TestParseWorkflowYAMLAsyncGroupSugar checks that group.mode: async expands to the expected flattened steps and blocking flags.
func TestParseWorkflowYAMLAsyncGroupSugar(t *testing.T) {
	y := `
steps:
  - id: setup
    cmd: echo setup
    is_blocking: true
  - group:
      mode: async
      tasks:
        - id: task_a
          cmd: echo a
        - id: task_b
          cmd: echo b
  - id: aggregate
    cmd: echo agg
    is_blocking: true
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 4 {
		t.Fatalf("flattened steps: want 4, got %d", len(w.Steps))
	}
	if w.Steps[0].ID != "setup" || !w.Steps[0].IsBlocking() {
		t.Fatalf("step0: %+v", w.Steps[0])
	}
	if w.Steps[1].ID != "task_a" || w.Steps[1].IsBlocking() {
		t.Fatalf("step1 should be non-blocking: %+v", w.Steps[1])
	}
	if w.Steps[2].ID != "task_b" || w.Steps[2].IsBlocking() {
		t.Fatalf("step2 should be non-blocking: %+v", w.Steps[2])
	}
	if w.Steps[3].ID != "aggregate" || !w.Steps[3].IsBlocking() {
		t.Fatalf("step3: %+v", w.Steps[3])
	}
}

// TestParseWorkflowYAMLGroupValidation rejects invalid group blocks (wrong mode, empty tasks, blocking inside tasks, extra keys).
func TestParseWorkflowYAMLGroupValidation(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			"group_mode_must_be_async_not_parallel",
			`steps:
  - group:
      mode: parallel
      tasks:
        - cmd: x
`,
		},
		{
			"group_tasks_must_not_be_empty",
			`steps:
  - group:
      mode: async
      tasks: []
`,
		},
		{
			"group_task_must_not_set_is_blocking_true",
			`steps:
  - group:
      mode: async
      tasks:
        - cmd: x
          is_blocking: true
`,
		},
		{
			"group_step_must_not_mix_group_with_other_step_keys",
			`steps:
  - group:
      mode: async
      tasks:
        - cmd: x
    cmd: oops
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseWorkflowYAML([]byte(tc.yaml))
			if err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

// TestParseWorkflowYAMLDescription checks top-level description is parsed into Workflow.Description.
func TestParseWorkflowYAMLDescription(t *testing.T) {
	y := `name: t
description: Do the task
isolate: alpine
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.Description != "Do the task" {
		t.Fatalf("description: %q", w.Description)
	}
}

func TestParseWorkflowYAMLWorkflowTypeAndCategory(t *testing.T) {
	y := `name: s
workflow_type: secretstore
category: tooling
steps:
  - id: x
    skip_container: true
    run: [scripts/a.sh]
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.WorkflowType != "secretstore" || w.Category != "tooling" {
		t.Fatalf("got workflow_type=%q category=%q", w.WorkflowType, w.Category)
	}
}

func TestParseWorkflowYAMLNamespace(t *testing.T) {
	y := `name: demo
namespace: my-org
run: echo hi
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.Namespace != "my-org" {
		t.Fatalf("namespace: %q", w.Namespace)
	}
	if err := ValidateWorkflowNamespaceField(w); err != nil {
		t.Fatal(err)
	}
}

func TestParseWorkflowYAMLCompileHooks(t *testing.T) {
	y := `name: demo
run: echo hi
compile_hooks:
  - echo one
  - go version
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.CompileHooks) != 2 || w.CompileHooks[0] != "echo one" {
		t.Fatalf("compile_hooks: %#v", w.CompileHooks)
	}
}

func TestValidateWorkflowNamespaceFieldReserved(t *testing.T) {
	if err := ValidateWorkflowNamespaceField(&Workflow{Namespace: "dockpipe"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateWorkflowCapabilityField(t *testing.T) {
	if err := ValidateWorkflowCapabilityField(&Workflow{Capability: "cli.codex"}); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a", 257)
	if err := ValidateWorkflowCapabilityField(&Workflow{Capability: long}); err == nil {
		t.Fatal("expected error for too-long capability")
	}
}

func TestValidateWorkflowTypeField(t *testing.T) {
	if err := ValidateWorkflowTypeField(&Workflow{WorkflowType: "secretstore"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorkflowTypeField(&Workflow{WorkflowType: "Bad"}); err == nil {
		t.Fatal("expected error for uppercase workflow_type")
	}
	if err := ValidateWorkflowTypeField(&Workflow{WorkflowType: "9bad"}); err == nil {
		t.Fatal("expected error when workflow_type does not start with a letter")
	}
	if err := ValidateWorkflowTypeField(nil); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowAnyContainerStep(t *testing.T) {
	w := &Workflow{Steps: []Step{{SkipContainer: true}, {Cmd: "echo x", Isolate: "alpine"}}}
	if !w.AnyContainerStep() {
		t.Fatal("expected AnyContainerStep true when one step uses the container")
	}
	w2 := &Workflow{Steps: []Step{{SkipContainer: true}}}
	if w2.AnyContainerStep() {
		t.Fatal("expected AnyContainerStep false when all steps skip_container")
	}
}

func TestWorkflowNeedsDockerReachable(t *testing.T) {
	vscodeLike := &Workflow{Steps: []Step{{SkipContainer: true, Run: []string{"scripts/vscode/vscode-code-server.sh"}}}}
	if !vscodeLike.NeedsDockerReachable() {
		t.Fatal("expected NeedsDockerReachable when host run: invokes docker")
	}
	hostOnly := &Workflow{Steps: []Step{{SkipContainer: true}}}
	if hostOnly.NeedsDockerReachable() {
		t.Fatal("expected false when no container and no run scripts")
	}
	withStepResolver := &Workflow{Steps: []Step{{SkipContainer: true, Resolver: "cursor"}}}
	if !withStepResolver.NeedsDockerReachable() {
		t.Fatal("expected NeedsDockerReachable when a step references a runtime profile name")
	}
	preflightOff := false
	hostRunNoDocker := &Workflow{DockerPreflight: &preflightOff, Steps: []Step{{SkipContainer: true, Run: []string{"scripts/print.sh"}}}}
	if hostRunNoDocker.NeedsDockerReachable() {
		t.Fatal("expected false when docker_preflight: false and no container steps")
	}
}

func TestParseWorkflowYAMLHostBuiltin(t *testing.T) {
	y := `
name: t
steps:
  - skip_container: true
    host_builtin: package_build_store
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 || w.Steps[0].HostBuiltin != "package_build_store" || !w.Steps[0].SkipContainer {
		t.Fatalf("got %+v", w.Steps[0])
	}
	if err := ValidateLoadedWorkflow(w); err != nil {
		t.Fatal(err)
	}
}

func TestValidateStepHostBuiltinRejectsCombinedRun(t *testing.T) {
	s := Step{SkipContainer: true, HostBuiltin: "package_build_store", Run: []string{"x.sh"}}
	if err := ValidateStepHostBuiltin(0, s); err == nil {
		t.Fatal("expected error when host_builtin is combined with run:")
	}
}

func TestValidateStepHostBuiltinUnknown(t *testing.T) {
	s := Step{SkipContainer: true, HostBuiltin: "nope"}
	if err := ValidateStepHostBuiltin(0, s); err == nil {
		t.Fatal("expected error for unknown host_builtin")
	}
}
