package application

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"dockpipe/src/lib/infrastructure"
)

const (
	dockpipeGitignoreBegin = "# BEGIN dockpipe-gitignore"
	dockpipeGitignoreEnd   = "# END dockpipe-gitignore"
)

const dockpipeGitignoreBlock = dockpipeGitignoreBegin + `
# Local tooling and cache (see dockpipe docs). Safe to re-run; idempotent.
# Compile output and host-run state live under bin/.dockpipe/ (ignored if you already ignore bin/).
bin/.dockpipe/
.dorkpipe/
.gocache/
.gomodcache/
tmp/
` + dockpipeGitignoreEnd + "\n"

// appendDockpipeGitignore writes a marked block to .gitignore at the git repository root
// (from GitTopLevel(projectDir)). If the block is already present, it prints and returns nil.
func appendDockpipeGitignore(projectDir string) error {
	top, err := infrastructure.GitTopLevel(projectDir)
	if err != nil {
		return fmt.Errorf("dockpipe init --gitignore requires a git working tree: %w", err)
	}
	path := filepath.Join(top, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		data = nil
	}
	if bytes.Contains(data, []byte(dockpipeGitignoreBegin)) {
		fmt.Fprintf(os.Stderr, "[dockpipe] %s already contains the dockpipe-gitignore block; skipping\n", path)
		return nil
	}
	var buf bytes.Buffer
	if len(data) > 0 {
		buf.Write(data)
		if data[len(data)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	buf.WriteString(dockpipeGitignoreBlock)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Appended dockpipe entries to %s\n", path)
	return nil
}
