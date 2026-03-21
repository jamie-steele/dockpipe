package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

func withRunStepsSeams(t *testing.T) {
	t.Helper()
	oldBuild := dockerBuildFn
	oldRun := runContainerFn
	oldSource := sourceHostScriptFn
	oldRunHost := runHostScriptFn
	oldStat := osStatFn
	oldGetwd := getwdFn
	t.Cleanup(func() {
		dockerBuildFn = oldBuild
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
	out := filepath.Join(wd, ".dockpipe", "outputs.env")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := baseRunStepsOpts()
	o.opts.Workdir = wd
	o.wf.Steps = []domain.Step{{SkipContainer: true, Outputs: ".dockpipe/outputs.env"}}
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
	getwdFn = func() (string, error) { return t.TempDir(), nil }
	dockerEnv := map[string]string{}
	if err := runBlockingStep(&o, 0, 1, dockerEnv); err != nil {
		t.Fatalf("runBlockingStep error: %v", err)
	}
	if !built || !ran {
		t.Fatalf("expected build + run, built=%v ran=%v", built, ran)
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
