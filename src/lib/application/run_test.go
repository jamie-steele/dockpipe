package application

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// writeTestCoreResolver seeds templates/core/resolvers/<name> for tests that use a temp repoRoot.
func writeTestCoreResolver(t *testing.T, repoRoot, name, body string) {
	t.Helper()
	p := filepath.Join(repoRoot, "templates", "core", "resolvers", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withRunSeams(t *testing.T) {
	t.Helper()
	t.Setenv("DOCKPIPE_SKIP_DOCKER_PREFLIGHT", "1")
	oldRepoRoot := repoRootAppFn
	oldLoadWf := loadWorkflowAppFn
	oldLoadResolver := loadResolverFileAppFn
	oldTemplateBuild := templateBuildAppFn
	oldMaybeTag := maybeVersionTagAppFn
	oldResolveAction := resolveActionPathFn
	oldSourceHost := sourceHostScriptAppFn
	oldDockerBuild := dockerBuildAppFn
	oldDockerImageExists := dockerImageExistsAppFn
	oldRunContainer := runContainerAppFn
	oldResolvePre := resolvePreScriptAppFn
	oldResolveWfScript := resolveWorkflowAppFn
	oldBundledCommit := isBundledCommitAppFn
	oldRunSteps := runStepsAppFn
	oldRunHost := runHostScriptAppFn
	oldExit := osExitAppFn
	t.Cleanup(func() {
		repoRootAppFn = oldRepoRoot
		loadWorkflowAppFn = oldLoadWf
		loadResolverFileAppFn = oldLoadResolver
		templateBuildAppFn = oldTemplateBuild
		maybeVersionTagAppFn = oldMaybeTag
		resolveActionPathFn = oldResolveAction
		sourceHostScriptAppFn = oldSourceHost
		dockerBuildAppFn = oldDockerBuild
		dockerImageExistsAppFn = oldDockerImageExists
		runContainerAppFn = oldRunContainer
		resolvePreScriptAppFn = oldResolvePre
		resolveWorkflowAppFn = oldResolveWfScript
		isBundledCommitAppFn = oldBundledCommit
		runStepsAppFn = oldRunSteps
		runHostScriptAppFn = oldRunHost
		osExitAppFn = oldExit
	})
	runHostScriptAppFn = func(scriptAbs string, env []string) error {
		t.Fatalf("unexpected RunHostScript call: %s", scriptAbs)
		return nil
	}
}

func captureRunTestStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = old
	})
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return string(b)
}

