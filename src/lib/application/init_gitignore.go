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
.gocache/
.gomodcache/
tmp/
.tmp/
` + dockpipeGitignoreEnd + "\n"

// appendDockpipeGitignore writes a marked block to .gitignore at the git repository root
// (from GitTopLevel(projectDir)). If the block is already present, it prints and returns nil.
func appendDockpipeGitignore(projectDir string) error {
	top, err := infrastructure.GitTopLevel(projectDir)
	if err != nil {
		return fmt.Errorf("dockpipe init --gitignore requires a git working tree: %w", err)
	}
	path := filepath.Join(top, ".gitignore")
	return infrastructure.RunOperationWithOptions(
		os.Stderr,
		"init.gitignore",
		"Updating .gitignore…",
		map[string]string{"path": path},
		infrastructure.OperationOptions{Spinner: false},
		func() error {
			data, err := os.ReadFile(path)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if os.IsNotExist(err) {
				data = nil
			}
			if bytes.Contains(data, []byte(dockpipeGitignoreBegin)) {
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
			return os.WriteFile(path, buf.Bytes(), 0o644)
		},
	)
}
