package mcpbridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveExecWorkdirRestrict(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", tmp)
	// Restriction defaults to on; do not opt out.

	got, err := resolveExecWorkdir("")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(tmp) {
		t.Fatalf("empty workdir: got %q want %q", got, tmp)
	}

	_, err = resolveExecWorkdir("/tmp")
	if err == nil {
		t.Fatal("expected error for workdir outside repo")
	}
}

func TestResolveExecWorkdirMapsPipeonContainerWorkdir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_WORKDIR", tmp)
	// Restriction defaults to on; do not opt out.

	got, err := resolveExecWorkdir("/work")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(tmp) {
		t.Fatalf("/work: got %q want %q", got, tmp)
	}

	nested, err := resolveExecWorkdir("/work/packages")
	if err != nil {
		t.Fatal(err)
	}
	wantNested := filepath.Join(tmp, "packages")
	if filepath.Clean(nested) != filepath.Clean(wantNested) {
		t.Fatalf("/work/packages: got %q want %q", nested, wantNested)
	}
}

func TestResolveExecWorkdirOptOut(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_RESTRICT_WORKDIR", "0")
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", tmp)
	t.Chdir(tmp)
	// /usr is absolute on Unix but not on Windows (filepath.IsAbs); use a path outside the repo on every OS.
	outside, err := os.MkdirTemp(filepath.Dir(tmp), "mcp-workdir-outside-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	want, err := filepath.Abs(outside)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveExecWorkdir(want)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCleanHostProviderStdoutDropsDockerDigestOnlyLines(t *testing.T) {
	digest := "sha256:c662a401a89df917d43ce9fa716d3514176f9f2302ab355cd2dbb75393041bb1"
	got := cleanHostProviderStdout(digest + "\nClaude response\n")
	if got != "Claude response" {
		t.Fatalf("got %q", got)
	}
	if got := cleanHostProviderStdout(digest + "\n"); got != "" {
		t.Fatalf("digest-only output should be empty, got %q", got)
	}
}

func TestClaudeAuthRequiredDetectsAuthFailures(t *testing.T) {
	if !claudeAuthRequired("", "Error: not authenticated. Please run claude login.", "", 1) {
		t.Fatal("expected auth failure detection")
	}
	if claudeAuthRequired("", "some runtime failure", "", 1) {
		t.Fatal("did not expect generic runtime failure to be auth-required")
	}
	if claudeAuthRequired("", "not authenticated", "", 0) {
		t.Fatal("successful runs should not request auth")
	}
}

func TestEnvWithOverridesReplacesExistingCaseInsensitively(t *testing.T) {
	got := envWithOverrides([]string{"DOCKPIPE_REPO_ROOT=old", "Path=x"}, map[string]string{
		"dockpipe_repo_root": "new",
		"DOCKPIPE_OP_INJECT": "0",
	})
	joined := "\n" + strings.Join(got, "\n") + "\n"
	if !strings.Contains(joined, "\ndockpipe_repo_root=new\n") {
		t.Fatalf("override missing: %#v", got)
	}
	if strings.Contains(joined, "\nDOCKPIPE_REPO_ROOT=old\n") {
		t.Fatalf("old value was not replaced: %#v", got)
	}
	if !strings.Contains(joined, "\nDOCKPIPE_OP_INJECT=0\n") {
		t.Fatalf("new value missing: %#v", got)
	}
}

func TestProviderAuthStatusClaudeUsesHostConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	status := providerAuthStatusFor("claude")
	if !status.Authenticated {
		t.Fatalf("expected Claude auth from host config, got %#v", status)
	}
	if status.AuthDir == "" {
		t.Fatalf("expected auth dir, got %#v", status)
	}
}

func TestProviderAuthStatusClaudeUsesAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	status := providerAuthStatusFor("claude")
	if !status.Authenticated {
		t.Fatalf("expected Claude auth from API key, got %#v", status)
	}
	if len(status.EnvKeys) != 1 || status.EnvKeys[0] != "ANTHROPIC_API_KEY" {
		t.Fatalf("expected API key env marker, got %#v", status.EnvKeys)
	}
}

func TestClaudeAuthCommandIsDirectHostCLI(t *testing.T) {
	got := claudeAuthCommand("C:\\Source\\dockpipe")
	if strings.Contains(got, "--workflow claude") || strings.Contains(got, " dockpipe ") || strings.Contains(got, "& 'dockpipe'") {
		t.Fatalf("auth command should not route through DockPipe workflow: %q", got)
	}
	if !strings.Contains(got, "claude auth login") {
		t.Fatalf("auth command should use direct Claude host auth: %q", got)
	}
}

func TestClaudeDockpipeArgsSelectResolverProfile(t *testing.T) {
	got := strings.Join(claudeDockpipeArgs(`C:\Source\dockpipe`), " ")
	if !strings.Contains(got, "--workflow claude") {
		t.Fatalf("expected claude workflow args, got %q", got)
	}
	if !strings.Contains(got, "--resolver claude") {
		t.Fatalf("expected claude resolver profile for auth mounts, got %q", got)
	}
}