// TestRunNonStepsHappyPath runs resolver-driven single-command mode with mocked docker build/run.
func TestRunNonStepsHappyPath(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		if name == "codex" {
			return "dockpipe-codex", "/build/codex", true
		}
		return "", "", false
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuilt := false
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error {
		dockerBuilt = true
		if image != "dockpipe-codex" || dockerfileDir != "/build/codex" || contextDir != repoRoot {
			t.Fatalf("unexpected docker build args: %q %q %q", image, dockerfileDir, contextDir)
		}
		return nil
	}
	containerRan := false
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		containerRan = true
		if o.Image != "dockpipe-codex" {
			t.Fatalf("unexpected image: %q", o.Image)
		}
		if len(argv) != 2 || argv[0] != "echo" || argv[1] != "hi" {
			t.Fatalf("unexpected argv: %#v", argv)
		}
		return 0, nil
	}

	err := Run([]string{"--resolver", "codex", "--", "echo", "hi"}, []string{"BASE=1"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !dockerBuilt || !containerRan {
		t.Fatalf("expected docker build and run, built=%v ran=%v", dockerBuilt, containerRan)
	}
}

func TestRunNonStepsForwardsWorkflowPolicyProxyEnv(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	wfDir := filepath.Join(repoRoot, "workflows", "proxydemo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: proxydemo
isolate: codex
vars:
  DOCKPIPE_POLICY_PROXY_URL: http://proxy-sidecar:8080
  DOCKPIPE_POLICY_PROXY_NO_PROXY: metadata.local
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		if name == "codex" {
			return "dockpipe-codex", "/build/codex", true
		}
		return "", "", false
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }
	var gotRunOpts infrastructure.RunOpts
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		gotRunOpts = o
		return 0, nil
	}

	err := Run([]string{"--workflow", "proxydemo", "--", "echo", "hi"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	em := domain.EnvSliceToMap(gotRunOpts.ExtraEnv)
	if em["DOCKPIPE_POLICY_PROXY_URL"] != "http://proxy-sidecar:8080" {
		t.Fatalf("expected workflow proxy URL forwarded, got %#v", em)
	}
	if em["DOCKPIPE_POLICY_PROXY_NO_PROXY"] != "metadata.local" {
		t.Fatalf("expected workflow proxy no_proxy forwarded, got %#v", em)
	}
}

func TestMaybeSkipDockerBuildForWorkflowTarballArtifact(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	workdir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "workflows", "mywf"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
isolate: codex
steps: []
`
	if err := os.WriteFile(filepath.Join(repoRoot, "workflows", "mywf", "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workdir, "templates", "core", "assets", "images", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "templates", "core", "assets", "images", "codex", "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "version"), []byte("0.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", workdir, "--from", filepath.Join(repoRoot, "workflows", "mywf")}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(workdir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.0.0.tar.gz")
	wfURI := "tar://" + tgz + "##workflows/mywf/config.yml"
	dockerImageExistsAppFn = func(image string) (bool, error) { return true, nil }
	buildDir := filepath.Join(workdir, "templates", "core", "assets", "images", "codex")
	skip, msg, err := maybeSkipDockerBuildForWorkflow(workdir, wfURI, filepath.Join(repoRoot, "workflows", "mywf"), "dockpipe-codex:0.0.0", buildDir, workdir)
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Fatal("expected docker build skip from compiled tarball artifact")
	}
	if !strings.Contains(msg, "using cached image artifact") {
		t.Fatalf("unexpected message: %q", msg)
	}
}

// TestRunHostIsolateHappyPath runs host isolate instead of docker when DOCKPIPE_RESOLVER_HOST_ISOLATE is set.
func TestRunHostIsolateHappyPath(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	isolatePath := filepath.Join(repoRoot, "scripts", "host-isolate-test.sh")
	if err := os.WriteFile(isolatePath, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestCoreResolver(t, repoRoot, "hostiso", "DOCKPIPE_RESOLVER_HOST_ISOLATE=scripts/host-isolate-test.sh\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{
			"DOCKPIPE_RESOLVER_HOST_ISOLATE": "scripts/host-isolate-test.sh",
		}, nil
	}
	dockerBuilt := false
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		dockerBuilt = true
		return "", "", false
	}
	containerRan := false
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		containerRan = true
		return 0, nil
	}
	var gotIsolate string
	runHostScriptAppFn = func(scriptAbs string, env []string) error {
		gotIsolate = scriptAbs
		return nil
	}

	err := Run([]string{"--resolver", "hostiso", "--workdir", repoRoot, "--"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if dockerBuilt || containerRan {
		t.Fatalf("expected no docker build/run, built=%v ran=%v", dockerBuilt, containerRan)
	}
	if filepath.Clean(gotIsolate) != filepath.Clean(isolatePath) {
		t.Fatalf("unexpected isolate script: got %q want %q", gotIsolate, isolatePath)
	}
}

// TestRunMissingWorkflowErrors when --workflow name has no matching config.yml.
func TestRunMissingWorkflowErrors(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }

	err := Run([]string{"--workflow", "missing", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), `workflow "missing" not found`) {
		t.Fatalf("expected workflow missing error, got %v", err)
	}
}

// TestRunPreScriptNotFoundErrors when resolved pre-script path does not exist on disk.
func TestRunPreScriptNotFoundErrors(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	resolvePreScriptAppFn = func(p, root, _ string) string { return filepath.Join(root, "does-not-exist.sh") }

	err := Run([]string{"--resolver", "codex", "--run", "scripts/pre.sh", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "pre-script not found") {
		t.Fatalf("expected pre-script not found error, got %v", err)
	}
}

// TestRunMissingDashErrors when user omits the standalone -- before the command.
func TestRunMissingDashErrors(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }

	err := Run([]string{"--resolver", "codex"}, nil)
	if err == nil || !strings.Contains(err.Error(), "expected -- before command") {
		t.Fatalf("expected missing -- error, got %v", err)
	}
}

// TestRunNonZeroContainerExitCallsExitFn propagates container exit code via os.Exit shim.
func TestRunNonZeroContainerExitCallsExitFn(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) { return 9, nil }
	exitCode := -1
	osExitAppFn = func(code int) { exitCode = code }

	err := Run([]string{"--resolver", "codex", "--", "echo", "hi"}, nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if exitCode != 9 {
		t.Fatalf("expected exit code 9, got %d", exitCode)
	}
}

// TestRunWorkflowStepsModeDelegatesToRunSteps when workflow has steps:, Run calls runStepsAppFn.
func TestRunWorkflowStepsModeDelegatesToRunSteps(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	workflowDir := filepath.Join(repoRoot, "templates", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "config.yml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadWorkflowAppFn = func(path string) (*domain.Workflow, error) {
		bFalse := false
		return &domain.Workflow{
			Resolver: "codex",
			Steps:    []domain.Step{{Cmd: "echo hi", Blocking: &bFalse}},
		}, nil
	}
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	called := false
	runStepsAppFn = func(o runStepsOpts) error {
		called = true
		if o.resolver != "codex" {
			t.Fatalf("expected resolver from workflow, got %q", o.resolver)
		}
		return nil
	}

	err := Run([]string{"--workflow", "demo", "--", "echo", "x"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !called {
		t.Fatal("expected runSteps delegation in steps mode")
	}
}

// TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap ensures --workdir is stored on envMap so
// envSlice rebuilds after strategy pre-scripts still pass DOCKPIPE_WORKDIR to host steps.
func TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	workflowDir := filepath.Join(repoRoot, "templates", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "config.yml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadWorkflowAppFn = func(path string) (*domain.Workflow, error) {
		bFalse := false
		return &domain.Workflow{
			Resolver: "codex",
			Steps:    []domain.Step{{Cmd: "echo hi", Blocking: &bFalse}},
		}, nil
	}
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	wantWd := "/path/to/your/project"
	runStepsAppFn = func(o runStepsOpts) error {
		if o.envMap["DOCKPIPE_WORKDIR"] != wantWd {
			t.Fatalf("envMap DOCKPIPE_WORKDIR=%q want %q", o.envMap["DOCKPIPE_WORKDIR"], wantWd)
		}
		return nil
	}

	base := []string{"DOCKPIPE_WORKDIR=/wrong/inherited"}
	err := Run([]string{"--workflow", "demo", "--workdir", wantWd, "--", "echo", "x"}, base)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestRunWorkflowAppliesCompiledRuntimePolicy(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	wfDir := filepath.Join(repoRoot, "templates", "secure")
	if err := os.MkdirAll(filepath.Join(wfDir, domain.RuntimeManifestDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	cfg := `name: secure
isolate: codex
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	rm := &domain.CompiledRuntimeManifest{
		Schema: 1,
		Kind:   domain.RuntimeManifestKind,
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "offline",
				Enforcement: "native",
			},
			FS: domain.CompiledFilesystemPolicy{
				Root:      "readonly",
				TempPaths: []string{"/tmp", "/work/cache"},
			},
			Process: domain.CompiledProcessPolicy{
				User:            "root",
				NoNewPrivileges: true,
				DropCaps:        []string{"ALL"},
				PIDLimit:        128,
				Resources: domain.CompiledResourceLimits{
					CPU:    "2",
					Memory: "1g",
				},
			},
		},
	}
	rm.EnforcementSummaries = []string{"filesystem and process defaults are emitted as the effective policy baseline"}
	rm.RuleIDs = []string{"security.preset.secure-default", "network.mode.offline"}
	rm.PolicyFingerprint, _ = domain.FingerprintJSON(rm.Security)
	rmb, err := marshalArtifactJSON(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, domain.RuntimeManifestDirName, domain.RuntimeManifestFileName), rmb, 0o644); err != nil {
		t.Fatal(err)
	}

	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }
	var got infrastructure.RunOpts
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		got = o
		return 0, nil
	}

	stderr := captureRunTestStderr(t, func() {
		err = Run([]string{"--workflow", "secure", "--workdir", repoRoot, "--", "echo", "hi"}, nil)
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got.NetworkMode != "none" {
		t.Fatalf("expected offline policy to set network=none, got %q", got.NetworkMode)
	}
	if !got.ReadOnlyRootFS {
		t.Fatal("expected readonly root filesystem")
	}
	if got.ContainerUser != "0:0" {
		t.Fatalf("expected root container user override, got %q", got.ContainerUser)
	}
	if got.PIDLimit != 128 || got.CPULimit != "2" || got.MemoryLimit != "1g" {
		t.Fatalf("unexpected resource limits: %+v", got)
	}
	if !strings.Contains(strings.Join(got.SecurityOpt, ","), "no-new-privileges") {
		t.Fatalf("expected no-new-privileges, got %#v", got.SecurityOpt)
	}
	if !strings.Contains(strings.Join(got.CapDrop, ","), "ALL") {
		t.Fatalf("expected cap drop, got %#v", got.CapDrop)
	}
	if !strings.Contains(strings.Join(got.TmpfsPaths, ","), "/tmp") {
		t.Fatalf("expected /tmp tmpfs, got %#v", got.TmpfsPaths)
	}
	for _, p := range got.TmpfsPaths {
		if strings.HasPrefix(p, "/work/") {
			t.Fatalf("workspace path should not be converted to tmpfs: %#v", got.TmpfsPaths)
		}
	}
	for _, want := range []string{
		"[dockpipe] runtime policy: network=offline, root=readonly, tmpfs=/tmp,/work/cache, no-new-privileges, cap-drop=ALL, pids=128, cpu=2, memory=1g",
		"[dockpipe] policy enforcement: network offline is enforced natively by the Docker runtime",
		"[dockpipe] policy note: filesystem and process defaults are emitted as the effective policy baseline",
		"[dockpipe] policy rules: security.preset.secure-default, network.mode.offline",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
	ents, err := os.ReadDir(infrastructure.RunPolicyRecordsDir(repoRoot))
	if err != nil {
		t.Fatalf("expected run policy record: %v", err)
	}
	if len(ents) == 0 {
		t.Fatal("expected at least one run policy record")
	}
	b, err := os.ReadFile(filepath.Join(infrastructure.RunPolicyRecordsDir(repoRoot), ents[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var rec infrastructure.RunPolicyRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.WorkflowName != "secure" || rec.NetworkEnforcement != "native" || rec.ImageRef != "dockpipe-codex" {
		t.Fatalf("unexpected run policy record: %+v", rec)
	}
	if rec.ImageArtifactDecision != "built image artifact for current run" {
		t.Fatalf("unexpected image decision: %+v", rec)
	}
	if !strings.Contains(strings.Join(rec.EnforcementNotes, "\n"), "enforced natively") {
		t.Fatalf("expected enforcement note in record, got %+v", rec)
	}
}

func TestRunWorkflowStepsModeLogsCompiledRuntimePolicy(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	workflowDir := filepath.Join(repoRoot, "templates", "demo")
	if err := os.MkdirAll(filepath.Join(workflowDir, domain.RuntimeManifestDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	if err := os.WriteFile(filepath.Join(workflowDir, "config.yml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadWorkflowAppFn = func(path string) (*domain.Workflow, error) {
		bFalse := false
		return &domain.Workflow{
			Resolver: "codex",
			Steps:    []domain.Step{{Cmd: "echo hi", Blocking: &bFalse}},
		}, nil
	}
	rm := &domain.CompiledRuntimeManifest{
		Schema: 1,
		Kind:   domain.RuntimeManifestKind,
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{Mode: "restricted", Enforcement: "advisory"},
			FS:      domain.CompiledFilesystemPolicy{Root: "readonly", TempPaths: []string{"/tmp"}},
			Process: domain.CompiledProcessPolicy{NoNewPrivileges: true, DropCaps: []string{"ALL"}, PIDLimit: 256},
		},
		EnforcementSummaries: []string{"filesystem and process defaults are emitted as the effective policy baseline"},
		RuleIDs:              []string{"filesystem.root.readonly", "process.no-new-privileges"},
	}
	rm.PolicyFingerprint, _ = domain.FingerprintJSON(rm.Security)
	rmb, err := marshalArtifactJSON(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, domain.RuntimeManifestDirName, domain.RuntimeManifestFileName), rmb, 0o644); err != nil {
		t.Fatal(err)
	}
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	called := false
	runStepsAppFn = func(o runStepsOpts) error {
		called = true
		return nil
	}

	var runErr error
	stderr := captureRunTestStderr(t, func() {
		runErr = Run([]string{"--workflow", "demo", "--", "echo", "x"}, nil)
	})
	if runErr != nil {
		t.Fatalf("Run failed: %v", runErr)
	}
	if !called {
		t.Fatal("expected runSteps delegation in steps mode")
	}
	for _, want := range []string{
		"[dockpipe] runtime policy: network=restricted, root=readonly, tmpfs=/tmp, no-new-privileges, cap-drop=ALL, pids=256",
		"[dockpipe] policy enforcement: network restricted is advisory in this build; full egress filtering is not active yet",
		"[dockpipe] policy coverage: domain allow/block rules are compiled for inspection but are not enforced natively by Docker",
		"[dockpipe] policy note: filesystem and process defaults are emitted as the effective policy baseline",
		"[dockpipe] policy rules: filesystem.root.readonly, process.no-new-privileges",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCompiledRuntimePolicyLogLinesExplainAdvisoryAllowlist(t *testing.T) {
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "allowlist",
				Enforcement: "advisory",
				Allow:       []string{"api.openai.com", "*.anthropic.com", "api.github.com"},
			},
		},
	}
	lines := strings.Join(compiledRuntimePolicyLogLines(rm), "\n")
	for _, want := range []string{
		"runtime policy: network=allowlist, allow=api.openai.com,*.anthropic.com,+1",
		"policy enforcement: network allowlist is advisory in this build; full egress filtering is not active yet",
		"policy coverage: domain allow/block rules are compiled for inspection but are not enforced natively by Docker",
	} {
		if !strings.Contains(lines, want) {
			t.Fatalf("expected log lines to contain %q, got:\n%s", want, lines)
		}
	}
}

func TestApplyCompiledRuntimePolicyInjectsProxyEnv(t *testing.T) {
	t.Setenv("DOCKPIPE_POLICY_PROXY_URL", "http://policy-proxy:8080")
	t.Setenv("DOCKPIPE_POLICY_PROXY_NO_PROXY", "metadata.local")
	runOpts := &infrastructure.RunOpts{
		ExtraEnv: []string{"BASE=1"},
	}
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "allowlist",
				Enforcement: "proxy",
				Allow:       []string{"api.openai.com", "*.anthropic.com"},
				Block:       []string{"*.facebook.com"},
			},
		},
	}
	if err := applyCompiledRuntimeManifest(runOpts, rm); err != nil {
		t.Fatalf("applyCompiledRuntimeManifest failed: %v", err)
	}
	em := domain.EnvSliceToMap(runOpts.ExtraEnv)
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		raw := em[key]
		if raw == "" {
			t.Fatalf("expected %s in proxy env, got %#v", key, em)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", key, err)
		}
		if u.Host != "policy-proxy:8080" || u.User == nil || u.User.Username() == "" {
			t.Fatalf("expected tokenized proxy URL for %s, got %q", key, raw)
		}
	}
	for key, want := range map[string]string{
		"DOCKPIPE_POLICY_PROXY_BASE_URL":      "http://policy-proxy:8080",
		"NO_PROXY":                            "metadata.local,localhost,127.0.0.1,::1",
		"no_proxy":                            "metadata.local,localhost,127.0.0.1,::1",
		"DOCKPIPE_POLICY_NETWORK_MODE":        "allowlist",
		"DOCKPIPE_POLICY_NETWORK_ENFORCEMENT": "proxy",
		"DOCKPIPE_POLICY_NETWORK_ALLOW":       "api.openai.com,*.anthropic.com",
		"DOCKPIPE_POLICY_NETWORK_BLOCK":       "*.facebook.com",
	} {
		if em[key] != want {
			t.Fatalf("expected %s=%q, got %#v", key, want, em)
		}
	}
}

func TestApplyCompiledRuntimePolicyProxyRequiresProxyURL(t *testing.T) {
	runOpts := &infrastructure.RunOpts{}
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "proxy",
			},
		},
	}
	if err := applyCompiledRuntimeManifest(runOpts, rm); err == nil || !strings.Contains(err.Error(), "DOCKPIPE_POLICY_PROXY_URL") {
		t.Fatalf("expected missing proxy URL error, got %v", err)
	}
}

