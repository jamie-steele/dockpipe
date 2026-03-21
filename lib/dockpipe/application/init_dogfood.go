package application

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/lib/dockpipe/infrastructure"
)

// Bundled dogfood workflow names (Codex presets: source under dockpipe/workflows/<name>/ in this repo;
// other dogfood names use templates/<name>/; materialized bundle: dockpipe/workflows/<name>/).
const (
	dogfoodWorkflowTest          = "test"
	dogfoodWorkflowCodexPAV      = "dogfood-codex-pav"
	dogfoodWorkflowCodexSecurity = "dogfood-codex-security"
)

// dogfoodInstallOpts selects optional workflows to copy from the bundled tree into the project (opt-in only).
type dogfoodInstallOpts struct {
	Test          bool
	CodexPAV      bool
	CodexSecurity bool
}

func (o dogfoodInstallOpts) any() bool {
	return o.Test || o.CodexPAV || o.CodexSecurity
}

// installDogfoodWorkflows copies selected bundled workflows into projectDir/dockpipe/workflows/<name>/.
// Skips a name if that directory already exists. No-op when all flags are false.
func installDogfoodWorkflows(repoRoot, projectDir string, o dogfoodInstallOpts) error {
	if !o.any() {
		return nil
	}
	if o.Test {
		if err := copyBundledWorkflowIntoProject(repoRoot, projectDir, dogfoodWorkflowTest); err != nil {
			return err
		}
	}
	if o.CodexPAV {
		if err := copyBundledWorkflowIntoProject(repoRoot, projectDir, dogfoodWorkflowCodexPAV); err != nil {
			return err
		}
	}
	if o.CodexSecurity {
		if err := copyBundledWorkflowIntoProject(repoRoot, projectDir, dogfoodWorkflowCodexSecurity); err != nil {
			return err
		}
	}
	return nil
}

func bundledDogfoodWorkflowSourceDir(repoRoot, workflowName string) string {
	switch workflowName {
	case dogfoodWorkflowCodexPAV, dogfoodWorkflowCodexSecurity:
		return filepath.Join(repoRoot, infrastructure.BundledDockpipeDir, "workflows", workflowName)
	default:
		return filepath.Join(infrastructure.WorkflowsRootDir(repoRoot), workflowName)
	}
}

func copyBundledWorkflowIntoProject(repoRoot, projectDir, workflowName string) error {
	src := bundledDogfoodWorkflowSourceDir(repoRoot, workflowName)
	st, err := os.Stat(src)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("dogfood: bundled workflow %q not found under install tree", workflowName)
	}
	dest := filepath.Join(projectDir, "dockpipe", "workflows", workflowName)
	if _, err := os.Stat(dest); err == nil {
		fmt.Fprintf(os.Stderr, "[dockpipe] dogfood: skip %q (dockpipe/workflows/%s already exists)\n", workflowName, workflowName)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := copyDir(src, dest); err != nil {
		return fmt.Errorf("dogfood: copy %s: %w", workflowName, err)
	}
	_ = filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".sh") {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	fmt.Printf("Installed dogfood workflow dockpipe/workflows/%s/\n", workflowName)
	return nil
}
