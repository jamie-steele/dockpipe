package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

// TestBuildWorkflowEnvIntoPrecedenceAndOverrides merges wf .env, repo .env, --env-file, DOCKPIPE_ENV_FILE, vars, --var.
func TestBuildWorkflowEnvIntoPrecedenceAndOverrides(t *testing.T) {
	tmp := t.TempDir()
	wfRoot := filepath.Join(tmp, "wf")
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(wfRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(wfRoot, ".env"), "WF=wf\nSHARED=wf\n")
	write(filepath.Join(repoRoot, ".env"), "REPO=repo\nSHARED=repo\n")
	custom := filepath.Join(tmp, "custom.env")
	write(custom, "CUSTOM=1\nSHARED=custom\n")
	envFile := filepath.Join(tmp, "env-file.env")
	write(envFile, "ENVFILE=1\nSHARED=envfile\n")

	t.Setenv("DOCKPIPE_ENV_FILE", envFile)
	env := map[string]string{"SHARED": "existing"}
	wf := &domain.Workflow{Vars: map[string]string{"WFVAR": "x", "SHARED": "fromwfvar"}}
	opts := &CliOpts{
		Workdir:      tmp,
		EnvFiles:     []string{custom},
		VarOverrides: []string{"SHARED=override", "CLI=ok"},
	}
	if err := buildWorkflowEnvInto(env, wf, filepath.Join(wfRoot, "config.yml"), wfRoot, repoRoot, opts); err != nil {
		t.Fatal(err)
	}

	if env["WF"] != "wf" || env["REPO"] != "repo" || env["CUSTOM"] != "1" || env["ENVFILE"] != "1" || env["WFVAR"] != "x" {
		t.Fatalf("expected merged env sources, got %#v", env)
	}
	if env["SHARED"] != "override" {
		t.Fatalf("cli override should win, got %q", env["SHARED"])
	}
	if env["CLI"] != "ok" {
		t.Fatalf("missing CLI var override: %#v", env)
	}
	if env["DOCKPIPE_WORKFLOW_CONFIG"] != filepath.Join(wfRoot, "config.yml") {
		t.Fatalf("missing workflow config injection: %#v", env)
	}
	if env["DOCKPIPE_WORKFLOW_DIR"] != wfRoot {
		t.Fatalf("missing workflow dir injection: %#v", env)
	}
}

