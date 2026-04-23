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
	oldStat := osStatFn
	oldGetwd := getwdFn
	t.Cleanup(func() {
		dockerBuildFn = oldBuild
		dockerImageExistsFn = oldImageExists
		composeLifecycleFn = oldCompose
		runContainerFn = oldRun
		sourceHostScriptFn = oldSource
		runHostScriptFn = oldRunHost
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

// TestRunBlockingStepSkipContainerMergesOutputs loads outputs.env into env for a blocking skip_container step.
func TestRunBlockingStepSkipContainerMergesOutputs(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	out := filepath.Join(wd, infrastructure.DockpipeDirRel, "outputs.env")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := baseRunStepsOpts()
	o.opts.Workdir = wd
	o.wf.Steps = []domain.Step{{SkipContainer: true, Outputs: domain.DefaultOutputsEnvRel}}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if o.envMap["A"] != "1" || dockerEnv["A"] != "1" {
		t.Fatalf("expected merged outputs, env=%#v docker=%#v", o.envMap, dockerEnv)
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

func TestRunBlockingStepSkipsBuildWhenCompiledImageArtifactExists(t *testing.T) {
	withRunStepsSeams(t)
	wd := t.TempDir()
	wfRoot := filepath.Join(wd, "wf")
	if err := os.MkdirAll(filepath.Join(wfRoot, domain.RuntimeManifestDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	o := baseRunStepsOpts()
	o.repoRoot = wd
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
	artifact, err := buildImageArtifactManifest(wd, "", "", "codex", "dockpipe-codex", filepath.Join(wd, "templates", "core", "assets", "images", "codex"), wd, policyFingerprint)
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
		Schema:            1,
		Kind:              domain.RuntimeManifestKind,
		PolicyFingerprint: policyFingerprint,
		Security: domain.CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "advisory",
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
	if !strings.Contains(rec.ImageArtifactDecision, "using cached image artifact") {
		t.Fatalf("expected cached image decision, got %+v", rec)
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
	artifact, err := buildImageArtifactManifest(wd, "", "", "codex", "dockpipe-codex", filepath.Join(wd, "templates", "core", "assets", "images", "codex"), wd, "sha256:oldpolicy")
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
		Schema: 1,
		Kind:   domain.RuntimeManifestKind,
		Security: domain.CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "advisory",
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
	skip, _, err := maybeSkipDockerBuildForStep(wd, wd, "", wfRoot, "dockpipe-codex", filepath.Join(wd, "templates", "core", "assets", "images", "codex"), wd)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("expected policy fingerprint mismatch to disable cache reuse")
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
	o.wf.Steps = []domain.Step{{SkipContainer: true, Blocking: &bFalse}}
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
		t.Fatal("expected host exec for skip_container parallel pre-script")
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
	o.wf.Steps = []domain.Step{{SkipContainer: true, HostBuiltin: "compose_up"}}
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
	o.wf.Steps = []domain.Step{{SkipContainer: true, HostBuiltin: "compose_down"}}
	composeLifecycleFn = func(opts infrastructure.ComposeLifecycleOpts) error {
		t.Fatalf("compose lifecycle should be skipped when autodown is disabled, got %+v", opts)
		return nil
	}
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
}
