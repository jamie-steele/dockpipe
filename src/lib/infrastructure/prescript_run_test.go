package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunHostScriptRunsSubprocessWithInheritedStdio(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "x.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\necho OUT\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunHostScript(script, os.Environ()); err != nil {
		t.Fatalf("RunHostScript: %v", err)
	}
}