func TestBuildWorkflowEnvIntoResolvesTypedInputsFromWorkflowEnv(t *testing.T) {
	tmp := t.TempDir()
	wfRoot := filepath.Join(tmp, "wf")
	repoRoot := filepath.Join(tmp, "repo")
	modelsDir := filepath.Join(wfRoot, "models")
	for _, dir := range []string{wfRoot, repoRoot, modelsDir} {
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
	write(filepath.Join(wfRoot, ".env"), "UH_VM_KEEPALIVE=true\n")
	write(filepath.Join(modelsDir, "QemuVmResolverConfig.pipe"), `public Class QemuVmResolverConfig
{
    public WindowsVmGeneral General;
    public WindowsVmAdvanced Advanced;
}
`)
	write(filepath.Join(modelsDir, "WindowsVmGeneral.pipe"), `public Class WindowsVmGeneral
{
    [EnvName = "DOCKPIPE_VM_EXEC_MODE"]
    public string ExecMode = "powershell";
}
`)
	write(filepath.Join(modelsDir, "WindowsVmAdvanced.pipe"), `public Class WindowsVmAdvanced
{
    [EnvName = "DOCKPIPE_VM_KEEPALIVE"]
    public string KeepAlive = "";
}
`)
	write(filepath.Join(wfRoot, "config.yml"), "name: demo\n")

	env := map[string]string{}
	wf := &domain.Workflow{
		Types: []string{"models/QemuVmResolverConfig.pipe"},
		Inputs: map[string]domain.InputBinding{
			"General.ExecMode":   {Value: "powershell"},
			"Advanced.KeepAlive": {From: "UH_VM_KEEPALIVE", Value: "false"},
		},
	}
	if err := buildWorkflowEnvInto(env, wf, filepath.Join(wfRoot, "config.yml"), wfRoot, repoRoot, &CliOpts{Workdir: repoRoot}); err != nil {
		t.Fatal(err)
	}
	if env["DOCKPIPE_VM_EXEC_MODE"] != "powershell" {
		t.Fatalf("DOCKPIPE_VM_EXEC_MODE=%q", env["DOCKPIPE_VM_EXEC_MODE"])
	}
	if env["DOCKPIPE_VM_KEEPALIVE"] != "true" {
		t.Fatalf("DOCKPIPE_VM_KEEPALIVE=%q", env["DOCKPIPE_VM_KEEPALIVE"])
	}
}

func TestBuildWorkflowEnvIntoInfersTypedInputsFromResolverConfig(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	wfRoot := filepath.Join(repoRoot, "workflows", "demo")
	resolverRoot := filepath.Join(repoRoot, "packages", "vm", "resolvers", "qemu")
	modelsDir := filepath.Join(resolverRoot, "models")
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
	write(filepath.Join(repoRoot, "dockpipe.config.json"), `{
  "compile": {
    "workflows": ["workflows", "packages"]
  }
}`)
	write(filepath.Join(resolverRoot, "profile"), "DOCKPIPE_RESOLVER_WORKFLOW=scripts/qemu.sh\n")
	write(filepath.Join(resolverRoot, "types.yml"), "types:\n  - models/QemuVmResolverConfig\n")
	write(filepath.Join(modelsDir, "QemuVmResolverConfig.pipe"), `public Class QemuVmResolverConfig
{
    public WindowsVmGeneral General;
}
`)
	write(filepath.Join(modelsDir, "WindowsVmGeneral.pipe"), `public Class WindowsVmGeneral
{
    [EnvName = "DOCKPIPE_VM_EXEC_MODE"]
    public string ExecMode = "powershell";
}
`)
	write(filepath.Join(wfRoot, "config.yml"), "name: demo\nresolver: qemu\n")

	env := map[string]string{}
	wf := &domain.Workflow{
		Resolver: "qemu",
		Inputs: map[string]domain.InputBinding{
			"General.ExecMode": {Value: "powershell"},
		},
	}
	if err := buildWorkflowEnvInto(env, wf, filepath.Join(wfRoot, "config.yml"), wfRoot, repoRoot, &CliOpts{Workdir: repoRoot}); err != nil {
		t.Fatal(err)
	}
	if env["DOCKPIPE_VM_EXEC_MODE"] != "powershell" {
		t.Fatalf("DOCKPIPE_VM_EXEC_MODE=%q", env["DOCKPIPE_VM_EXEC_MODE"])
	}
}

func TestBuildWorkflowEnvIntoResolvesTypedInputsFromTarWorkflowConfig(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	storeRoot := filepath.Join(projectRoot, "bin", ".dockpipe", "internal", "packages", "workflows")
	stageRoot := filepath.Join(tmp, "stage", "demo")
	modelsDir := filepath.Join(stageRoot, "models")
	for _, dir := range []string{projectRoot, storeRoot, modelsDir} {
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
	write(filepath.Join(projectRoot, "dockpipe.config.json"), "{}")
	write(filepath.Join(stageRoot, "config.yml"), "name: demo\n")
	write(filepath.Join(modelsDir, "QemuVmResolverConfig.pipe"), `public Class QemuVmResolverConfig
{
    public WindowsVmGeneral General;
}
`)
	write(filepath.Join(modelsDir, "WindowsVmGeneral.pipe"), `public Class WindowsVmGeneral
{
    [EnvName = "DOCKPIPE_VM_EXEC_MODE"]
    public string ExecMode = "powershell";
}
`)
	tgz := filepath.Join(storeRoot, "dockpipe-workflow-demo-0.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(stageRoot, tgz, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	wf := &domain.Workflow{
		Types: []string{"models/QemuVmResolverConfig.pipe"},
		Inputs: map[string]domain.InputBinding{
			"General.ExecMode": {Value: "powershell"},
		},
	}
	wfConfig, err := infrastructure.ResolveWorkflowConfigPathWithWorkdir(projectRoot, projectRoot, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := buildWorkflowEnvInto(env, wf, wfConfig, stageRoot, projectRoot, &CliOpts{Workdir: projectRoot}); err != nil {
		t.Fatal(err)
	}
	onDiskConfig := filepath.Join(projectRoot, "bin", ".dockpipe", "internal", "cache", "tarball")
	if got := env["DOCKPIPE_WORKFLOW_CONFIG"]; !strings.HasSuffix(filepath.ToSlash(got), "/workflows/demo/config.yml") || !strings.Contains(filepath.ToSlash(got), filepath.ToSlash(onDiskConfig)) {
		t.Fatalf("expected extracted workflow config path, got %q", got)
	}
	if env["DOCKPIPE_WORKFLOW_CONFIG_URI"] != wfConfig {
		t.Fatalf("expected original tar uri in DOCKPIPE_WORKFLOW_CONFIG_URI, got %q", env["DOCKPIPE_WORKFLOW_CONFIG_URI"])
	}
	if env["DOCKPIPE_VM_EXEC_MODE"] != "powershell" {
		t.Fatalf("DOCKPIPE_VM_EXEC_MODE=%q", env["DOCKPIPE_VM_EXEC_MODE"])
	}
}

func TestBuildWorkflowEnvIntoInfersTypedInputsFromResolverConfigForTarWorkflow(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	storeRoot := filepath.Join(projectRoot, "bin", ".dockpipe", "internal", "packages", "workflows")
	stageRoot := filepath.Join(tmp, "stage", "demo")
	resolverRoot := filepath.Join(projectRoot, "packages", "vm", "resolvers", "qemu")
	modelsDir := filepath.Join(resolverRoot, "models")
	for _, dir := range []string{projectRoot, storeRoot, stageRoot, modelsDir} {
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
	write(filepath.Join(projectRoot, "dockpipe.config.json"), `{
  "compile": {
    "workflows": ["workflows", "packages/vm"]
  }
}`)
	write(filepath.Join(stageRoot, "config.yml"), "name: demo\nresolver: qemu\nruntime: vm\n")
	write(filepath.Join(resolverRoot, "profile"), "DOCKPIPE_RESOLVER_WORKFLOW=scripts/qemu.sh\n")
	write(filepath.Join(resolverRoot, "types.yml"), "types:\n  - models/QemuVmResolverConfig\n")
	write(filepath.Join(modelsDir, "QemuVmResolverConfig.pipe"), `public Class QemuVmResolverConfig
{
    public WindowsVmGeneral General;
}
`)
	write(filepath.Join(modelsDir, "WindowsVmGeneral.pipe"), `public Class WindowsVmGeneral
{
    [EnvName = "DOCKPIPE_VM_EXEC_MODE"]
    public string ExecMode = "powershell";
}
`)
	tgz := filepath.Join(storeRoot, "dockpipe-workflow-demo-0.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(stageRoot, tgz, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	wf := &domain.Workflow{
		Resolver: "qemu",
		Runtime:  "vm",
		Inputs: map[string]domain.InputBinding{
			"General.ExecMode": {Value: "powershell"},
		},
	}
	wfConfig, err := infrastructure.ResolveWorkflowConfigPathWithWorkdir(projectRoot, projectRoot, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := buildWorkflowEnvInto(env, wf, wfConfig, stageRoot, projectRoot, &CliOpts{Workdir: projectRoot}); err != nil {
		t.Fatal(err)
	}
	if env["DOCKPIPE_VM_EXEC_MODE"] != "powershell" {
		t.Fatalf("DOCKPIPE_VM_EXEC_MODE=%q", env["DOCKPIPE_VM_EXEC_MODE"])
	}
}

// TestMergeExtraEnvCLIIntoSteps applies --env for multi-step workflows; --var keys are not overwritten.
func TestMergeExtraEnvCLIIntoSteps(t *testing.T) {
	// Simulates env after buildWorkflowEnvInto: --var already set SHARED.
	env := map[string]string{"FROM": "base", "SHARED": "from-var"}
	MergeExtraEnvCLIIntoSteps(env, []string{" R2_BUCKET=b ", "FROM=from-env", "SHARED=from-env"}, []string{"SHARED=from-var"})
	if env["R2_BUCKET"] != "b" {
		t.Fatalf("R2_BUCKET: got %q", env["R2_BUCKET"])
	}
	if env["FROM"] != "from-env" {
		t.Fatalf("FROM: got %q", env["FROM"])
	}
	if env["SHARED"] != "from-var" {
		t.Fatalf("--var should win over --env for SHARED: got %q", env["SHARED"])
	}
}

func TestPrependPATHDirPreservesColonDelimitedPath(t *testing.T) {
	current := "/mingw64/bin:/usr/local/bin:/c/Program Files/Docker/Docker/resources/bin"
	got := prependPATHDir(current, `C:\Source\dockpipe\src\bin`)
	want := `C:\Source\dockpipe\src\bin:/mingw64/bin:/usr/local/bin:/c/Program Files/Docker/Docker/resources/bin`
	if got != want {
		t.Fatalf("prependPATHDir() = %q want %q", got, want)
	}
}

func TestPrependPATHDirAvoidsDuplicateEntry(t *testing.T) {
	current := `C:\Source\dockpipe\src\bin;/usr/bin`
	got := prependPATHDir(current, `C:\Source\dockpipe\src\bin`)
	if got != current {
		t.Fatalf("prependPATHDir() = %q want %q", got, current)
	}
}

// TestLockedKeysAndApplyOutputsFile merges step outputs into env except CLI-locked keys and deletes the file after read.
func TestLockedKeysAndApplyOutputsFile(t *testing.T) {
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "outputs.env")
	if err := os.WriteFile(outFile, []byte("A=2\nB=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envMap := map[string]string{"A": "1"}
	dockerEnv := map[string]string{"A": "1"}
	locked := lockedKeys([]string{"A=lock", "X=1"})
	applyOutputsFile(outFile, envMap, dockerEnv, locked, nil, "")

	if envMap["A"] != "1" || dockerEnv["A"] != "1" {
		t.Fatalf("locked key should not change: env=%#v docker=%#v", envMap, dockerEnv)
	}
	if envMap["B"] != "2" || dockerEnv["B"] != "2" {
		t.Fatalf("unlocked output key should merge: env=%#v docker=%#v", envMap, dockerEnv)
	}
	if _, err := os.Stat(outFile); !os.IsNotExist(err) {
		t.Fatalf("outputs file should be removed, err=%v", err)
	}
}

// TestApplyOutputsFileDoesNotWipeSecretAPIKeys prevents step outputs from clearing OPENAI_API_KEY etc. with empty values.
func TestApplyOutputsFileDoesNotWipeSecretAPIKeys(t *testing.T) {
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "outputs.env")
	if err := os.WriteFile(outFile, []byte("OPENAI_API_KEY=\nOTHER=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envMap := map[string]string{"OPENAI_API_KEY": "sk-from-host", "OTHER": "1"}
	dockerEnv := map[string]string{"OPENAI_API_KEY": "sk-from-host", "OTHER": "1"}
	applyOutputsFile(outFile, envMap, dockerEnv, nil, nil, "")

	if envMap["OPENAI_API_KEY"] != "sk-from-host" || dockerEnv["OPENAI_API_KEY"] != "sk-from-host" {
		t.Fatalf("secret should not be wiped: env=%#v docker=%#v", envMap, dockerEnv)
	}
	if envMap["OTHER"] != "2" || dockerEnv["OTHER"] != "2" {
		t.Fatalf("non-secret should merge: env=%#v docker=%#v", envMap, dockerEnv)
	}
}
