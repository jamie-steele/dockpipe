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

func TestParseWorkflowYAMLStepVM(t *testing.T) {
	y := `
steps:
  - runtime: vm
    resolver: qemu
    vm:
      mounts:
        - host: C:\src\repo
          guest: C:\uh
        - host: C:\tmp\artifacts
          guest: C:\artifacts
      guest_path: C:\uh
      host_context: C:\src\repo
      interactive_debug: true
      interactive_ssh: false
      keepalive: true
      keepalive_seconds: "28800"
      hostfwd: tcp::3389-:3389
    cmd: hostname
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 {
		t.Fatalf("steps: got %d", len(w.Steps))
	}
	got := w.Steps[0].VM
	if got.GuestPath != `C:\uh` || got.HostContext != `C:\src\repo` || got.InteractiveDebug == nil || !*got.InteractiveDebug || got.InteractiveSSH == nil || *got.InteractiveSSH || got.KeepAlive == nil || !*got.KeepAlive || got.KeepAliveSeconds != "28800" || got.HostFwd != "tcp::3389-:3389" {
		t.Fatalf("vm: got %+v", got)
	}
	if len(got.Mounts) != 2 || got.Mounts[0].Host != `C:\src\repo` || got.Mounts[0].Guest != `C:\uh` || got.Mounts[1].Host != `C:\tmp\artifacts` || got.Mounts[1].Guest != `C:\artifacts` {
		t.Fatalf("vm.mounts: got %+v", got.Mounts)
	}
}

func TestValidateStepVMFieldRejectsConflictingInteractiveModes(t *testing.T) {
	step := Step{
		VM: StepVMConfig{
			InteractiveDebug: func() *bool {
				v := true
				return &v
			}(),
			InteractiveSSH: func() *bool {
				v := true
				return &v
			}(),
		},
	}
	err := ValidateStepVMField(0, step)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive vm interactive mode error, got %v", err)
	}
}

func TestParseWorkflowYAMLInputs(t *testing.T) {
	y := `
types:
  - models/QemuVmResolverConfig.pipe
inputs:
  Advanced.KeepAlive:
    from: UH_VM_KEEPALIVE
    value: false
steps:
  - inputs:
      General.ExecMode: powershell
    cmd: hostname
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Inputs["Advanced.KeepAlive"]; got.From != "UH_VM_KEEPALIVE" || got.Value != "false" {
		t.Fatalf("workflow inputs: %+v", got)
	}
	if got := w.Steps[0].Inputs["General.ExecMode"]; got.Value != "powershell" {
		t.Fatalf("step inputs: %+v", got)
	}
}

func TestParseWorkflowYAMLStepPackageWorkflowField(t *testing.T) {
	y := `
steps:
  - workflow: nested-flow
    package: dockpipe.demo
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 || w.Steps[0].WorkflowName != "nested-flow" || w.Steps[0].Package != "dockpipe.demo" {
		t.Fatalf("workflow/package: got %+v", w.Steps)
	}
}

func TestParseWorkflowYAMLAgentTaskRoleContext(t *testing.T) {
	y := `
steps:
  - kind: host
    run: scripts/dorkpipe/orchestrate-plan.sh
    agent:
      orchestration:
        tasks:
          - id: scout
            agent: source_scout
            brief: Scout the repo
            context:
              required_artifacts:
                - shared/request.md
              source_roots:
                - /work
            goal: Scout the repo
            materialize_outputs:
              - path: index.md
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 || w.Steps[0].Agent.Orchestration == nil || len(w.Steps[0].Agent.Orchestration.Tasks) != 1 {
		t.Fatalf("agent task parse: got %+v", w.Steps)
	}
	task := w.Steps[0].Agent.Orchestration.Tasks[0]
	if task.Agent != "source_scout" || task.Brief != "Scout the repo" || len(task.Context.RequiredArtifacts) != 1 || len(task.Context.SourceRoots) != 1 || len(task.MaterializeOutputs) != 1 {
		t.Fatalf("agent task role/context parse: got %+v", task)
	}
}

