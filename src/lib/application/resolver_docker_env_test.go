package application

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestMergeEnvHintKeys(t *testing.T) {
	t.Run("extra env wins over host", func(t *testing.T) {
		dst := map[string]string{"OPENAI_API_KEY": "from-cli"}
		src := map[string]string{"OPENAI_API_KEY": "from-host"}
		mergeEnvHintKeys(dst, src, "OPENAI_API_KEY")
		if dst["OPENAI_API_KEY"] != "from-cli" {
			t.Fatalf("expected cli value kept, got %#v", dst)
		}
	})
	t.Run("host fills when dst empty", func(t *testing.T) {
		dst := map[string]string{}
		src := map[string]string{"OPENAI_API_KEY": "from-host"}
		mergeEnvHintKeys(dst, src, "OPENAI_API_KEY")
		if dst["OPENAI_API_KEY"] != "from-host" {
			t.Fatalf("expected host copy, got %#v", dst)
		}
	})
	t.Run("comma-separated hints", func(t *testing.T) {
		dst := map[string]string{}
		src := map[string]string{"ANTHROPIC_API_KEY": "a", "CLAUDE_API_KEY": "c"}
		mergeEnvHintKeys(dst, src, "ANTHROPIC_API_KEY, CLAUDE_API_KEY")
		if dst["ANTHROPIC_API_KEY"] != "a" || dst["CLAUDE_API_KEY"] != "c" {
			t.Fatalf("expected both, got %#v", dst)
		}
	})
	t.Run("nil ra path is no-op in mergeResolverAuthEnvFromHost", func(t *testing.T) {
		dst := map[string]string{}
		src := map[string]string{"OPENAI_API_KEY": "x"}
		mergeResolverAuthEnvFromHost(dst, src, nil)
		if len(dst) != 0 {
			t.Fatalf("expected no keys, got %#v", dst)
		}
	})
}

func TestResolverAuthMountSpecs(t *testing.T) {
	home := t.TempDir()
	authDir := filepath.Join(home, ".claude")
	configFile := filepath.Join(home, ".claude.json")
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configFile, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ra := &domain.ResolverAssignments{
		AuthDirEnv:          "CLAUDE_HOME",
		AuthDir:             ".claude",
		ContainerAuthDir:    "/home/node/.claude",
		AuthMountMode:       "ro",
		ConfigFile:          ".claude.json",
		ContainerConfigFile: "/home/node/.claude.json",
	}
	got := resolverAuthMountSpecs(ra, map[string]string{})
	want := []string{
		filepath.Clean(authDir) + ":/home/node/.claude:ro",
		filepath.Clean(configFile) + ":/home/node/.claude.json:ro",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts got %#v want %#v", got, want)
	}
}

func TestMergeResolverAuthEnvFromHostSetsContainerAuthDir(t *testing.T) {
	dst := map[string]string{}
	src := map[string]string{}
	ra := &domain.ResolverAssignments{
		AuthDirEnv:       "CLAUDE_HOME",
		ContainerAuthDir: "/home/node/.claude",
	}
	mergeResolverAuthEnvFromHost(dst, src, ra)
	if dst["CLAUDE_HOME"] != "/home/node/.claude" {
		t.Fatalf("CLAUDE_HOME got %q", dst["CLAUDE_HOME"])
	}
}