func TestApplyCompiledRuntimePolicyUsesRunEnvProxyExport(t *testing.T) {
	runOpts := &infrastructure.RunOpts{
		ExtraEnv: []string{
			"DOCKPIPE_POLICY_PROXY_URL=http://proxy-sidecar:8080",
			"DOCKPIPE_POLICY_PROXY_NO_PROXY=metadata.local",
		},
	}
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "proxy",
			},
		},
	}
	if err := applyCompiledRuntimeManifest(runOpts, rm); err != nil {
		t.Fatalf("applyCompiledRuntimeManifest failed: %v", err)
	}
	em := domain.EnvSliceToMap(runOpts.ExtraEnv)
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY"} {
		raw := em[key]
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", key, err)
		}
		if u.Host != "proxy-sidecar:8080" || u.User == nil || u.User.Username() == "" {
			t.Fatalf("expected tokenized proxy URL for %s, got %q", key, raw)
		}
	}
	for key, want := range map[string]string{
		"DOCKPIPE_POLICY_PROXY_BASE_URL":      "http://proxy-sidecar:8080",
		"NO_PROXY":                            "metadata.local,localhost,127.0.0.1,::1",
		"DOCKPIPE_POLICY_NETWORK_ENFORCEMENT": "proxy",
	} {
		if em[key] != want {
			t.Fatalf("expected %s=%q, got %#v", key, want, em)
		}
	}
}