func TestParseWorkflowYAMLRejectsRuntimePackageResolverOverload(t *testing.T) {
	y := `
steps:
  - runtime: package
    resolver: old-style-name
    package: dockpipe.demo
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = ValidateLoadedWorkflow(w)
	if err == nil || !strings.Contains(err.Error(), "runtime: package is no longer supported") {
		t.Fatalf("expected runtime package validation error, got %v", err)
	}
}

func TestParseWorkflowYAMLRejectsSkipContainerAlias(t *testing.T) {
	y := `
steps:
  - skip_container: true
    cmd: echo hi
`
	_, err := ParseWorkflowYAML([]byte(y))
	if err == nil || !strings.Contains(err.Error(), "skip_container is no longer supported") {
		t.Fatalf("expected skip_container rejection, got %v", err)
	}
}

func TestParseWorkflowYAMLRejectsDefaultResolver(t *testing.T) {
	y := `
name: t
default_resolver: codex
`
	_, err := ParseWorkflowYAML([]byte(y))
	if err == nil || !strings.Contains(err.Error(), "default_resolver is no longer supported") {
		t.Fatalf("expected default_resolver rejection, got %v", err)
	}
}

func TestParseWorkflowYAMLSecurityNetwork(t *testing.T) {
	y := `
security:
  profile: sidecar-client
  network:
    mode: offline
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.Security.Profile != "sidecar-client" {
		t.Fatalf("security.profile: got %q", w.Security.Profile)
	}
	if w.Security.Network.Mode != "offline" {
		t.Fatalf("security.network.mode: got %q", w.Security.Network.Mode)
	}
	if len(w.Security.Network.Allow) != 0 {
		t.Fatalf("security.network.allow: got %#v", w.Security.Network.Allow)
	}
}

func TestParseWorkflowYAMLStepSecurity(t *testing.T) {
	y := `
steps:
  - cmd: echo hi
    security:
      profile: internet-client
      network:
        mode: allowlist
        allow: [api.openai.com]
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Steps[0].Security.Profile; got != "internet-client" {
		t.Fatalf("step security.profile: %q", got)
	}
	if got := w.Steps[0].Security.Network.Mode; got != "allowlist" {
		t.Fatalf("step security.network.mode: %q", got)
	}
}

func TestParseWorkflowYAMLContainer(t *testing.T) {
	y := `
container:
  workdir_host: ../consumer
  work_path: src/app
  mounts:
    - host: ../shared
      guest: /workspace/shared
      mode: ro
steps:
  - cmd: echo hi
    container:
      workdir_host: ../override
      work_path: tools
      mounts:
        - host: ../cache
          guest: /workspace/cache
          mode: rw
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.Container.WorkdirHost != "../consumer" || w.Container.WorkPath != "src/app" {
		t.Fatalf("workflow container: %+v", w.Container)
	}
	if len(w.Container.Mounts) != 1 || w.Container.Mounts[0].Guest != "/workspace/shared" || w.Container.Mounts[0].Mode != "ro" {
		t.Fatalf("workflow container mounts: %+v", w.Container.Mounts)
	}
	if len(w.Steps) != 1 {
		t.Fatalf("steps: got %d", len(w.Steps))
	}
	got := w.Steps[0].Container
	if got.WorkdirHost != "../override" || got.WorkPath != "tools" {
		t.Fatalf("step container: %+v", got)
	}
	if len(got.Mounts) != 1 || got.Mounts[0].Guest != "/workspace/cache" || got.Mounts[0].Mode != "rw" {
		t.Fatalf("step container mounts: %+v", got.Mounts)
	}
}

func TestValidateStepContainerFieldRejectsHostAndPackagedWorkflow(t *testing.T) {
	hostStep := Step{
		Kind: "host",
		Container: WorkflowContainerConfig{
			WorkdirHost: "../repo",
		},
	}
	if err := ValidateStepContainerField(0, hostStep); err == nil || !strings.Contains(err.Error(), "kind: host step does not use container") {
		t.Fatalf("expected host-step container validation error, got %v", err)
	}

	packaged := Step{
		WorkflowName: "child",
		Package:      "dockpipe.demo",
		Container: WorkflowContainerConfig{
			Mounts: []WorkflowContainerMount{{Host: "../repo", Guest: "/work"}},
		},
	}
	if err := ValidateStepContainerField(0, packaged); err == nil || !strings.Contains(err.Error(), "packaged workflow step does not use container") {
		t.Fatalf("expected packaged-step container validation error, got %v", err)
	}
}

