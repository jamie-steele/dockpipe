package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func withRunStepsSeams(t *testing.T) {
	t.Helper()
	oldBuild := dockerBuildFn
	oldImageExists := dockerImageExistsFn
	oldCompose := composeLifecycleFn
	oldRun := runContainerFn
	oldSource := sourceHostScriptFn
	oldRunHost := runHostScriptFn
	oldRunHostCmd := runHostCommandFn
	oldStat := osStatFn
	oldGetwd := getwdFn
	t.Cleanup(func() {
		dockerBuildFn = oldBuild
		dockerImageExistsFn = oldImageExists
		composeLifecycleFn = oldCompose
		runContainerFn = oldRun
		sourceHostScriptFn = oldSource
		runHostScriptFn = oldRunHost
		runHostCommandFn = oldRunHostCmd
		osStatFn = oldStat
		getwdFn = oldGetwd
	})
}

func baseRunStepsOpts() runStepsOpts {
	return runStepsOpts{
		wf:       &domain.Workflow{},
		wfRoot:   "/wf",
		repoRoot: "/repo",
		envMap:   map[string]string{},
		envSlice: nil,
		locked:   map[string]bool{},
		opts:     &CliOpts{},
		dataVol:  "dockpipe-data",
	}
}

func TestApplyWorkflowContainerMountEnvResolvesMounts(t *testing.T) {
	wd := t.TempDir()
	o := baseRunStepsOpts()
	o.projectRoot = wd
	o.repoRoot = wd
	o.envMap = map[string]string{"DOCKPIPE_WORKDIR": wd}
	o.wf.Container.Mounts = []domain.WorkflowContainerMount{
		{Host: "../shared", Guest: "/shared", Mode: "ro"},
		{Host: "cache", Guest: "/cache", Mode: "rw"},
	}
	if err := applyWorkflowContainerMountEnv(&o); err != nil {
		t.Fatalf("applyWorkflowContainerMountEnv: %v", err)
	}
	want := filepath.Clean(filepath.Join(wd, "..", "shared")) + ":/shared:ro\n" +
		filepath.Clean(filepath.Join(wd, "cache")) + ":/cache:rw"
	if got := o.envMap["DOCKPIPE_CONTAINER_MOUNTS"]; got != want {
		t.Fatalf("DOCKPIPE_CONTAINER_MOUNTS=%q want %q", got, want)
	}
}

// TestRunBlockingStepHostMergesOutputs loads outputs.env into env for a blocking host step.
func TestRunBlockingStepHostMergesOutputs(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	out := filepath.Join(wd, infrastructure.DockpipeDirRel, "workflows", "ci", "artifacts", domain.DefaultOutputsEnvRel)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := baseRunStepsOpts()
	o.opts.Workdir = wd
	o.envMap["DOCKPIPE_WORKFLOW_NAME"] = "ci"
	o.wf.Name = "ci"
	o.wf.Steps = []domain.Step{{Kind: "host", Outputs: domain.DefaultOutputsEnvRel}}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if o.envMap["A"] != "1" || dockerEnv["A"] != "1" {
		t.Fatalf("expected merged outputs, env=%#v docker=%#v", o.envMap, dockerEnv)
	}
}

