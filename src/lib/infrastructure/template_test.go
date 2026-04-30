package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTemplateBuild maps template names to image names and Dockerfile directories under the repo.
func TestTemplateBuild(t *testing.T) {
	repoRoot := localModuleRoot(t)
	core := CoreDir(repoRoot)
	cases := []struct {
		name  string
		image string
		dir   string
		ok    bool
	}{
		{"base-dev", "dockpipe-base-dev", filepath.Join(core, "assets", "images", "base-dev"), true},
		{"dev", "dockpipe-dev", filepath.Join(core, "assets", "images", "dev"), true},
		{"agent-dev", "dockpipe-claude", DockerfileDir(repoRoot, "claude"), true},
		{"claude", "dockpipe-claude", DockerfileDir(repoRoot, "claude"), true},
		{"codex", "dockpipe-codex", DockerfileDir(repoRoot, "codex"), true},
		{"vscode", "dockpipe-vscode", DockerfileDir(repoRoot, "vscode"), true},
		{"ollama", "dockpipe-ollama", DockerfileDir(repoRoot, "ollama"), true},
		{"unknown", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img, dir, ok := TemplateBuild(repoRoot, tc.name)
			if img != tc.image || dir != tc.dir || ok != tc.ok {
				t.Fatalf("TemplateBuild(%q) = (%q, %q, %v), want (%q, %q, %v)", tc.name, img, dir, ok, tc.image, tc.dir, tc.ok)
			}
			if tc.ok {
				if st, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil || st.IsDir() {
					t.Fatalf("expected Dockerfile under %q, stat err=%v", dir, err)
				}
			}
		})
	}
}

// TestMaybeVersionTag appends dockpipe-* image tags from VERSION file when missing.
func TestMaybeVersionTag(t *testing.T) {
	tmp := t.TempDir()
	versionFile := filepath.Join(tmp, "version")
	if err := os.WriteFile(versionFile, []byte("0.6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := MaybeVersionTag(tmp, "dockpipe-claude"); got != "dockpipe-claude:0.6.0" {
		t.Fatalf("dockpipe image should be tagged, got %q", got)
	}
	if got := MaybeVersionTag(tmp, "ubuntu"); got != "ubuntu" {
		t.Fatalf("non-dockpipe image should not be tagged, got %q", got)
	}
	if got := MaybeVersionTag(tmp, "dockpipe-claude:custom"); got != "dockpipe-claude:custom" {
		t.Fatalf("pre-tagged image should not be changed, got %q", got)
	}
	if got := MaybeVersionTag(tmp, ""); got != "" {
		t.Fatalf("empty image should stay empty, got %q", got)
	}
}
