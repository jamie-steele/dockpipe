package infrastructure

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func withComposeSeams(t *testing.T) {
	t.Helper()
	old := composeExecCommandFn
	t.Cleanup(func() {
		composeExecCommandFn = old
	})
}

func TestRunComposeLifecycleUpBuildsArgs(t *testing.T) {
	withComposeSeams(t)
	var gotName string
	var gotArgs []string
	composeExecCommandFn = func(_ context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return exec.Command("bash", "-c", "exit 0")
	}
	err := RunComposeLifecycle(ComposeLifecycleOpts{
		Action:           "up",
		File:             "/wf/assets/compose/docker-compose.yml",
		Project:          "dockpipe-dev",
		ProjectDirectory: "/repo",
		Services:         []string{"proxy", "mcp"},
		Env:              []string{"MCP_HTTP_URL=http://127.0.0.1:8766", "DATABASE_URL=postgres://local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "docker" {
		t.Fatalf("expected docker command, got %q", gotName)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{
		"compose",
		"-p dockpipe-dev",
		"-f /wf/assets/compose/docker-compose.yml",
		"--project-directory /repo",
		"up -d --remove-orphans",
		"proxy",
		"mcp",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected args to contain %q, got %s", want, joined)
		}
	}
	merged := strings.Join(mergeComposeProcessEnv([]string{"MCP_HTTP_URL=http://127.0.0.1:8766", "DATABASE_URL=postgres://local"}), "\n")
	for _, want := range []string{"MCP_HTTP_URL=http://127.0.0.1:8766", "DATABASE_URL=postgres://local"} {
		if !strings.Contains(merged, want) {
			t.Fatalf("expected merged env to contain %q, got %s", want, merged)
		}
	}
}

func TestRunComposeLifecycleDefaultsProjectDirectoryFromFile(t *testing.T) {
	withComposeSeams(t)
	var gotArgs []string
	composeExecCommandFn = func(_ context.Context, name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		return exec.Command("bash", "-c", "exit 0")
	}
	if err := RunComposeLifecycle(ComposeLifecycleOpts{
		Action: "ps",
		File:   "/wf/assets/compose/docker-compose.yml",
	}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "--project-directory /wf/assets/compose") {
		t.Fatalf("expected default project directory from compose file, got %s", joined)
	}
	if !strings.Contains(joined, " ps") {
		t.Fatalf("expected ps action, got %s", joined)
	}
}

func TestRunComposeLifecycleRejectsUnknownAction(t *testing.T) {
	withComposeSeams(t)
	if err := RunComposeLifecycle(ComposeLifecycleOpts{
		Action: "logs",
		File:   "/wf/assets/compose/docker-compose.yml",
	}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMergeComposeProcessEnvOverridesKeys(t *testing.T) {
	t.Setenv("MCP_HTTP_URL", "http://old")
	merged := strings.Join(mergeComposeProcessEnv([]string{"MCP_HTTP_URL=http://new", "NEW_KEY=value"}), "\n")
	for _, want := range []string{"MCP_HTTP_URL=http://new", "NEW_KEY=value"} {
		if !strings.Contains(merged, want) {
			t.Fatalf("expected merged env to contain %q, got %s", want, merged)
		}
	}
}
