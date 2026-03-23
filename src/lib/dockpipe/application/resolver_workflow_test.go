package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/dockpipe/infrastructure"
)

func TestRunEmbeddedResolverWorkflowWithLoad_CallsRunSteps(t *testing.T) {
	repoRoot := t.TempDir()
	wfDir := filepath.Join(repoRoot, "templates", "cursor-dev")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// skip_container avoids AnyContainerStep / Docker preflight in CI/sandbox.
	yml := "name: x\nsteps:\n  - cmd: echo x\n    skip_container: true\n"
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	runSteps := func(o runStepsOpts) error {
		called = true
		if o.wfRoot != wfDir || o.repoRoot != repoRoot {
			t.Fatalf("unexpected runSteps opts wfRoot=%q repoRoot=%q", o.wfRoot, o.repoRoot)
		}
		return nil
	}
	env := map[string]string{}
	opts := &CliOpts{}
	err := runEmbeddedResolverWorkflowWithLoad(infrastructure.LoadWorkflow, runSteps, "cursor-dev", repoRoot, env, opts, nil, nil, "", "", "cursor", "")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected runSteps to run")
	}
}
