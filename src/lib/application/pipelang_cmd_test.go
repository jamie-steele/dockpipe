package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

const samplePipeLang = `public Interface DeployConfig
{
    public string Image;
    public int Replicas;
    public bool Public;
    public string FullImage();
}

public Class DefaultDeployConfig : DeployConfig
{
    public string Image = "nginx";
    public int Replicas = 1;
    public bool Public = false;
    private string InternalSuffix = ":latest";

    public string FullImage() => Image + InternalSuffix;
    public bool IsScaled(int threshold) => Replicas > threshold;
    private bool IsTiny() => Replicas < 1;
}
`

func TestCmdPipeLangCompileAndInvoke(t *testing.T) {
	wd := t.TempDir()
	in := filepath.Join(wd, "demo.pipe")
	if err := os.WriteFile(in, []byte(samplePipeLang), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(wd, "out")
	if err := cmdPipeLang([]string{"compile", "--in", in, "--entry", "DefaultDeployConfig", "--out", outDir}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, p := range []string{
		filepath.Join(outDir, "DefaultDeployConfig.workflow.yml"),
		filepath.Join(outDir, "DefaultDeployConfig.bindings.json"),
		filepath.Join(outDir, "DefaultDeployConfig.bindings.env"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing output %s: %v", p, err)
		}
	}

	if err := cmdPipeLang([]string{"invoke", "--in", in, "--class", "DefaultDeployConfig", "--method", "IsScaled", "--arg", "0"}); err != nil {
		t.Fatalf("invoke: %v", err)
	}
}

func TestCmdPipeLangCompileDefaultOutputUsesDockpipeStateRoot(t *testing.T) {
	wd := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	in := filepath.Join(wd, "demo.pipe")
	if err := os.WriteFile(in, []byte(samplePipeLang), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPipeLang([]string{"compile", "--in", in, "--entry", "DefaultDeployConfig"}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	want := filepath.Join(wd, "bin", ".dockpipe", "pipelang", "DefaultDeployConfig.workflow.yml")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("missing default output %s: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(wd, ".dockpipe", "pipelang", "DefaultDeployConfig.workflow.yml")); !os.IsNotExist(err) {
		t.Fatalf("did not expect legacy .dockpipe output, stat err=%v", err)
	}
}

func TestPipeLangBindingsEnvConsumableByScript(t *testing.T) {
	wd := t.TempDir()
	in := filepath.Join(wd, "demo.pipe")
	if err := os.WriteFile(in, []byte(samplePipeLang), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(wd, "out")
	if err := cmdPipeLang([]string{"compile", "--in", in, "--out", outDir}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	envPath := filepath.Join(outDir, "DefaultDeployConfig.bindings.env")
	bashCmd, _, err := dockpipeBashShellCommand(". \"" + filepath.ToSlash(envPath) + "\"; test \"$PIPELANG_IMAGE\" = \"nginx\"; test \"$PIPELANG_REPLICAS\" = \"1\"; echo ok")
	if err != nil {
		t.Fatalf("resolve bash: %v", err)
	}
	cmd := bashCmd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script consume env: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("unexpected script output: %s", string(out))
	}
}

func TestPipeLangSplitFilesCompileAndInvoke(t *testing.T) {
	wd := t.TempDir()
	modelsDir := filepath.Join(wd, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	iface := `Interface IAppConfig { string Name; string FullName(); }`
	class := `Class AppConfig : IAppConfig { string Name = "dockpipe"; string FullName() => Name + "-cloud"; }`
	if err := os.WriteFile(filepath.Join(modelsDir, "iface.pipe"), []byte(iface), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "config.pipe"), []byte(class), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(wd, "out")
	if err := cmdPipeLang([]string{"compile", "--in", filepath.Join(wd, "config.pipe"), "--out", outDir}); err != nil {
		t.Fatalf("compile split files: %v", err)
	}
	if err := cmdPipeLang([]string{"invoke", "--in", filepath.Join(wd, "config.pipe"), "--method", "FullName"}); err != nil {
		t.Fatalf("invoke split files: %v", err)
	}
}

func TestPipeLangInvokePrivateMethodDenied(t *testing.T) {
	wd := t.TempDir()
	in := filepath.Join(wd, "demo.pipe")
	if err := os.WriteFile(in, []byte(samplePipeLang), 0o644); err != nil {
		t.Fatal(err)
	}
	err := cmdPipeLang([]string{"invoke", "--in", in, "--class", "DefaultDeployConfig", "--method", "IsTiny"})
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("expected private method invoke failure, got %v", err)
	}
}

func TestCmdPipeLangCompileMirrorsOperationEvent(t *testing.T) {
	wd := t.TempDir()
	in := filepath.Join(wd, "demo.pipe")
	if err := os.WriteFile(in, []byte(samplePipeLang), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(wd, "out")
	eventLog := filepath.Join(wd, "events.jsonl")
	t.Setenv(infrastructure.EnvDockpipeEventLog, eventLog)

	if _, err := captureResultStderr(t, func() error {
		return cmdPipeLang([]string{"compile", "--in", in, "--out", outDir})
	}); err != nil {
		t.Fatal(err)
	}
	events, err := infrastructure.ReadOperationEvents(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d want 1: %#v", len(events), events)
	}
	event := events[0]
	if event.Schema != infrastructure.OperationEventSchemaV1 || event.Type != infrastructure.OperationEventKind || event.Unit != "pipelang.compile" || event.Status != infrastructure.OperationStatusDone {
		t.Fatalf("unexpected event: %#v", event)
	}
	for key, want := range map[string]string{
		"input_path":         filepath.ToSlash(in),
		"output_dir":         filepath.ToSlash(outDir),
		"entry_class":        "DefaultDeployConfig",
		"workflow_path":      filepath.ToSlash(filepath.Join(outDir, "DefaultDeployConfig.workflow.yml")),
		"bindings_json_path": filepath.ToSlash(filepath.Join(outDir, "DefaultDeployConfig.bindings.json")),
		"bindings_env_path":  filepath.ToSlash(filepath.Join(outDir, "DefaultDeployConfig.bindings.env")),
		"result":             "compiled",
	} {
		if got := event.IDs[key]; got != want {
			t.Fatalf("event ID %s = %q want %q (event: %#v)", key, got, want, event)
		}
	}
}

func TestCmdPipeLangCompileFailureMirrorsOperationEvent(t *testing.T) {
	wd := t.TempDir()
	in := filepath.Join(wd, "missing.pipe")
	eventLog := filepath.Join(wd, "events.jsonl")
	t.Setenv(infrastructure.EnvDockpipeEventLog, eventLog)

	if _, err := captureResultStderr(t, func() error {
		return cmdPipeLang([]string{"compile", "--in", in, "--out", filepath.Join(wd, "out")})
	}); err == nil {
		t.Fatal("expected compile failure")
	}
	events, err := infrastructure.ReadOperationEvents(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d want 1: %#v", len(events), events)
	}
	event := events[0]
	if event.Schema != infrastructure.OperationEventSchemaV1 || event.Type != infrastructure.OperationEventKind || event.Unit != "pipelang.compile" || event.Status != infrastructure.OperationStatusFail {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.IDs["input_path"] != filepath.ToSlash(in) || event.IDs["result"] != "failed" {
		t.Fatalf("unexpected failure IDs: %#v", event.IDs)
	}
	if event.Error == "" {
		t.Fatalf("expected failure error: %#v", event)
	}
}