func TestValidateWorkflowContainerConfigRejectsInvalidWorkPathAndMode(t *testing.T) {
	err := ValidateWorkflowContainerConfig("container", WorkflowContainerConfig{WorkPath: "/absolute"})
	if err == nil || !strings.Contains(err.Error(), "work_path must be relative") {
		t.Fatalf("expected work_path validation error, got %v", err)
	}

	err = ValidateWorkflowContainerConfig("container", WorkflowContainerConfig{
		Mounts: []WorkflowContainerMount{{Host: "../repo", Guest: "/work", Mode: "readonly"}},
	})
	if err == nil || !strings.Contains(err.Error(), "container.mounts[0].mode") {
		t.Fatalf("expected mount mode validation error, got %v", err)
	}
}

func TestParseWorkflowYAMLWorkspace(t *testing.T) {
	y := `
name: session-demo
workspace:
  repo: biztraak
  mode: managed
  base: main
  storage: worktree
  lifecycle:
    branch_prefix: ai
    branch: js/features/demo
    checkpoint: auto
    publish: review
steps:
  - id: ok
    kind: host
    cmd: echo ok
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Workspace.Repo; got != "biztraak" {
		t.Fatalf("workspace.repo = %q", got)
	}
	if got := w.Workspace.Lifecycle.Checkpoint; got != "auto" {
		t.Fatalf("workspace.lifecycle.checkpoint = %q", got)
	}
	if got := w.Workspace.Lifecycle.Branch; got != "js/features/demo" {
		t.Fatalf("workspace.lifecycle.branch = %q", got)
	}
	if err := ValidateLoadedWorkflow(w); err != nil {
		t.Fatalf("ValidateLoadedWorkflow: %v", err)
	}
}

func TestValidateWorkflowWorkspaceConfigRejectsBadMode(t *testing.T) {
	err := ValidateWorkflowWorkspaceConfig("workspace", WorkflowWorkspaceConfig{Mode: "magic"})
	if err == nil || !strings.Contains(err.Error(), "workspace.mode") {
		t.Fatalf("expected workspace.mode validation error, got %v", err)
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

// TestParseWorkflowYAMLStepIDAndDisplayName checks step id and DisplayName for plain steps.
func TestParseWorkflowYAMLStepIDAndDisplayName(t *testing.T) {
	y := `
steps:
  - id: a
    cmd: echo a
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
	if w.Steps[0].ID != "a" || !w.Steps[0].IsBlocking() {
		t.Fatalf("step0: id=%q blocking=%v", w.Steps[0].ID, w.Steps[0].IsBlocking())
	}
	if w.Steps[1].ID != "b" || !w.Steps[1].IsBlocking() {
		t.Fatalf("step1: id=%q blocking=%v", w.Steps[1].ID, w.Steps[1].IsBlocking())
	}
	if w.Steps[0].DisplayName(0) != "a" || w.Steps[1].DisplayName(1) != "b" {
		t.Fatalf("DisplayName: %q %q", w.Steps[0].DisplayName(0), w.Steps[1].DisplayName(1))
	}
}

