package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"
)

func withRunStepSeams(t *testing.T, fn func()) {
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
	fn()
}

// TestRunSteps_ParallelBatchAggregatesOutputsInOrder merges async skip_container outputs in declaration order (last wins).
func TestRunSteps_ParallelBatchAggregatesOutputsInOrder(t *testing.T) {
	tmp := t.TempDir()
	aPath := filepath.Join(tmp, ".dockpipe", "a.env")
	bPath := filepath.Join(tmp, ".dockpipe", "b.env")
	if err := os.MkdirAll(filepath.Dir(aPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aPath, []byte("KEY=from_a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("KEY=from_b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	withRunStepSeams(t, func() {
		// No containers should run in this test.
		runContainerFn = func(infrastructure.RunOpts, []string) (int, error) {
			t.Fatalf("runContainerFn should not be called for skip_container steps")
			return 0, nil
		}

		nonBlocking := false
		wf := &domain.Workflow{
			Steps: []domain.Step{
				{ID: "a", SkipContainer: true, Outputs: ".dockpipe/a.env", Blocking: &nonBlocking},
				{ID: "b", SkipContainer: true, Outputs: ".dockpipe/b.env", Blocking: &nonBlocking},
				{ID: "join", SkipContainer: true}, // default blocking
			},
		}
		o := runStepsOpts{
			wf:       wf,
			envMap:   map[string]string{"BASE": "x"},
			envSlice: []string{"BASE=x"},
			locked:   map[string]bool{},
			opts:     &CliOpts{Workdir: tmp},
		}
		if err := runSteps(o); err != nil {
			t.Fatalf("runSteps error: %v", err)
		}
		if got := o.envMap["KEY"]; got != "from_b" {
			t.Fatalf("expected last-write-wins merge (from_b), got %q", got)
		}
	})
}

// TestRunStepPreScripts_UsesInjectedSourceFunction resolves workflow-relative pre-script paths and merges sourced env.
func TestRunStepPreScripts_UsesInjectedSourceFunction(t *testing.T) {
	withRunStepSeams(t, func() {
		osStatFn = func(string) (os.FileInfo, error) { return nil, nil }
		sourceHostScriptFn = func(scriptPath string, env []string) (map[string]string, error) {
			return map[string]string{"FROM_PRE": scriptPath, "SEEN": "1"}, nil
		}

		o := &runStepsOpts{
			wfRoot:   "/wf",
			repoRoot: "/repo",
			envMap:   map[string]string{},
			envSlice: []string{},
			opts:     &CliOpts{},
		}
		step := domain.Step{Run: []string{"local/pre.sh"}}
		if err := runStepPreScripts(o, 1, step); err != nil {
			t.Fatalf("runStepPreScripts err: %v", err)
		}
		if o.envMap["SEEN"] != "1" {
			t.Fatalf("expected env mutation from source script, got %#v", o.envMap)
		}
		// ResolveWorkflowScript returns ToSlash paths on every GOOS (see infrastructure/paths.go).
		wantPath := filepath.ToSlash(filepath.Join("/wf", "local/pre.sh"))
		if o.envMap["FROM_PRE"] != wantPath {
			t.Fatalf("expected resolved workflow path %q, got %q", wantPath, o.envMap["FROM_PRE"])
		}
	})
}

// TestRunStepPreScripts_SkipContainerUsesRunHostExec runs skip_container run: via RunHostScript (not sourced).
func TestRunStepPreScripts_SkipContainerUsesRunHostExec(t *testing.T) {
	withRunStepSeams(t, func() {
		tmp := t.TempDir()
		script := filepath.Join(tmp, "host.sh")
		if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\necho ok\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		osStatFn = func(name string) (os.FileInfo, error) { return os.Stat(name) }
		called := false
		runHostScriptFn = func(path string, env []string) error {
			called = true
			// ResolveWorkflowScript uses ToSlash; compare in slash form for Windows.
			if filepath.ToSlash(path) != filepath.ToSlash(script) {
				t.Fatalf("path %q want %q", path, script)
			}
			return nil
		}
		o := &runStepsOpts{
			wfRoot:   tmp,
			repoRoot: "/repo",
			envMap:   map[string]string{},
			envSlice: []string{},
			opts:     &CliOpts{},
		}
		step := domain.Step{SkipContainer: true, Run: []string{"host.sh"}}
		if err := runStepPreScripts(o, 0, step); err != nil {
			t.Fatalf("runStepPreScripts: %v", err)
		}
		if !called {
			t.Fatal("expected RunHostScript for skip_container")
		}
	})
}
