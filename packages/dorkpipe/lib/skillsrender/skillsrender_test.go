package skillsrender

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunListAndDryRunGeneric(t *testing.T) {
	root := t.TempDir()
	assetsDir := filepath.Join(root, "assets")
	skillDir := filepath.Join(assetsDir, "skills", "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yml"), []byte(strings.Join([]string{
		"name: demo-skill",
		"description: Demo description",
		"short_description: Demo short",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "instructions.md"), []byte("Use the demo skill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"DOCKPIPE_ASSETS_DIR": assetsDir,
	}

	var listOut bytes.Buffer
	if err := Run([]string{"--list"}, env, &listOut, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := listOut.String(); !strings.Contains(got, "demo-skill\tDemo description") {
		t.Fatalf("unexpected list output %q", got)
	}

	outputDir := filepath.Join(root, "rendered")
	var dryRunOut bytes.Buffer
	if err := Run([]string{"--target", "generic", "--output", outputDir, "--dry-run"}, env, &dryRunOut, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := dryRunOut.String(); !strings.Contains(got, "would-render: demo-skill") {
		t.Fatalf("unexpected dry-run output %q", got)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create output directory, err=%v", err)
	}
}

func TestRunWritesGenericFiles(t *testing.T) {
	root := t.TempDir()
	assetsDir := filepath.Join(root, "assets")
	skillDir := filepath.Join(assetsDir, "skills", "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yml"), []byte(strings.Join([]string{
		"name: demo-skill",
		"description: Demo description",
		"short_description: Demo short",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "instructions.md"), []byte("Use the demo skill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(root, "rendered")
	env := map[string]string{
		"DOCKPIPE_ASSETS_DIR": assetsDir,
	}
	if err := Run([]string{"--target", "generic", "--output", outputDir}, env, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	skillJSON, err := os.ReadFile(filepath.Join(outputDir, "demo-skill", "skill.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]string
	if err := json.Unmarshal(skillJSON, &meta); err != nil {
		t.Fatal(err)
	}
	if meta["name"] != "demo-skill" || meta["description"] != "Demo description" {
		t.Fatalf("unexpected skill.json %#v", meta)
	}
}