func TestRunBlockingStepHostCommandRunsOnHost(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	o := baseRunStepsOpts()
	o.opts.Workdir = wd
	o.envMap["DOCKPIPE_WORKFLOW_NAME"] = "ci"
	o.wf.Name = "ci"
	o.wf.Steps = []domain.Step{{Kind: "host", CWD: "artifacts", Cmd: "echo host"}}
	called := false
	runHostCommandFn = func(cmd string, env []string) error {
		called = true
		if cmd != "echo host" {
			t.Fatalf("unexpected host cmd %q", cmd)
		}
		joined := strings.Join(env, "\n")
		wantArtifact := filepath.Join(wd, "bin", ".dockpipe", "workflows", "ci", "artifacts")
		if !strings.Contains(joined, "DOCKPIPE_SOURCE_ROOT="+wd) {
			t.Fatalf("missing source root in env:\n%s", joined)
		}
		if !strings.Contains(joined, "DOCKPIPE_ARTIFACT_ROOT="+wantArtifact) || !strings.Contains(joined, "DOCKPIPE_STEP_CWD="+wantArtifact) {
			t.Fatalf("missing artifact cwd env %q in:\n%s", wantArtifact, joined)
		}
		if !strings.Contains(joined, "DOCKPIPE_OUTPUT_ROOT="+wantArtifact) {
			t.Fatalf("missing output root env %q in:\n%s", wantArtifact, joined)
		}
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if !called {
		t.Fatal("expected host command execution")
	}
}

func TestRunBlockingStepContainerArtifactsCWDUsesWorkPath(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	o := baseRunStepsOpts()
	o.projectRoot = wd
	o.repoRoot = wd
	o.opts.Workdir = wd
	o.envMap["DOCKPIPE_WORKFLOW_NAME"] = "ci"
	o.wf.Name = "ci"
	o.wf.Steps = []domain.Step{{CWD: "artifacts", Isolate: "alpine", Cmd: "true"}}

	var got infrastructure.RunOpts
	runContainerFn = func(opts infrastructure.RunOpts, argv []string) (int, error) {
		got = opts
		if len(argv) != 1 || argv[0] != "true" {
			t.Fatalf("argv = %#v", argv)
		}
		return 0, nil
	}

	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if got.WorkdirHost != wd {
		t.Fatalf("WorkdirHost = %q want %q", got.WorkdirHost, wd)
	}
	wantWorkPath := filepath.ToSlash(filepath.Join("bin", ".dockpipe", "workflows", "ci", "artifacts"))
	if got.WorkPath != wantWorkPath {
		t.Fatalf("WorkPath = %q want %q", got.WorkPath, wantWorkPath)
	}
}

func TestRunBlockingStepHostIsolateGetsProfileEnvAndStepCommand(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	repo := t.TempDir()
	profileDir := filepath.Join(repo, "src", "core", "runtimes", "vmimage")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "DOCKPIPE_RUNTIME_SUBSTRATE=vmimage\nDOCKPIPE_RUNTIME_HOST_SCRIPT=scripts/core.assets.scripts.vmimage-run.sh\nDOCKPIPE_RUNTIME_HOST_REQUIRES_DOCKER=0\n"
	if err := os.WriteFile(filepath.Join(profileDir, "profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptAbs := filepath.Join(repo, "src", "core", "assets", "scripts", "vmimage-run.sh")
	if err := os.MkdirAll(filepath.Dir(scriptAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptAbs, []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := baseRunStepsOpts()
	o.repoRoot = repo
	o.projectRoot = repo
	o.opts.Workdir = wd
	o.envMap["DOCKPIPE_WORKFLOW_NAME"] = "vm"
	o.wf.Name = "vm"
	o.wf.Steps = []domain.Step{{Runtime: "vmimage", Cmd: "echo hi"}}
	var gotEnv []string
	runHostScriptFn = func(scriptPath string, env []string) error {
		gotEnv = append([]string(nil), env...)
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	joined := strings.Join(gotEnv, "\n")
	if !strings.Contains(joined, "DOCKPIPE_RUNTIME_SUBSTRATE=vmimage") {
		t.Fatalf("expected runtime substrate in env, got %q", joined)
	}
	if !strings.Contains(joined, "DOCKPIPE_STEP_CMD=echo hi") {
		t.Fatalf("expected step command in env, got %q", joined)
	}
	wantOutputs := filepath.Join(wd, infrastructure.DockpipeDirRel, "workflows", "vm", "artifacts", domain.DefaultOutputsEnvRel)
	if !strings.Contains(joined, "DOCKPIPE_STEP_OUTPUTS_FILE="+wantOutputs) {
		t.Fatalf("expected outputs file in env, got %q", joined)
	}
}

func TestRunBlockingStepScopesKeepRepoCWDAndArtifactOutputs(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	o := baseRunStepsOpts()
	o.projectRoot = wd
	o.repoRoot = wd
	o.opts.Workdir = wd
	o.envMap["DOCKPIPE_WORKFLOW_NAME"] = "doctor"
	o.wf.Name = "doctor"
	o.wf.Steps = []domain.Step{{
		Kind:    "host",
		CWD:     "repo",
		Scopes:  domain.StepScopes{Source: "repo", Artifacts: "artifacts"},
		Outputs: "doctor.env",
		Cmd:     "echo host",
	}}

	called := false
	runHostCommandFn = func(cmd string, env []string) error {
		called = true
		joined := strings.Join(env, "\n")
		wantArtifact := filepath.Join(wd, infrastructure.DockpipeDirRel, "workflows", "doctor", "artifacts")
		if !strings.Contains(joined, "DOCKPIPE_SOURCE_ROOT="+wd) {
			t.Fatalf("missing source root in env:\n%s", joined)
		}
		if !strings.Contains(joined, "DOCKPIPE_STEP_CWD="+wd) {
			t.Fatalf("missing repo step cwd in env:\n%s", joined)
		}
		if !strings.Contains(joined, "DOCKPIPE_ARTIFACT_ROOT="+wantArtifact) || !strings.Contains(joined, "DOCKPIPE_OUTPUT_ROOT="+wantArtifact) {
			t.Fatalf("missing artifact/output root %q in env:\n%s", wantArtifact, joined)
		}
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if !called {
		t.Fatal("expected host command execution")
	}
	wantOutputs := filepath.Join(wd, infrastructure.DockpipeDirRel, "workflows", "doctor", "artifacts", "doctor.env")
	if got := stepOutputsAbsPath(&o, o.wf.Steps[0], o.envMap); got != wantOutputs {
		t.Fatalf("outputs path = %q want %q", got, wantOutputs)
	}
}

func TestRunBlockingStepHostIsolateAllowsRawNonShellCommandString(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	repo := t.TempDir()
	profileDir := filepath.Join(repo, "src", "core", "runtimes", "vmimage")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "DOCKPIPE_RUNTIME_SUBSTRATE=vmimage\nDOCKPIPE_RUNTIME_HOST_SCRIPT=scripts/core.assets.scripts.vmimage-run.sh\nDOCKPIPE_RUNTIME_HOST_REQUIRES_DOCKER=0\n"
	if err := os.WriteFile(filepath.Join(profileDir, "profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptAbs := filepath.Join(repo, "src", "core", "assets", "scripts", "vmimage-run.sh")
	if err := os.MkdirAll(filepath.Dir(scriptAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptAbs, []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := baseRunStepsOpts()
	o.repoRoot = repo
	o.projectRoot = repo
	o.opts.Workdir = wd
	o.wf.Steps = []domain.Step{{
		Runtime: "vmimage",
		Cmd:     `if ($env:DOCKPIPE_VM_GUEST_COMMAND) { Invoke-Expression $env:DOCKPIPE_VM_GUEST_COMMAND } else { whoami; hostname }`,
	}}
	var gotEnv []string
	runHostScriptFn = func(scriptPath string, env []string) error {
		gotEnv = append([]string(nil), env...)
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	joined := strings.Join(gotEnv, "\n")
	if !strings.Contains(joined, `DOCKPIPE_STEP_CMD=if ($env:DOCKPIPE_VM_GUEST_COMMAND) { Invoke-Expression $env:DOCKPIPE_VM_GUEST_COMMAND } else { whoami; hostname }`) {
		t.Fatalf("expected raw step command in env, got %q", joined)
	}
}

func TestRunBlockingStepInheritsWorkflowRuntimeForHostIsolate(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	repo := t.TempDir()
	profileDir := filepath.Join(repo, "src", "core", "runtimes", "vmimage")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "DOCKPIPE_RUNTIME_SUBSTRATE=vmimage\nDOCKPIPE_RUNTIME_HOST_SCRIPT=scripts/core.assets.scripts.vmimage-run.sh\nDOCKPIPE_RUNTIME_HOST_REQUIRES_DOCKER=0\n"
	if err := os.WriteFile(filepath.Join(profileDir, "profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptAbs := filepath.Join(repo, "src", "core", "assets", "scripts", "vmimage-run.sh")
	if err := os.MkdirAll(filepath.Dir(scriptAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptAbs, []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := baseRunStepsOpts()
	o.repoRoot = repo
	o.projectRoot = repo
	o.opts.Workdir = wd
	o.wf.Runtime = "vmimage"
	o.wf.Steps = []domain.Step{{
		Cmd: `if ($env:DOCKPIPE_VM_GUEST_COMMAND) { Invoke-Expression $env:DOCKPIPE_VM_GUEST_COMMAND } else { whoami; hostname }`,
	}}
	var calledHostScript bool
	runHostScriptFn = func(scriptPath string, env []string) error {
		calledHostScript = true
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if !calledHostScript {
		t.Fatal("expected workflow runtime default to resolve to host isolate script")
	}
}

func TestMergeStepVarsAppliesVMOverrides(t *testing.T) {
	o := baseRunStepsOpts()
	o.repoRoot = `C:\repo`
	o.projectRoot = `C:\repo`
	o.opts.Workdir = `C:\repo`
	o.envMap["DOCKPIPE_WORKDIR"] = `C:\repo`
	step := domain.Step{
		VM: domain.StepVMConfig{
			GuestPath: `C:\uh`,
			InteractiveDebug: func() *bool {
				v := true
				return &v
			}(),
			InteractiveSSH: func() *bool {
				v := true
				return &v
			}(),
			KeepAliveSeconds: "28800",
			HostFwd:          "tcp::3389-:3389",
			KeepAlive: func() *bool {
				v := true
				return &v
			}(),
		},
	}
	dockerEnv := map[string]string{}
	if err := mergeStepVars(&o, step, dockerEnv); err != nil {
		t.Fatal(err)
	}
	if got := o.envMap["DOCKPIPE_VM_SYNC_HOST_PATH"]; got != `C:\repo` {
		t.Fatalf("DOCKPIPE_VM_SYNC_HOST_PATH=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_SYNC_GUEST_PATH"]; got != `C:\uh` {
		t.Fatalf("DOCKPIPE_VM_SYNC_GUEST_PATH=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_MOUNTS"]; got != "C:\\repo\tC:\\uh" {
		t.Fatalf("DOCKPIPE_VM_MOUNTS=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_INTERACTIVE"]; got != "true" {
		t.Fatalf("DOCKPIPE_VM_INTERACTIVE=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_INTERACTIVE_SSH"]; got != "true" {
		t.Fatalf("DOCKPIPE_VM_INTERACTIVE_SSH=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_KEEPALIVE"]; got != "true" {
		t.Fatalf("DOCKPIPE_VM_KEEPALIVE=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_KEEPALIVE_SECONDS"]; got != "28800" {
		t.Fatalf("DOCKPIPE_VM_KEEPALIVE_SECONDS=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_HOSTFWD"]; got != "tcp::3389-:3389" {
		t.Fatalf("DOCKPIPE_VM_HOSTFWD=%q", got)
	}
	if dockerEnv["DOCKPIPE_VM_SYNC_GUEST_PATH"] != `C:\uh` {
		t.Fatalf("docker env missing guest path: %#v", dockerEnv)
	}
}

func TestMergeStepVarsVMMultipleMounts(t *testing.T) {
	o := baseRunStepsOpts()
	step := domain.Step{
		VM: domain.StepVMConfig{
			Mounts: []domain.StepVMMount{
				{Host: `C:\src\repo`, Guest: `C:\uh`},
				{Host: `C:\tmp\artifacts`, Guest: `C:\artifacts`},
			},
		},
	}
	dockerEnv := map[string]string{}
	if err := mergeStepVars(&o, step, dockerEnv); err != nil {
		t.Fatal(err)
	}
	want := "C:\\src\\repo\tC:\\uh\nC:\\tmp\\artifacts\tC:\\artifacts"
	if got := o.envMap["DOCKPIPE_VM_MOUNTS"]; got != want {
		t.Fatalf("DOCKPIPE_VM_MOUNTS=%q want %q", got, want)
	}
	if dockerEnv["DOCKPIPE_VM_MOUNTS"] != want {
		t.Fatalf("docker env missing mounts: %#v", dockerEnv)
	}
}

func TestMergeStepVarsVMHostContextOverrideWins(t *testing.T) {
	o := baseRunStepsOpts()
	o.repoRoot = `C:\repo`
	o.projectRoot = `C:\repo`
	o.opts.Workdir = `C:\repo`
	o.envMap["DOCKPIPE_WORKDIR"] = `C:\repo`
	step := domain.Step{
		VM: domain.StepVMConfig{
			HostContext: `C:\other`,
			GuestPath:   `C:\uh`,
		},
	}
	dockerEnv := map[string]string{}
	if err := mergeStepVars(&o, step, dockerEnv); err != nil {
		t.Fatal(err)
	}
	if got := o.envMap["DOCKPIPE_VM_SYNC_HOST_PATH"]; got != `C:\other` {
		t.Fatalf("DOCKPIPE_VM_SYNC_HOST_PATH=%q", got)
	}
}

func TestMergeStepVarsAppliesTypedInputs(t *testing.T) {
	wfRoot := t.TempDir()
	modelsDir := filepath.Join(wfRoot, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(wfRoot, "config.yml"), "name: demo\n")
	write(filepath.Join(modelsDir, "QemuVmResolverConfig.pipe"), `public Class QemuVmResolverConfig
{
    public WindowsVmGeneral General;
    public WindowsVmAdvanced Advanced;
}
`)
	write(filepath.Join(modelsDir, "WindowsVmGeneral.pipe"), `public Class WindowsVmGeneral
{
    [EnvName = "DOCKPIPE_VM_EXEC_MODE"]
    public string ExecMode = "raw";
}
`)
	write(filepath.Join(modelsDir, "WindowsVmAdvanced.pipe"), `public Class WindowsVmAdvanced
{
    [EnvName = "DOCKPIPE_VM_KEEPALIVE"]
    public string KeepAlive = "";
}
`)
	o := baseRunStepsOpts()
	o.wfRoot = wfRoot
	o.wf.Types = []string{"models/QemuVmResolverConfig.pipe"}
	o.envMap["UH_VM_KEEPALIVE"] = "true"
	step := domain.Step{
		Inputs: map[string]domain.InputBinding{
			"General.ExecMode":   {Value: "powershell"},
			"Advanced.KeepAlive": {From: "UH_VM_KEEPALIVE", Value: "false"},
		},
	}
	dockerEnv := map[string]string{}
	if err := mergeStepVars(&o, step, dockerEnv); err != nil {
		t.Fatal(err)
	}
	if got := o.envMap["DOCKPIPE_VM_EXEC_MODE"]; got != "powershell" {
		t.Fatalf("DOCKPIPE_VM_EXEC_MODE=%q", got)
	}
	if got := o.envMap["DOCKPIPE_VM_KEEPALIVE"]; got != "true" {
		t.Fatalf("DOCKPIPE_VM_KEEPALIVE=%q", got)
	}
}

func TestRunBlockingStepHostIsolateReappliesWorkflowInputsFromWorkflowVars(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	repo := t.TempDir()
	profileDir := filepath.Join(repo, "src", "core", "runtimes", "vm")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "DOCKPIPE_RUNTIME_SUBSTRATE=vmimage\nDOCKPIPE_RUNTIME_HOST_SCRIPT=scripts/core.assets.scripts.vmimage-run.sh\nDOCKPIPE_RUNTIME_HOST_REQUIRES_DOCKER=0\n"
	if err := os.WriteFile(filepath.Join(profileDir, "profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptAbs := filepath.Join(repo, "src", "core", "assets", "scripts", "vmimage-run.sh")
	if err := os.MkdirAll(filepath.Dir(scriptAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptAbs, []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wfRoot := filepath.Join(repo, "workflows", "uh-vm")
	modelsDir := filepath.Join(repo, "packages", "vm", "resolvers", "qemu", "models")
	for _, dir := range []string{wfRoot, modelsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(repo, "dockpipe.config.json"), `{
  "compile": {
    "workflows": ["workflows", "packages/vm"]
  }
}`)
	write(filepath.Join(repo, "packages", "vm", "resolvers", "qemu", "profile"), "DOCKPIPE_RESOLVER_VM_BACKEND=auto\n")
	write(filepath.Join(repo, "packages", "vm", "resolvers", "qemu", "types.yml"), "types:\n  - models/QemuVmResolverConfig.pipe\n")
	write(filepath.Join(modelsDir, "QemuVmResolverConfig.pipe"), `public Class QemuVmResolverConfig
{
    public WindowsVmGeneral General;
    public WindowsVmStorage Storage;
}
`)
	write(filepath.Join(modelsDir, "WindowsVmGeneral.pipe"), `public Class WindowsVmGeneral
{
    [EnvName = "DOCKPIPE_VM_BOOT_SOURCE"]
    public string BootSource = "";
}
`)
	write(filepath.Join(modelsDir, "WindowsVmStorage.pipe"), `public Class WindowsVmStorage
{
    [EnvName = "DOCKPIPE_VM_DISK"]
    public string Disk = "";
}
`)
	write(filepath.Join(wfRoot, "config.yml"), "name: uh-vm\nruntime: vm\nresolver: qemu\n")

	o := baseRunStepsOpts()
	o.repoRoot = repo
	o.projectRoot = repo
	o.wfRoot = wfRoot
	o.wfConfig = filepath.Join(wfRoot, "config.yml")
	o.opts.Workdir = wd
	o.wf.Runtime = "vm"
	o.wf.Resolver = "qemu"
	o.wf.Vars = map[string]string{
		"DOCKPIPE_UH_VM_DISK": `C:\vm\win10.qcow2`,
	}
	o.wf.Inputs = map[string]domain.InputBinding{
		"General.BootSource": {Value: "image"},
		"Storage.Disk":       {From: "DOCKPIPE_UH_VM_DISK"},
	}
	o.wf.Steps = []domain.Step{{Cmd: "hostname"}}

	if err := buildWorkflowEnvInto(o.envMap, o.wf, o.wfConfig, o.wfRoot, o.repoRoot, &CliOpts{Workdir: repo}); err != nil {
		t.Fatal(err)
	}
	o.envSlice = domain.EnvMapToSlice(o.envMap)

	var gotEnv []string
	runHostScriptFn = func(scriptPath string, env []string) error {
		gotEnv = append([]string(nil), env...)
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	joined := strings.Join(gotEnv, "\n")
	if !strings.Contains(joined, "DOCKPIPE_VM_BOOT_SOURCE=image") {
		t.Fatalf("expected DOCKPIPE_VM_BOOT_SOURCE in host isolate env, got %q", joined)
	}
	if !strings.Contains(joined, `DOCKPIPE_VM_DISK=C:\vm\win10.qcow2`) {
		t.Fatalf("expected DOCKPIPE_VM_DISK in host isolate env, got %q", joined)
	}
}

// TestRunBlockingStepBuildAndRun builds the isolate image if needed and runs the container command.
func TestRunBlockingStepBuildAndRun(t *testing.T) {
	withRunStepsSeams(t)
	o := baseRunStepsOpts()
	wd := t.TempDir()
	o.projectRoot = wd
	o.repoRoot = wd
	o.wf.Steps = []domain.Step{{Cmd: "echo hi", Isolate: "codex"}}
	built := false
	dockerBuildFn = func(image, dockerfileDir, contextDir string) error {
		built = true
		return nil
	}
	ran := false
	runContainerFn = func(ro infrastructure.RunOpts, argv []string) (int, error) {
		ran = true
		if len(argv) != 2 || argv[0] != "echo" || argv[1] != "hi" {
			t.Fatalf("unexpected argv: %#v", argv)
		}
		return 0, nil
	}
	getwdFn = func() (string, error) { return wd, nil }
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if !built || !ran {
		t.Fatalf("expected build + run, built=%v ran=%v", built, ran)
	}
}

func TestRunBlockingStepPackageWorkflowUsesWorkflowField(t *testing.T) {
	withRunStepsSeams(t)
	repo := t.TempDir()
	projectCfg := `{"schema":1,"compile":{"workflows":["vendor/my-workflows"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(projectCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	childDir := filepath.Join(repo, "vendor", "my-workflows", "nested-flow")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	childCfg := `name: nested-flow
namespace: dockpipe-demo
steps:
  - id: nested-host
    kind: host
    cmd: echo nested
`
	if err := os.WriteFile(filepath.Join(childDir, "config.yml"), []byte(childCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	o := baseRunStepsOpts()
	o.repoRoot = repo
	o.projectRoot = repo
	o.opts.Workdir = repo
	o.wf.Steps = []domain.Step{{
		WorkflowName: "nested-flow",
		Package:      "dockpipe-demo",
	}}
	getwdFn = func() (string, error) { return repo, nil }
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
}

func TestRunBlockingStepSkipsBuildWhenCompiledImageArtifactExists(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	wfRoot := filepath.Join(wd, "wf")
	if err := os.MkdirAll(filepath.Join(wfRoot, domain.RuntimeManifestDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	o := baseRunStepsOpts()
	o.repoRoot = wd
	o.projectRoot = wd
	o.wfRoot = wfRoot
	o.wf.Steps = []domain.Step{{Cmd: "echo hi", Isolate: "codex"}}
	if err := os.MkdirAll(filepath.Join(wd, "templates", "core", "assets", "images", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "templates", "core", "assets", "images", "codex", "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	policyFingerprint, err := defaultRuntimePolicyFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := buildImageArtifactManifest(wd, "", "", "codex", "dockpipe-codex", filepath.Join(wd, "templates", "core", "assets", "images", "codex"), wd, policyFingerprint, domain.ImageArtifactProvenance{Isolate: "codex", DockpipeVersion: authoredPackageVersion(wd)})
	if err != nil {
		t.Fatal(err)
	}
	b, err := marshalArtifactJSON(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.ImageArtifactFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	indexed := *artifact
	indexed.ArtifactState = "materialized"
	if err := persistImageArtifactIndexRecord(wd, &indexed); err != nil {
		t.Fatal(err)
	}
	rm := &domain.CompiledRuntimeManifest{
		Schema:            2,
		Kind:              domain.RuntimeManifestKind,
		PolicyProfile:     "secure-default",
		PolicyFingerprint: policyFingerprint,
		Security: domain.CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "offline",
				Enforcement: "native",
				InternalDNS: true,
			},
			FS: domain.CompiledFilesystemPolicy{
				Root:      "readonly",
				Writes:    "workspace-only",
				TempPaths: []string{"/tmp"},
			},
			Process: domain.CompiledProcessPolicy{
				User:            "non-root",
				NoNewPrivileges: true,
				DropCaps:        []string{"ALL"},
				PIDLimit:        256,
			},
		},
	}
	rmb, err := marshalArtifactJSON(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.RuntimeManifestFileName), rmb, 0o644); err != nil {
		t.Fatal(err)
	}
	dockerImageExistsFn = func(image string) (bool, error) { return true, nil }
	dockerBuildFn = func(image, dockerfileDir, contextDir string) error {
		t.Fatalf("docker build should have been skipped")
		return nil
	}
	ran := false
	runContainerFn = func(ro infrastructure.RunOpts, argv []string) (int, error) {
		ran = true
		return 0, nil
	}
	getwdFn = func() (string, error) { return wd, nil }
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if !ran {
		t.Fatal("expected container run")
	}
	ents, err := os.ReadDir(infrastructure.RunPolicyRecordsDir(wd))
	if err != nil {
		t.Fatalf("expected step run policy record: %v", err)
	}
	if len(ents) == 0 {
		t.Fatal("expected at least one step run policy record")
	}
	brec, err := os.ReadFile(filepath.Join(infrastructure.RunPolicyRecordsDir(wd), ents[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var rec infrastructure.RunPolicyRecord
	if err := json.Unmarshal(brec, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.StepID != "step-1" || rec.ImageArtifactDecision == "" {
		t.Fatalf("unexpected step run policy record: %+v", rec)
	}
	if !strings.Contains(rec.ImageArtifactDecision, "image: ready materialized image artifact") {
		t.Fatalf("expected materialized image decision, got %+v", rec)
	}
}

func TestMaybeSkipDockerBuildRejectsPolicyFingerprintMismatch(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	wfRoot := filepath.Join(wd, "wf")
	if err := os.MkdirAll(filepath.Join(wfRoot, domain.RuntimeManifestDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wd, "templates", "core", "assets", "images", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "templates", "core", "assets", "images", "codex", "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact, err := buildImageArtifactManifest(wd, "", "", "codex", "dockpipe-codex", filepath.Join(wd, "templates", "core", "assets", "images", "codex"), wd, "sha256:oldpolicy", domain.ImageArtifactProvenance{Isolate: "codex", DockpipeVersion: authoredPackageVersion(wd)})
	if err != nil {
		t.Fatal(err)
	}
	b, err := marshalArtifactJSON(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.ImageArtifactFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	rm := &domain.CompiledRuntimeManifest{
		Schema:        2,
		Kind:          domain.RuntimeManifestKind,
		PolicyProfile: "secure-default",
		Security: domain.CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "offline",
				Enforcement: "native",
				InternalDNS: true,
			},
			FS: domain.CompiledFilesystemPolicy{
				Root:      "readonly",
				Writes:    "workspace-only",
				TempPaths: []string{"/tmp"},
			},
			Process: domain.CompiledProcessPolicy{
				User:            "non-root",
				NoNewPrivileges: true,
				DropCaps:        []string{"ALL"},
				PIDLimit:        999,
			},
		},
	}
	rm.PolicyFingerprint, err = domain.FingerprintJSON(rm.Security)
	if err != nil {
		t.Fatal(err)
	}
	rmb, err := marshalArtifactJSON(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.RuntimeManifestFileName), rmb, 0o644); err != nil {
		t.Fatal(err)
	}
	dockerImageExistsFn = func(image string) (bool, error) { return true, nil }
	skip, _, err := maybeSkipDockerBuildForStep(wd, wd, "", wfRoot, "", "", "dockpipe-codex", filepath.Join(wd, "templates", "core", "assets", "images", "codex"), wd)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("expected policy fingerprint mismatch to disable cache reuse")
	}
}

func TestMaybeSkipDockerBuildUsesMaterializedImageIndex(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	wfRoot := filepath.Join(wd, "wf")
	if err := os.MkdirAll(filepath.Join(wfRoot, domain.RuntimeManifestDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	buildDir := filepath.Join(wd, "templates", "core", "assets", "images", "codex")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	policyFingerprint, err := defaultRuntimePolicyFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := buildImageArtifactManifest(wd, "", "", "codex", "dockpipe-codex", buildDir, wd, policyFingerprint, domain.ImageArtifactProvenance{Isolate: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := marshalArtifactJSON(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.ImageArtifactFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	indexed := *artifact
	indexed.ArtifactState = "materialized"
	if err := persistImageArtifactIndexRecord(wd, &indexed); err != nil {
		t.Fatal(err)
	}
	dockerImageExistsFn = func(image string) (bool, error) {
		if image != "dockpipe-codex" {
			t.Fatalf("unexpected image exists check %q", image)
		}
		return true, nil
	}
	skip, msg, err := maybeSkipDockerBuildForStep(wd, wd, "", wfRoot, "", policyFingerprint, "dockpipe-codex", buildDir, wd)
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Fatal("expected materialized image index to skip docker build")
	}
	if !strings.Contains(msg, "image: ready materialized image artifact") {
		t.Fatalf("expected materialized image decision, got %q", msg)
	}
}

// TestPrefetchDockerBuildsForBatchDedupes builds each distinct isolate image once per async batch.
func TestPrefetchDockerBuildsForBatchDedupes(t *testing.T) {
	withRunStepsSeams(t)
	o := baseRunStepsOpts()
	bFalse := false
	o.wf.Steps = []domain.Step{
		{Cmd: "echo a", Isolate: "codex", Blocking: &bFalse},
		{Cmd: "echo b", Isolate: "codex", Blocking: &bFalse},
	}
	count := 0
	dockerBuildFn = func(image, dockerfileDir, contextDir string) error {
		count++
		return nil
	}
	if err := prefetchDockerBuildsForBatch(&o, 0, 2, 2, map[string]string{}, map[string]string{}); err != nil {
		t.Fatalf("prefetchDockerBuildsForBatch error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one deduped build call, got %d", count)
	}
}

// TestRunParallelStepWorkerNonZeroExit surfaces container non-zero exit as a parallel-step error.
func TestRunParallelStepWorkerNonZeroExit(t *testing.T) {
	withRunStepsSeams(t)
	o := baseRunStepsOpts()
	wd := t.TempDir()
	o.projectRoot = wd
	o.repoRoot = wd
	bFalse := false
	o.wf.Steps = []domain.Step{{Cmd: "echo a", Isolate: "ubuntu:latest", Blocking: &bFalse}}
	runContainerFn = func(ro infrastructure.RunOpts, argv []string) (int, error) { return 3, nil }
	err := runParallelStepWorker(&o, 0, 1, 0, map[string]string{}, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "parallel step 1 exited with code 3") {
		t.Fatalf("expected non-zero worker error, got %v", err)
	}
}

// TestRunParallelStepWorkerFirstStepExtraPreScript runs workflow-level extra pre-scripts on the first parallel step only.
func TestRunParallelStepWorkerFirstStepExtraPreScript(t *testing.T) {
	withRunStepsSeams(t)
	pre := filepath.Join(t.TempDir(), "pre.sh")
	if err := os.WriteFile(pre, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	o := baseRunStepsOpts()
	bFalse := false
	o.firstStepExtra = []string{pre}
	o.wf.Steps = []domain.Step{{Kind: "host", Blocking: &bFalse}}
	osStatFn = func(name string) (os.FileInfo, error) {
		return os.Stat(name)
	}
	called := false
	runHostScriptFn = func(scriptPath string, env []string) error {
		called = true
		if scriptPath != pre {
			t.Fatalf("unexpected script path %q want %q", scriptPath, pre)
		}
		return nil
	}
	if err := runParallelStepWorker(&o, 0, 1, 0, map[string]string{}, map[string]string{}); err != nil {
		t.Fatalf("runParallelStepWorker error: %v", err)
	}
	if !called {
		t.Fatal("expected host exec for kind: host parallel pre-script")
	}
}

func TestRunBlockingStepComposeHostBuiltin(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	wfRoot := filepath.Join(wd, "workflows", "stack")
	if err := os.MkdirAll(filepath.Join(wfRoot, "assets", "compose"), 0o755); err != nil {
		t.Fatal(err)
	}
	o := baseRunStepsOpts()
	o.opts.Workdir = wd
	o.wfRoot = wfRoot
	o.wf.Compose = domain.WorkflowComposeConfig{
		File:             "assets/compose/docker-compose.yml",
		Project:          "dockpipe-dev",
		ProjectDirectory: "../../..",
		Exports: map[string]string{
			"OLLAMA_HOST": "http://host.docker.internal:11434",
		},
		Services: []string{"proxy"},
	}
	o.envMap["MCP_HTTP_URL"] = "http://127.0.0.1:8766"
	o.envMap["DATABASE_URL"] = "postgres://local"
	o.wf.Steps = []domain.Step{{Kind: "host", HostBuiltin: "compose_up"}}
	var got infrastructure.ComposeLifecycleOpts
	composeLifecycleFn = func(opts infrastructure.ComposeLifecycleOpts) error {
		got = opts
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if got.Action != "up" {
		t.Fatalf("expected compose up action, got %+v", got)
	}
	if !strings.HasSuffix(filepath.ToSlash(got.File), "workflows/stack/assets/compose/docker-compose.yml") {
		t.Fatalf("unexpected compose file: %q", got.File)
	}
	if joined := strings.Join(got.Env, "\n"); !strings.Contains(joined, "MCP_HTTP_URL=http://127.0.0.1:8766") || !strings.Contains(joined, "DATABASE_URL=postgres://local") {
		t.Fatalf("unexpected compose env: %v", got.Env)
	}
	wantProjectDir := filepath.Clean(filepath.Join(wfRoot, "../../.."))
	if filepath.Clean(got.ProjectDirectory) != wantProjectDir {
		t.Fatalf("unexpected compose project directory: %q want %q", got.ProjectDirectory, wantProjectDir)
	}
	if len(got.Services) != 1 || got.Services[0] != "proxy" {
		t.Fatalf("unexpected compose services: %+v", got.Services)
	}
	if o.envMap["OLLAMA_HOST"] != "http://host.docker.internal:11434" {
		t.Fatalf("expected compose export in envMap, got %+v", o.envMap)
	}
}

func TestRunBlockingStepComposeDownSkipsWhenAutodownDisabled(t *testing.T) {
	withRunStepsSeams(t)
	o := baseRunStepsOpts()
	o.wf.Compose = domain.WorkflowComposeConfig{
		File:        "assets/compose/docker-compose.yml",
		AutodownEnv: "DORKPIPE_DEV_STACK_AUTODOWN",
	}
	o.envMap["DORKPIPE_DEV_STACK_AUTODOWN"] = "0"
	o.wf.Steps = []domain.Step{{Kind: "host", HostBuiltin: "compose_down"}}
	composeLifecycleFn = func(opts infrastructure.ComposeLifecycleOpts) error {
		t.Fatalf("compose lifecycle should be skipped when autodown is disabled, got %+v", opts)
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
}
