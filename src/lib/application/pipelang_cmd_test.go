package application

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const samplePipeLang = `Interface DeployConfig
{
    string Image;
    int Replicas;
    bool Public;
    string FullImage();
}

Class DefaultDeployConfig : DeployConfig
{
    string Image = "nginx";
    int Replicas = 1;
    bool Public = false;

    string FullImage() => Image + ":latest";
    bool IsScaled(int threshold) => Replicas > threshold;
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
