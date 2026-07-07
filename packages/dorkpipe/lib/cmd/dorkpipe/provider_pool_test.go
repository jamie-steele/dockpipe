package main

import (
	"reflect"
	"testing"
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
