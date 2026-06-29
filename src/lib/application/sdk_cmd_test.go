package application

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(buf.String()), runErr
}

func TestCmdScopeWorkflowAndPackageObjects(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("DOCKPIPE_WORKDIR", wd)
	t.Setenv("DOCKPIPE_BIN", filepath.Join(wd, "dockpipe"))
	t.Setenv("DOCKPIPE_WORKFLOW_NAME", "Doctor/Check")
	t.Setenv("DOCKPIPE_SOURCE_ROOT", wd)
	t.Setenv("DOCKPIPE_ARTIFACT_ROOT", "")
	t.Setenv("DOCKPIPE_OUTPUT_ROOT", "")

	gotWorkflow, err := captureStdout(t, func() error {
		return cmdScope(nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	var wf map[string]string
	if err := json.Unmarshal([]byte(gotWorkflow), &wf); err != nil {
		t.Fatalf("workflow scope should be json: %v\n%s", err, gotWorkflow)
	}
	wantArtifact := filepath.Join(wd, "bin", ".dockpipe", "workflows", "Doctor-Check", "artifacts")
	if wf["kind"] != "workflow" || wf["root"] != wantArtifact || wf["output_root"] != wantArtifact {
		t.Fatalf("unexpected workflow scope: %#v", wf)
	}
	if wf["dockpipe_bin"] == "" {
		t.Fatalf("workflow scope missing dockpipe_bin: %#v", wf)
	}

	gotNamed, err := captureStdout(t, func() error {
		return cmdScope([]string{"name.123213"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wantArtifact, "scopes", "name.123213"); gotNamed != want {
		t.Fatalf("named scope = %q want %q", gotNamed, want)
	}

	gotOtherWorkflow, err := captureStdout(t, func() error {
		return cmdScope([]string{"workflow", "docs.orchestrate", "dorkpipe", "orchestrate"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "bin", ".dockpipe", "workflows", "docs.orchestrate", "artifacts", "dorkpipe", "orchestrate"); gotOtherWorkflow != want {
		t.Fatalf("workflow scope path = %q want %q", gotOtherWorkflow, want)
	}

	gotPackage, err := captureStdout(t, func() error {
		return cmdScope([]string{"--package", "MyPackage"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]string
	if err := json.Unmarshal([]byte(gotPackage), &pkg); err != nil {
		t.Fatalf("package scope should be json: %v\n%s", err, gotPackage)
	}
	if pkg["kind"] != "package" || pkg["scope"] != "mypackage" || pkg["root"] != filepath.Join(wd, "bin", ".dockpipe", "packages", "mypackage") {
		t.Fatalf("unexpected package scope: %#v", pkg)
	}
	if pkg["dockpipe_bin"] == "" {
		t.Fatalf("package scope missing dockpipe_bin: %#v", pkg)
	}

	gotPackagePath, err := captureStdout(t, func() error {
		return cmdScope([]string{"--package", "MyPackage", "training", "metrics.jsonl"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "bin", ".dockpipe", "packages", "mypackage", "training", "metrics.jsonl"); gotPackagePath != want {
		t.Fatalf("package scope path = %q want %q", gotPackagePath, want)
	}
}

func TestCmdGetStateFields(t *testing.T) {
	wd := t.TempDir()
	pkgRoot := filepath.Join(wd, "packages", "Pipeon Dev")
	t.Setenv("DOCKPIPE_WORKDIR", wd)
	t.Setenv("DOCKPIPE_PACKAGE_ROOT", pkgRoot)
	t.Setenv("DOCKPIPE_PACKAGE_ID", "")
	t.Setenv("DOCKPIPE_PACKAGE_STATE_DIR", "")
	t.Setenv("DOCKPIPE_STATE_DIR", "")
	t.Setenv("DOCKPIPE_WORKFLOW_NAME", "Doctor/Check")
	t.Setenv("DOCKPIPE_ARTIFACT_ROOT", "")
	t.Setenv("DOCKPIPE_OUTPUT_ROOT", "")

	gotState, err := captureStdout(t, func() error {
		return cmdGet([]string{"state_dir"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "bin", ".dockpipe"); gotState != want {
		t.Fatalf("state_dir = %q want %q", gotState, want)
	}

	gotID, err := captureStdout(t, func() error {
		return cmdGet([]string{"package_id"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotID != "pipeon-dev" {
		t.Fatalf("package_id = %q want pipeon-dev", gotID)
	}

	gotPackageState, err := captureStdout(t, func() error {
		return cmdGet([]string{"package_state_dir"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "bin", ".dockpipe", "packages", "pipeon-dev"); gotPackageState != want {
		t.Fatalf("package_state_dir = %q want %q", gotPackageState, want)
	}

	gotArtifactRoot, err := captureStdout(t, func() error {
		return cmdGet([]string{"artifact_root"})
	})
	if err != nil {
		t.Fatal(err)
	}
	wantArtifactRoot := filepath.Join(wd, "bin", ".dockpipe", "workflows", "Doctor-Check", "artifacts")
	if gotArtifactRoot != wantArtifactRoot {
		t.Fatalf("artifact_root = %q want %q", gotArtifactRoot, wantArtifactRoot)
	}

	t.Setenv("DOCKPIPE_OUTPUT_ROOT", filepath.Join(wd, "custom-output"))
	gotOutputRoot, err := captureStdout(t, func() error {
		return cmdGet([]string{"output_root"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "custom-output"); gotOutputRoot != want {
		t.Fatalf("output_root = %q want %q", gotOutputRoot, want)
	}
}

func TestCmdScopeResolverAuthFields(t *testing.T) {
	wd := t.TempDir()
	t.Setenv("DOCKPIPE_WORKDIR", wd)
	t.Setenv("HOME", filepath.Join(wd, "home"))
	if err := os.MkdirAll(filepath.Join(wd, "packages", "agent", "resolvers", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "dockpipe.config.json"), []byte(`{"schema":1,"compile":{"workflows":["packages"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := strings.Join([]string{
		"DOCKPIPE_RESOLVER_AUTH_DIR_ENV=CODEX_HOME",
		"DOCKPIPE_RESOLVER_AUTH_DIR=.codex",
		"DOCKPIPE_RESOLVER_CONTAINER_AUTH_DIR=/home/node/.codex",
		"DOCKPIPE_RESOLVER_AUTH_MOUNT_MODE=ro",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(wd, "packages", "agent", "resolvers", "codex", "profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}

	gotDefault, err := captureStdout(t, func() error {
		return cmdScope([]string{"resolver", "codex", "auth-dir"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "home", ".codex"); gotDefault != want {
		t.Fatalf("auth-dir = %q want %q", gotDefault, want)
	}

	t.Setenv("CODEX_HOME", filepath.Join(wd, "custom-codex"))
	gotEnv, err := captureStdout(t, func() error {
		return cmdScope([]string{"resolver", "codex", "auth-dir"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "custom-codex"); gotEnv != want {
		t.Fatalf("auth-dir env = %q want %q", gotEnv, want)
	}

	gotContainer, err := captureStdout(t, func() error {
		return cmdScope([]string{"resolver", "codex", "container-auth-dir"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotContainer != "/home/node/.codex" {
		t.Fatalf("container-auth-dir = %q", gotContainer)
	}

	gotMode, err := captureStdout(t, func() error {
		return cmdScope([]string{"resolver", "codex", "auth-mount-mode"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMode != "ro" {
		t.Fatalf("auth-mount-mode = %q", gotMode)
	}
}
