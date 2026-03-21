package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoRootUsesEnvOverride returns DOCKPIPE_REPO_ROOT when set.
func TestRepoRootUsesEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	override := filepath.Join(tmp, "repo-root")
	if err := os.MkdirAll(override, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_REPO_ROOT", override)

	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot error: %v", err)
	}
	want, _ := filepath.Abs(override)
	if got != want {
		t.Fatalf("RepoRoot() = %q, want %q", got, want)
	}
}

// TestLoadResolverFileParsesAssignments reads DOCKPIPE_RESOLVER_* lines from resolver env files.
func TestLoadResolverFileParsesAssignments(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "resolver.env")
	content := `
# comment
DOCKPIPE_RESOLVER_TEMPLATE=codex
DOCKPIPE_RESOLVER_PRE_SCRIPT = scripts/pre.sh
INVALID_LINE
DOCKPIPE_RESOLVER_ACTION = actions/do.sh
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadResolverFile(p)
	if err != nil {
		t.Fatalf("LoadResolverFile error: %v", err)
	}
	if m["DOCKPIPE_RESOLVER_TEMPLATE"] != "codex" {
		t.Fatalf("template mismatch: %#v", m)
	}
	if m["DOCKPIPE_RESOLVER_PRE_SCRIPT"] != "scripts/pre.sh" {
		t.Fatalf("pre-script mismatch: %#v", m)
	}
	if m["DOCKPIPE_RESOLVER_ACTION"] != "actions/do.sh" {
		t.Fatalf("action mismatch: %#v", m)
	}
	if _, ok := m["INVALID_LINE"]; ok {
		t.Fatalf("invalid line should be ignored: %#v", m)
	}
}

// TestSourceHostScriptExportsEnvironment runs a bash script and captures exported variables plus inherited env.
func TestSourceHostScriptExportsEnvironment(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "pre.sh")
	if err := os.WriteFile(script, []byte("export TEST_VAR=hello\nexport OTHER=world\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	m, err := SourceHostScript(script, []string{"BASE=yes"})
	if err != nil {
		t.Fatalf("SourceHostScript error: %v", err)
	}
	if m["TEST_VAR"] != "hello" || m["OTHER"] != "world" {
		t.Fatalf("missing exported vars: %#v", m)
	}
	if m["BASE"] != "yes" {
		t.Fatalf("expected inherited env BASE=yes, got %#v", m["BASE"])
	}
}

// TestCommitOnHostNoRepoReturnsNil skips git work when the workdir is not a git repository.
func TestCommitOnHostNoRepoReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitOnHost(tmp, "msg", "", false); err != nil {
		t.Fatalf("CommitOnHost should skip non-repo and return nil: %v", err)
	}
}

// TestCommitOnHostCreatesCommitAndBundle creates a commit and default (thin) bundle in a real git repo.
func TestCommitOnHostCreatesCommitAndBundle(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return string(out)
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Dockpipe Test")

	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(tmp, "repo.bundle")
	if err := CommitOnHost(tmp, "test commit", bundle, false); err != nil {
		t.Fatalf("CommitOnHost error: %v", err)
	}

	logOut := run("log", "--oneline", "-1")
	if !strings.Contains(logOut, "test commit") {
		t.Fatalf("expected commit message in log, got: %q", logOut)
	}
	if _, err := os.Stat(bundle); err != nil {
		t.Fatalf("expected bundle file %q: %v", bundle, err)
	}
}

// TestCommitOnHostBundleDefaultIsCurrentBranchOnly writes a bundle listing only the current branch head.
func TestCommitOnHostBundleDefaultIsCurrentBranchOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return string(out)
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Dockpipe Test")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "init")

	run("checkout", "-b", "side")
	if err := os.WriteFile(filepath.Join(tmp, "only-side.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "only-side.txt")
	run("commit", "-m", "side only")

	run("checkout", "main")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle := filepath.Join(tmp, "thin.bundle")
	if err := CommitOnHost(tmp, "update main", bundle, false); err != nil {
		t.Fatalf("CommitOnHost: %v", err)
	}

	out, err := exec.Command("git", "bundle", "list-heads", bundle).CombinedOutput()
	if err != nil {
		t.Fatalf("git bundle list-heads: %v\n%s", err, out)
	}
	heads := string(out)
	if strings.Contains(heads, "refs/heads/side") {
		t.Fatalf("thin bundle should not advertise side, got:\n%s", heads)
	}
	if !strings.Contains(heads, "refs/heads/main") {
		t.Fatalf("expected refs/heads/main in bundle heads, got:\n%s", heads)
	}
}

// TestCommitOnHostBundleAllIncludesEveryHead uses git bundle --all so every branch head is advertised.
func TestCommitOnHostBundleAllIncludesEveryHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return string(out)
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Dockpipe Test")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "init")

	run("checkout", "-b", "side")
	if err := os.WriteFile(filepath.Join(tmp, "only-side.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "only-side.txt")
	run("commit", "-m", "side only")

	run("checkout", "main")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle := filepath.Join(tmp, "fat.bundle")
	if err := CommitOnHost(tmp, "update main", bundle, true); err != nil {
		t.Fatalf("CommitOnHost: %v", err)
	}

	out, err := exec.Command("git", "bundle", "list-heads", bundle).CombinedOutput()
	if err != nil {
		t.Fatalf("git bundle list-heads: %v\n%s", err, out)
	}
	heads := string(out)
	if !strings.Contains(heads, "refs/heads/side") || !strings.Contains(heads, "refs/heads/main") {
		t.Fatalf("expected both branch heads with bundleAll, got:\n%s", heads)
	}
}