func TestPolicyProxyURLWithTokenEncodesCompiledPolicy(t *testing.T) {
	raw := policyProxyURLWithToken("http://policy-proxy:8080", domain.CompiledNetworkPolicy{
		Mode:  "allowlist",
		Allow: []string{"api.openai.com", "*.anthropic.com"},
		Block: []string{"*.facebook.com"},
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse tokenized proxy url: %v", err)
	}
	if u.Host != "policy-proxy:8080" || u.User == nil {
		t.Fatalf("unexpected tokenized proxy url: %q", raw)
	}
	username := u.User.Username()
	decoded, err := base64.RawURLEncoding.DecodeString(username)
	if err != nil {
		t.Fatalf("decode token username: %v", err)
	}
	var payload struct {
		Version string   `json:"version"`
		Mode    string   `json:"mode"`
		Allow   []string `json:"allow"`
		Block   []string `json:"block"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("unmarshal token payload: %v", err)
	}
	if payload.Version != "dockpipe-proxy-v1" || payload.Mode != "allowlist" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if !strings.Contains(strings.Join(payload.Allow, ","), "api.openai.com") {
		t.Fatalf("unexpected allow payload: %+v", payload.Allow)
	}
	if !strings.Contains(strings.Join(payload.Block, ","), "*.facebook.com") {
		t.Fatalf("unexpected block payload: %+v", payload.Block)
	}
}

// TestRunAutoBranchForRepoWithoutBranch sets work branch when --repo is set without --branch (clone-worktree flow).
func TestRunAutoBranchForRepoWithoutBranch(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "scripts", "clone-worktree.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }
	var gotRunOpts infrastructure.RunOpts
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		gotRunOpts = o
		return 0, nil
	}

	err := Run([]string{"--resolver", "codex", "--repo", "https://example.com/r.git", "--workdir", repoRoot, "--", "echo", "x"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if gotRunOpts.Image == "" {
		t.Fatalf("expected run container options to be populated")
	}
}

// TestRunInfersOriginWhenWorkflowUsesCloneWorktree fills --repo from git remote when workflow uses clone-worktree pre-script.
func TestRunInfersOriginWhenWorkflowUsesCloneWorktree(t *testing.T) {
	withRunSeams(t)
	// Single tree: git remote, workflow YAML, and scripts/ must share the same project root as cwd
	// so origin inference and scripts/… resolution agree (see projectRoot in paths.go).
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	wantURL := "https://example.test/unite.git"
	if out, err := exec.Command("git", "-C", root, "remote", "add", "origin", wantURL).CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}

	wfDir := filepath.Join(root, "templates", "wfinfer")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestCoreResolver(t, root, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	cfg := `run: scripts/clone-worktree.sh
act: scripts/noop.sh
isolate: codex
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"clone-worktree.sh", "noop.sh"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", name), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	repoRootAppFn = func() (string, error) { return root, nil }
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }

	var capturedPreEnv []string
	sourceHostScriptAppFn = func(scriptPath string, env []string) (map[string]string, error) {
		capturedPreEnv = append([]string(nil), env...)
		return map[string]string{"DOCKPIPE_WORKDIR": filepath.Join(root, "fake-worktree")}, nil
	}

	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		return 0, nil
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	err := Run([]string{"--workflow", "wfinfer", "--", "echo", "hi"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	want := "DOCKPIPE_REPO_URL=" + wantURL
	found := false
	for _, e := range capturedPreEnv {
		if e == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in pre-script env, got:\n%s", want, strings.Join(capturedPreEnv, "\n"))
	}
}

// TestStrategyHookOrder asserts strategy before → container → strategy after for single-command mode.
func TestStrategyHookOrder(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	stratDir := filepath.Join(repoRoot, "templates", "core", "strategies")
	if err := os.MkdirAll(stratDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stratDir, "hook-order"), []byte("DOCKPIPE_STRATEGY_BEFORE=scripts/strat-before.sh\nDOCKPIPE_STRATEGY_AFTER=scripts/strat-after.sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptDir := filepath.Join(repoRoot, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"strat-before.sh", "strat-after.sh"} {
		if err := os.WriteFile(filepath.Join(scriptDir, n), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	wfDir := t.TempDir()
	writeTestCoreResolver(t, repoRoot, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	cfg := `name: hook-order
strategy: hook-order
isolate: codex
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		if name == "codex" {
			return "dockpipe-codex", "/build/codex", true
		}
		return "", "", false
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }

	var order []string
	sourceHostScriptAppFn = func(scriptPath string, env []string) (map[string]string, error) {
		order = append(order, "before:"+filepath.Base(scriptPath))
		return nil, nil
	}
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		order = append(order, "container")
		return 0, nil
	}
	runHostScriptAppFn = func(scriptAbs string, env []string) error {
		order = append(order, "after:"+filepath.Base(scriptAbs))
		return nil
	}

	err := Run([]string{"--workflow-file", filepath.Join(wfDir, "config.yml"), "--workdir", repoRoot, "--", "echo", "hi"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	want := []string{"before:strat-before.sh", "container", "after:strat-after.sh"}
	if len(order) != len(want) {
		t.Fatalf("hook order: got %#v want %#v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("hook order: got %#v want %#v", order, want)
		}
	}
}
