package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIsolationProfileMergesRuntimeThenResolver(t *testing.T) {
	repo := t.TempDir()
	rtDir := filepath.Join(repo, "templates", "core", "runtimes")
	rsDir := filepath.Join(repo, "templates", "core", "resolvers")
	_ = os.MkdirAll(rtDir, 0o755)
	_ = os.MkdirAll(rsDir, 0o755)
	if err := os.WriteFile(filepath.Join(rtDir, "r1"), []byte("DOCKPIPE_RUNTIME_WORKFLOW=wf1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, "s1"), []byte("DOCKPIPE_RESOLVER_CMD=cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadIsolationProfile(repo, "r1", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if m["DOCKPIPE_RUNTIME_WORKFLOW"] != "wf1" || m["DOCKPIPE_RESOLVER_CMD"] != "cli" {
		t.Fatalf("merged map: %#v", m)
	}
}

func TestLoadIsolationProfileSameNamePairs(t *testing.T) {
	repo := t.TempDir()
	rtDir := filepath.Join(repo, "templates", "core", "runtimes")
	rsDir := filepath.Join(repo, "templates", "core", "resolvers")
	_ = os.MkdirAll(rtDir, 0o755)
	_ = os.MkdirAll(rsDir, 0o755)
	if err := os.WriteFile(filepath.Join(rtDir, "both"), []byte("DOCKPIPE_RUNTIME_TYPE=execution\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, "both"), []byte("DOCKPIPE_RESOLVER_CMD=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadIsolationProfile(repo, "both", "")
	if err != nil {
		t.Fatal(err)
	}
	if m["DOCKPIPE_RUNTIME_TYPE"] != "execution" || m["DOCKPIPE_RESOLVER_CMD"] != "x" {
		t.Fatalf("merged: %#v", m)
	}
}

func writeRuntimeFile(t *testing.T, repo, name, body string) {
	t.Helper()
	p := filepath.Join(repo, "templates", "core", "runtimes", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeResolverFile(t *testing.T, repo, name, body string) {
	t.Helper()
	p := filepath.Join(repo, "templates", "core", "resolvers", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadIsolationProfileRepresentativePairs exercises runtime × resolver combinations without Docker.
// Each subtest uses its own temp repo so shared names (e.g. docker) do not overwrite prior rows.
func TestLoadIsolationProfileRepresentativePairs(t *testing.T) {
	cases := []struct {
		name   string
		rt, rs string
		rtBody string
		rsBody string
	}{
		{"dockerimage+claude", "dockerimage", "claude", "DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n", "DOCKPIPE_RESOLVER_TEMPLATE=claude\n"},
		{"dockerimage+codex", "dockerimage", "codex", "DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n"},
		{"dockerimage+vscode", "dockerimage", "vscode", "DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n", "DOCKPIPE_RESOLVER_WORKFLOW=vscode\n"},
		{"dockerimage+code-server", "dockerimage", "code-server", "DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n", "DOCKPIPE_RESOLVER_TEMPLATE=code-server\n"},
		{"dockerimage+cursor-dev", "dockerimage", "cursor-dev", "DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n", "DOCKPIPE_RESOLVER_CMD=cursor-dev\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			writeRuntimeFile(t, repo, tc.rt, tc.rtBody)
			writeResolverFile(t, repo, tc.rs, tc.rsBody)
			m, err := LoadIsolationProfile(repo, tc.rt, tc.rs)
			if err != nil {
				t.Fatalf("LoadIsolationProfile: %v", err)
			}
			if got := m["DOCKPIPE_RUNTIME_SUBSTRATE"]; got == "" {
				t.Fatalf("missing runtime substrate in %#v", m)
			}
			// Resolver-side signal must be present (template, workflow, or cmd).
			if m["DOCKPIPE_RESOLVER_TEMPLATE"] == "" && m["DOCKPIPE_RESOLVER_WORKFLOW"] == "" && m["DOCKPIPE_RESOLVER_CMD"] == "" {
				t.Fatalf("missing resolver keys in %#v", m)
			}
		})
	}
}

func TestLoadIsolationProfileNoProfilesErrors(t *testing.T) {
	repo := t.TempDir()
	_, err := LoadIsolationProfile(repo, "missing-rt", "missing-rs")
	if err == nil {
		t.Fatal("expected error when no profile files exist")
	}
}

// TestLoadIsolationProfileExplicitPairDoesNotCrossRead verifies that with an explicit runtime+resolver
// pair, only templates/core/runtimes/<runtime> and templates/core/resolvers/<resolver> participate
// (resolver-only file under runtimes/ must not satisfy the resolver name).
func TestLoadIsolationProfileExplicitPairDoesNotCrossRead(t *testing.T) {
	repo := t.TempDir()
	// Wrong: only a "codex" file under runtimes/ — must not count as resolver "codex".
	writeRuntimeFile(t, repo, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=should-not-use\n")
	writeRuntimeFile(t, repo, "dockerimage", "DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n")
	writeResolverFile(t, repo, "codex", "DOCKPIPE_RESOLVER_TEMPLATE=codex\n")
	m, err := LoadIsolationProfile(repo, "dockerimage", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if m["DOCKPIPE_RESOLVER_TEMPLATE"] != "codex" {
		t.Fatalf("resolver merge: want codex from resolvers/, got %#v", m)
	}
	if m["DOCKPIPE_RUNTIME_SUBSTRATE"] != "dockerimage" {
		t.Fatalf("runtime merge: %#v", m)
	}
}
