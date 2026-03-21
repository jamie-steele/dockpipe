package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdInitDogfoodFlags copies bundled dogfood workflows when flags are set (uses real repo templates).
func TestCmdInitDogfoodFlags(t *testing.T) {
	repoRoot := testRepoRoot(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"--dogfood-test", "--dogfood-codex-pav"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	for _, name := range []string{"test", "dogfood-codex-pav"} {
		p := filepath.Join(project, "dockpipe", "workflows", name, "config.yml")
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(project, "dockpipe", "workflows", "dogfood-codex-pav", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "resolver: codex") || !strings.Contains(string(b), "runtime: docker") {
		t.Fatalf("expected codex resolver and docker runtime in dogfood-codex-pav config:\n%s", string(b))
	}
}

// TestCmdInitDogfoodSkipsExistingDir does not overwrite an existing templates/<name>/.
func TestCmdInitDogfoodSkipsExistingDir(t *testing.T) {
	repoRoot := testRepoRoot(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	existing := filepath.Join(project, "dockpipe", "workflows", "test", "config.yml")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("name: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdInit([]string{"--dogfood-test"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "name: test\n" {
		t.Fatalf("dogfood must not overwrite existing dockpipe/workflows/test: %q", string(b))
	}
}
