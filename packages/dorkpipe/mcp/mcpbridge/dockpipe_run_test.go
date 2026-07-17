package mcpbridge

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDockpipeRunInputCommandArgsPreservesOptionalPackage(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_RESTRICT_WORKDIR", "0")
	workdir := t.TempDir()
	got, err := (dockpipeRunInput{Workflow: "devcontainer", Package: "ide", Workdir: workdir, Argv: []string{"discover", "--workspace", "."}}).commandArgs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--workflow", "devcontainer", "--package", "ide", "--workdir", filepath.Clean(workdir), "--", "discover", "--workspace", "."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestDockpipeRunInputRejectsUnknownResultMode(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_RESTRICT_WORKDIR", "0")
	_, err := (dockpipeRunInput{Workflow: "example", ResultMode: "events"}).commandArgs()
	if err == nil {
		t.Fatal("expected result_mode validation error")
	}
}

func TestDockpipeRunInputStdoutModePreservesPackageEventStream(t *testing.T) {
	stream := "{\"contract_version\":\"devcontainer.lifecycle.v1\"}\n"
	got, isError, err := (dockpipeRunInput{ResultMode: "stdout"}).formatResult(stream, "ignored stderr", 0)
	if err != nil || isError || string(got) != stream {
		t.Fatalf("result = %q error=%v err=%v", got, isError, err)
	}
}
