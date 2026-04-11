package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestMaybeRemoveStrayDashInjectFile_RemovesEnvLikeFile(t *testing.T) {
	tmp := t.TempDir()
	dash := filepath.Join(tmp, "-")
	if err := os.WriteFile(dash, []byte("FOO=bar\n# c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	maybeRemoveStrayDashInjectFile(tmp)
	if _, err := os.Stat(dash); err == nil {
		t.Fatal("expected stray - removed")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestMaybeRemoveStrayDashInjectFile_KeepEnv(t *testing.T) {
	t.Setenv("DOCKPIPE_KEEP_DASH_FILE", "1")
	tmp := t.TempDir()
	dash := filepath.Join(tmp, "-")
	content := []byte("FOO=bar\n")
	if err := os.WriteFile(dash, content, 0o644); err != nil {
		t.Fatal(err)
	}
	maybeRemoveStrayDashInjectFile(tmp)
	b, err := os.ReadFile(dash)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(content) {
		t.Fatalf("file changed: %q", b)
	}
}

func TestMaybeRemoveStrayDashInjectFile_SkipsNoEquals(t *testing.T) {
	tmp := t.TempDir()
	dash := filepath.Join(tmp, "-")
	if err := os.WriteFile(dash, []byte("not env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	maybeRemoveStrayDashInjectFile(tmp)
	if _, err := os.Stat(dash); err != nil {
		t.Fatal(err)
	}
}

func TestMaybeRemoveStrayDashInjectFile_SkipsBinary(t *testing.T) {
	tmp := t.TempDir()
	dash := filepath.Join(tmp, "-")
	if err := os.WriteFile(dash, []byte("FOO=bar\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	maybeRemoveStrayDashInjectFile(tmp)
	if _, err := os.Stat(dash); err != nil {
		t.Fatal(err)
	}
}

func TestMergeOpInjectFromProjectIfEnabled_MergesVaultKeys(t *testing.T) {
	t.Setenv("DOCKPIPE_OP_INJECT", "1")
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "secrets": { "op_inject_template": ".env.op.template" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env.op.template"), []byte("# x\nVAULT_K=resolved\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldRun := runOpInjectFn
	oldLook := opLookPathFn
	defer func() {
		runOpInjectFn = oldRun
		opLookPathFn = oldLook
	}()
	opLookPathFn = func(string) (string, error) { return "/fake/op", nil }
	runOpInjectFn = func(string) ([]byte, error) {
		return []byte("VAULT_K=injected\nOTHER=2\n"), nil
	}

	env := map[string]string{"PRE": "1"}
	opts := &CliOpts{Workdir: tmp}
	if err := mergeOpInjectFromProjectIfEnabled(env, opts, filepath.Join(tmp, "wf"), nil); err != nil {
		t.Fatal(err)
	}
	if env["VAULT_K"] != "injected" || env["OTHER"] != "2" || env["PRE"] != "1" {
		t.Fatalf("env: %#v", env)
	}
}

func TestMergeOpInjectFromProjectIfEnabled_SkipsWithNoOpInject(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "secrets": { "op_inject_template": ".env.op.template" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env.op.template"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dash := filepath.Join(tmp, "-")
	if err := os.WriteFile(dash, []byte("ACCIDENTAL=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldRun := runOpInjectFn
	defer func() { runOpInjectFn = oldRun }()
	runOpInjectFn = func(string) ([]byte, error) {
		t.Fatal("op inject should not run")
		return nil, nil
	}

	env := map[string]string{}
	opts := &CliOpts{Workdir: tmp, NoOpInject: true}
	if err := mergeOpInjectFromProjectIfEnabled(env, opts, tmp, nil); err != nil {
		t.Fatal(err)
	}
	if len(env) != 0 {
		t.Fatalf("expected no merge, got %#v", env)
	}
	if _, err := os.Stat(dash); err == nil {
		t.Fatal("expected stray - removed even when op inject is skipped")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestMergeOpInjectFromProjectIfEnabled_SkipsWhenProjectVaultNone(t *testing.T) {
	t.Setenv("DOCKPIPE_OP_INJECT", "1")
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "secrets": { "op_inject_template": ".env.op.template", "vault": "none" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env.op.template"), []byte("SHOULD_NOT=run\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldRun := runOpInjectFn
	defer func() { runOpInjectFn = oldRun }()
	runOpInjectFn = func(string) ([]byte, error) {
		t.Fatal("op inject should not run when secrets.vault is none")
		return nil, nil
	}

	env := map[string]string{}
	opts := &CliOpts{Workdir: tmp}
	if err := mergeOpInjectFromProjectIfEnabled(env, opts, tmp, nil); err != nil {
		t.Fatal(err)
	}
	if len(env) != 0 {
		t.Fatalf("expected no merge, got %#v", env)
	}
}

func TestMergeOpInjectFromProjectIfEnabled_WorkflowVaultOverridesProjectNone(t *testing.T) {
	t.Setenv("DOCKPIPE_OP_INJECT", "1")
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "secrets": { "op_inject_template": ".env.op.template", "vault": "none" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env.op.template"), []byte("K=from_op\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldRun := runOpInjectFn
	oldLook := opLookPathFn
	defer func() {
		runOpInjectFn = oldRun
		opLookPathFn = oldLook
	}()
	opLookPathFn = func(string) (string, error) { return "/fake/op", nil }
	runOpInjectFn = func(string) ([]byte, error) {
		return []byte("K=injected\n"), nil
	}

	env := map[string]string{}
	opts := &CliOpts{Workdir: tmp}
	wf := &domain.Workflow{Vault: "op"}
	if err := mergeOpInjectFromProjectIfEnabled(env, opts, tmp, wf); err != nil {
		t.Fatal(err)
	}
	if env["K"] != "injected" {
		t.Fatalf("workflow vault: op should override project none: %#v", env)
	}
}