func TestParseWorkflowYAMLRejectsPlainIsBlockingFalse(t *testing.T) {
	y := `
steps:
  - id: a
    cmd: echo a
    is_blocking: false
`
	_, err := ParseWorkflowYAML([]byte(y))
	if err == nil || !strings.Contains(err.Error(), "is_blocking: false is no longer supported on plain steps") {
		t.Fatalf("expected plain is_blocking false rejection, got %v", err)
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

func TestParseWorkflowYAMLFinallySugar(t *testing.T) {
	y := `
steps:
  - id: main
    cmd: echo main
finally:
  - id: cleanup
    kind: host
    run: scripts/cleanup.sh
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 || w.Steps[0].ID != "main" {
		t.Fatalf("unexpected steps: %+v", w.Steps)
	}
	if len(w.Finally) != 1 || w.Finally[0].ID != "cleanup" {
		t.Fatalf("unexpected finally steps: %+v", w.Finally)
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
icon: assets/images/icon.png
steps:
  - id: x
    kind: host
    run: [scripts/a.sh]
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.WorkflowType != "secretstore" || w.Category != "tooling" {
		t.Fatalf("got workflow_type=%q category=%q", w.WorkflowType, w.Category)
	}
	if w.Icon != "assets/images/icon.png" {
		t.Fatalf("icon: %q", w.Icon)
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

func TestParseWorkflowYAMLImagePackages(t *testing.T) {
	y := `name: demo
image:
  packages:
    apt:
      - golang-go
      - cargo
run: echo hi
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	got := w.Image.Packages.Apt
	if len(got) != 2 || got[0] != "golang-go" || got[1] != "cargo" {
		t.Fatalf("image.packages.apt: %#v", got)
	}
}

func TestParseWorkflowYAMLVault(t *testing.T) {
	y := `name: demo
vault: none
run: echo hi
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.Vault != "none" {
		t.Fatalf("vault: %q", w.Vault)
	}
	if err := ValidateWorkflowVaultField(w); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWorkflowNamespaceFieldReserved(t *testing.T) {
	if err := ValidateWorkflowNamespaceField(&Workflow{Namespace: "dockpipe"}); err == nil {
		t.Fatal("expected error")
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
	w := &Workflow{Steps: []Step{{Kind: "host"}, {Cmd: "echo x", Isolate: "alpine"}}}
	if !w.AnyContainerStep() {
		t.Fatal("expected AnyContainerStep true when one step uses the container")
	}
	w2 := &Workflow{Steps: []Step{{Kind: "host"}}}
	if w2.AnyContainerStep() {
		t.Fatal("expected AnyContainerStep false when all steps are host steps")
	}
}

func TestWorkflowNeedsDockerReachable(t *testing.T) {
	vscodeLike := &Workflow{Steps: []Step{{Kind: "host", Run: []string{"assets/scripts/vscode-session.sh"}}}}
	if !vscodeLike.NeedsDockerReachable() {
		t.Fatal("expected NeedsDockerReachable when host run: invokes docker")
	}
	hostOnly := &Workflow{Steps: []Step{{Kind: "host"}}}
	if hostOnly.NeedsDockerReachable() {
		t.Fatal("expected false when no container and no run scripts")
	}
	composeHostBuiltin := &Workflow{Steps: []Step{{Kind: "host", HostBuiltin: "compose_up"}}}
	if !composeHostBuiltin.NeedsDockerReachable() {
		t.Fatal("expected NeedsDockerReachable for compose host builtin")
	}
	withStepResolver := &Workflow{Steps: []Step{{Kind: "host", Resolver: "cursor"}}}
	if !withStepResolver.NeedsDockerReachable() {
		t.Fatal("expected NeedsDockerReachable when a step references a runtime profile name")
	}
	preflightOff := false
	hostRunNoDocker := &Workflow{DockerPreflight: &preflightOff, Steps: []Step{{Kind: "host", Run: []string{"scripts/print.sh"}}}}
	if hostRunNoDocker.NeedsDockerReachable() {
		t.Fatal("expected false when docker_preflight: false and no container steps")
	}
}

func TestParseWorkflowYAMLHostBuiltin(t *testing.T) {
	y := `
name: t
compose:
  file: assets/compose/docker-compose.yml
  autodown_env: DORKPIPE_DEV_STACK_AUTODOWN
  exports:
    OLLAMA_HOST: http://host.docker.internal:11434
steps:
  - kind: host
    host_builtin: compose_up
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 1 || w.Steps[0].HostBuiltin != "compose_up" || w.Steps[0].KindName() != "host" {
		t.Fatalf("got %+v", w.Steps[0])
	}
	if w.Compose.File != "assets/compose/docker-compose.yml" {
		t.Fatalf("unexpected compose config: %+v", w.Compose)
	}
	if w.Compose.AutodownEnv != "DORKPIPE_DEV_STACK_AUTODOWN" {
		t.Fatalf("unexpected compose autodown env: %+v", w.Compose)
	}
	if w.Compose.Exports["OLLAMA_HOST"] != "http://host.docker.internal:11434" {
		t.Fatalf("unexpected compose exports: %+v", w.Compose.Exports)
	}
	if err := ValidateLoadedWorkflow(w); err != nil {
		t.Fatal(err)
	}
}

func TestValidateStepHostBuiltinRejectsCombinedRun(t *testing.T) {
	s := Step{Kind: "host", HostBuiltin: "package_build_store", Run: []string{"x.sh"}}
	if err := ValidateStepHostBuiltin(0, s); err == nil {
		t.Fatal("expected error when host_builtin is combined with run:")
	}
}

func TestValidateStepHostBuiltinUnknown(t *testing.T) {
	s := Step{Kind: "host", HostBuiltin: "nope"}
	if err := ValidateStepHostBuiltin(0, s); err == nil {
		t.Fatal("expected error for unknown host_builtin")
	}
}

func TestValidateLoadedWorkflowRejectsComposeBuiltinWithoutComposeConfig(t *testing.T) {
	w := &Workflow{
		Steps: []Step{{Kind: "host", HostBuiltin: "compose_up"}},
	}
	if err := ValidateLoadedWorkflow(w); err == nil {
		t.Fatal("expected compose builtin validation error")
	}
}

func TestValidateLoadedWorkflowRejectsHostStepRuntimeFields(t *testing.T) {
	cases := []Step{
		{Kind: "host", Runtime: "dockerimage"},
		{Kind: "host", Resolver: "codex"},
		{Kind: "host", Isolate: "alpine:3.22"},
		{Kind: "host", Security: WorkflowSecurityConfig{Profile: "secure-default"}},
		{Kind: "host", VM: StepVMConfig{GuestPath: `C:\uh`}},
	}
	for _, step := range cases {
		w := &Workflow{Steps: []Step{step}}
		if err := ValidateLoadedWorkflow(w); err == nil {
			t.Fatalf("expected host-step validation error for %+v", step)
		}
	}
}

func TestValidateLoadedWorkflowRejectsVMHostContextWithoutGuestPath(t *testing.T) {
	w := &Workflow{
		Steps: []Step{{
			Runtime:  "vm",
			Resolver: "qemu",
			VM:       StepVMConfig{HostContext: `C:\src\repo`},
			Cmd:      "hostname",
		}},
	}
	if err := ValidateLoadedWorkflow(w); err == nil || !strings.Contains(err.Error(), "vm.host_context requires vm.guest_path") {
		t.Fatalf("expected vm.host_context validation error, got %v", err)
	}
}

func TestValidateLoadedWorkflowRejectsIncompleteVMMount(t *testing.T) {
	w := &Workflow{
		Steps: []Step{{
			Runtime:  "vm",
			Resolver: "qemu",
			VM: StepVMConfig{
				Mounts: []StepVMMount{{Host: `C:\src\repo`}},
			},
			Cmd: "hostname",
		}},
	}
	if err := ValidateLoadedWorkflow(w); err == nil || !strings.Contains(err.Error(), "vm.mounts[0] requires both host and guest") {
		t.Fatalf("expected vm.mounts validation error, got %v", err)
	}
}

func TestValidateLoadedWorkflowRejectsPackagedWorkflowStepCmd(t *testing.T) {
	w := &Workflow{
		Steps: []Step{{
			WorkflowName: "child",
			Package:      "acme",
			Cmd:          "echo nope",
		}},
	}
	if err := ValidateLoadedWorkflow(w); err == nil || !strings.Contains(err.Error(), "do not also set cmd/command") {
		t.Fatalf("expected packaged workflow cmd validation error, got %v", err)
	}
}

func TestValidateLoadedWorkflowRejectsPackagedWorkflowStepSecurity(t *testing.T) {
	w := &Workflow{
		Steps: []Step{{
			WorkflowName: "child",
			Package:      "acme",
			Security:     WorkflowSecurityConfig{Profile: "sidecar-client"},
		}},
	}
	if err := ValidateLoadedWorkflow(w); err == nil || !strings.Contains(err.Error(), "do not also set security") {
		t.Fatalf("expected packaged workflow security validation error, got %v", err)
	}
}

func TestValidateLoadedWorkflowRejectsTopLevelSingleFlowFieldsWithSteps(t *testing.T) {
	cases := []struct {
		name string
		wf   *Workflow
		want string
	}{
		{
			name: "run",
			wf: &Workflow{
				Run:   RunSpec{"scripts/setup.sh"},
				Steps: []Step{{Cmd: "echo hi"}},
			},
			want: "top-level run",
		},
		{
			name: "act",
			wf: &Workflow{
				Act:   "scripts/after.sh",
				Steps: []Step{{Cmd: "echo hi"}},
			},
			want: "top-level act/action",
		},
		{
			name: "action",
			wf: &Workflow{
				Action: "scripts/after.sh",
				Steps:  []Step{{Cmd: "echo hi"}},
			},
			want: "top-level act/action",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLoadedWorkflow(tc.wf)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q validation error, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateLoadedWorkflowRejectsFinallyWithoutSteps(t *testing.T) {
	w := &Workflow{
		Finally: []Step{{Kind: "host", Run: []string{"scripts/cleanup.sh"}}},
	}
	err := ValidateLoadedWorkflow(w)
	if err == nil || !strings.Contains(err.Error(), "workflow with finally: requires at least one main step") {
		t.Fatalf("expected finally without steps validation error, got %v", err)
	}
}
