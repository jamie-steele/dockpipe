package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

func withRunStepSeams(t *testing.T, fn func()) {
	t.Helper()
	oldBuild := dockerBuildFn
	oldRun := runContainerFn
	oldSource := sourceHostScriptFn
	oldStat := osStatFn
	oldGetwd := getwdFn
	t.Cleanup(func() {
		dockerBuildFn = oldBuild
		runContainerFn = oldRun
		sourceHostScriptFn = oldSource
		osStatFn = oldStat
		getwdFn = oldGetwd
	})
	fn()
}

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

