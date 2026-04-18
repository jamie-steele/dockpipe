package application

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	cmd := exec.Command("bash", "-lc", ". \""+envPath+"\"; test \"$PIPELANG_IMAGE\" = \"nginx\"; test \"$PIPELANG_REPLICAS\" = \"1\"; echo ok")
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
