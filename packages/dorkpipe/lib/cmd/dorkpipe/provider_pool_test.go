package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProviderPoolPromptArgsUsesCanonicalWorkflowArgs(t *testing.T) {
	t.Setenv("DOCKPIPE_ARGS_JSON", `["--provider","ollama","--prompt","hello"]`)
	got := providerPoolPromptArgs(nil)
	want := []string{"--provider", "ollama", "--prompt", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%v want %v", got, want)
	}
}

func TestProviderPoolPromptArgsAppendsWorkflowArgsAfterScriptFlags(t *testing.T) {
	t.Setenv("DOCKPIPE_ARGS_JSON", `["--provider","ollama","--prompt","hello"]`)
	got := providerPoolPromptArgs([]string{"--workdir", "C:\\repo"})
	want := []string{"--workdir", "C:\\repo", "--provider", "ollama", "--prompt", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%v want %v", got, want)
	}
}

func TestProviderPoolShapeJSONTags(t *testing.T) {
	raw, err := json.Marshal(providerPoolProviderShape{
		MinReady:        1,
		MaxActive:       2,
		IdleTTLSeconds:  900,
		Role:            "direct",
		SessionAffinity: true,
		WarmMode:        "guarded_container",
		RequiresAuth:    true,
		WarmSource:      "docker-claude-resolver",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if got == "" || !containsAll(got,
		`"min_ready":1`,
		`"max_active":2`,
		`"idle_ttl_seconds":900`,
		`"session_affinity":true`,
		`"warm_mode":"guarded_container"`,
	) {
		t.Fatalf("unexpected json: %s", got)
	}
}

func TestProviderPoolDockpipeBinPrefersRepoLocalBinary(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "src", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(binDir, "dockpipe.exe")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := providerPoolDockpipeBin(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q want %q", got, path)
	}
}

func TestProviderPoolPromptWarmWaitTimeout(t *testing.T) {
	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "")
	if got := providerPoolPromptWarmWaitTimeout(); got != 10*time.Second {
		t.Fatalf("default timeout = %s, want 10s", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "0")
	if got := providerPoolPromptWarmWaitTimeout(); got != 0 {
		t.Fatalf("zero timeout = %s, want 0", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "3")
	if got := providerPoolPromptWarmWaitTimeout(); got != 3*time.Second {
		t.Fatalf("seconds timeout = %s, want 3s", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "1500ms")
	if got := providerPoolPromptWarmWaitTimeout(); got != 1500*time.Millisecond {
		t.Fatalf("duration timeout = %s, want 1500ms", got)
	}
}

func TestProviderPoolClaudeWarmBootstrapScriptUsesAllowlistAndPortableKeepalive(t *testing.T) {
	script := providerPoolClaudeWarmBootstrapScript()
	for _, name := range []string{
		".credentials.json",
		".last-cleanup",
		"history.jsonl",
		"ide",
		"mcp-needs-auth-cache.json",
		"plans",
		"plugins",
		"skills",
		"projects",
		"session-env",
		"sessions",
		"settings.json",
		"shell-snapshots",
		"policy-limits.json",
		"remote-settings.json",
	} {
		if !strings.Contains(script, name) {
			t.Fatalf("bootstrap script missing allowlisted path %q", name)
		}
	}
	if strings.Contains(script, "runuser") {
		t.Fatalf("bootstrap script should not depend on runuser: %s", script)
	}
	if !strings.Contains(script, "while :; do sleep 3600; done") {
		t.Fatalf("bootstrap script missing keepalive loop: %s", script)
	}
}

func TestProviderPoolClaudePromptDockerArgsDoNotKeepStdinOpen(t *testing.T) {
	args := providerPoolClaudePromptDockerArgs("worker", "sonnet", "hello")
	if len(args) == 0 {
		t.Fatal("expected docker args")
	}
	if args[0] != "exec" {
		t.Fatalf("first arg = %q, want exec", args[0])
	}
	for _, arg := range args {
		if arg == "-i" {
			t.Fatalf("claude prompt args should not keep stdin open: %v", args)
		}
	}
	if !reflect.DeepEqual(args[:8], []string{"exec", "-u", "node", "-e", "HOME=/home/node", "-w", "/work", "worker"}) {
		t.Fatalf("unexpected docker exec prefix: %v", args)
	}
	if !containsAll(strings.Join(args, "\x00"), "claude", "--dangerously-skip-permissions", "--model", "sonnet", "-p", "hello") {
		t.Fatalf("unexpected claude args: %v", args)
	}
}

func TestProviderPoolClaudeStreamWorkerModeCanBeDisabledExplicitly(t *testing.T) {
	t.Setenv("DORKPIPE_PROVIDER_POOL_CLAUDE_STREAM_WORKER", "")
	if !providerPoolClaudeStreamWorkerEnabled() {
		t.Fatal("stream worker should be enabled by default")
	}
	if got := providerPoolClaudeWorkerMode(); got != "stream_worker" {
		t.Fatalf("mode = %q, want stream_worker", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_CLAUDE_STREAM_WORKER", "single_prompt")
	if providerPoolClaudeStreamWorkerEnabled() {
		t.Fatal("stream worker should be disabled by explicit single_prompt mode")
	}
	if got := providerPoolClaudeWorkerMode(); got != "single_prompt" {
		t.Fatalf("mode = %q, want single_prompt", got)
	}
}

func TestProviderPoolClaudeStreamDaemonArgsUseGenericWorkerBoundary(t *testing.T) {
	args := providerPoolClaudeStreamDaemonDockerArgs("worker", "/tmp/dorkpipe-provider-pool/claude.sock", "sonnet")
	if len(args) == 0 {
		t.Fatal("expected docker args")
	}
	if !reflect.DeepEqual(args[:8], []string{"exec", "-d", "-u", "node", "-e", "HOME=/home/node", "-w", "/work"}) {
		t.Fatalf("unexpected daemon docker prefix: %v", args)
	}
	joined := strings.Join(args, "\x00")
	if !containsAll(joined,
		"worker",
		"node",
		"--input-format', 'stream-json'",
		"--output-format', 'stream-json'",
		"--include-partial-messages",
		"--replay-user-messages",
		"--verbose",
		"/tmp/dorkpipe-provider-pool/claude.sock",
		"sonnet",
	) {
		t.Fatalf("unexpected daemon args: %v", args)
	}
}

func TestProviderPoolClaudeStreamClientArgsUseUnixSocket(t *testing.T) {
	args := providerPoolClaudeStreamClientDockerArgs("worker", "/tmp/dorkpipe-provider-pool/claude.sock", "hello", "turn-1")
	if !reflect.DeepEqual(args[:7], []string{"exec", "-u", "node", "-e", "HOME=/home/node", "-w", "/work"}) {
		t.Fatalf("unexpected client docker prefix: %v", args)
	}
	joined := strings.Join(args, "\x00")
	if !containsAll(joined, "worker", "node", "createConnection", "/tmp/dorkpipe-provider-pool/claude.sock", "hello", "turn-1") {
		t.Fatalf("unexpected client args: %v", args)
	}
}

func TestMergePromptTimingsPreservesProviderTiming(t *testing.T) {
	dst := map[string]int64{"status_ms": 7}
	mergePromptTimings(dst, map[string]any{
		"claude_command_ms": float64(25),
	})
	if dst["status_ms"] != 7 {
		t.Fatalf("status timing changed: %v", dst)
	}
	if dst["claude_command_ms"] != 25 {
		t.Fatalf("claude command timing missing: %v", dst)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
