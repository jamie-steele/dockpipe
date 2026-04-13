package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"

	"gopkg.in/yaml.v3"
)

const dockpipeProjectReadme = `# Dockpipe project

- **workflows/** — Default home for named workflows (**config.yml** per folder); **dockpipe init &lt;name&gt;** creates **workflows/&lt;name&gt;/** (override with **--workflows-dir** or **DOCKPIPE_WORKFLOWS_DIR**).
- **Compiled packages** — Workflows, resolvers, and core slices are expected to come from **compile** / **install** flows now rather than a copied **templates/core/** scaffold.
- **templates/&lt;name&gt;/** — Legacy named workflows; still resolved if **workflows/** does not define the same name.
- **.env.vault.template.example** — Example **op://** mapping for **op inject** (install the **op** CLI from 1Password). Copy to **.env.vault.template** and see **docs/vault.md** for **secrets.vault** and workflow **vault:**. For a vendor-neutral path, use **secretstore** + **dotenv** and **.env.secretstore**.
- **dockpipe.config.json** (optional) — Repo-root JSON: **compile** source lists and optional **secrets** (**vault_template** preferred; **op_inject_template** is legacy). Omit to use built-in compile defaults when you add a config file later.
`

// ensureProjectScaffold creates the minimal project scaffold and root metadata files.
// Shared workflows/resolvers/runtime material now comes from compile/install flows instead of copying templates/core.
func ensureProjectScaffold(repoRoot, projectDir string) error {
	_ = os.MkdirAll(filepath.Join(projectDir, "workflows"), 0o755)
	readme := filepath.Join(projectDir, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		_ = os.WriteFile(readme, []byte(dockpipeProjectReadme), 0o644)
	}
	dc := filepath.Join(projectDir, domain.DockpipeProjectConfigFileName)
	if _, err := os.Stat(dc); os.IsNotExist(err) {
		b, err := domain.DefaultDockpipeProjectConfigBytes()
		if err != nil {
			return fmt.Errorf("%s: %w", domain.DockpipeProjectConfigFileName, err)
		}
		if err := os.WriteFile(dc, append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	if err := infrastructure.WriteVaultTemplateExampleIfMissing(projectDir); err != nil {
		return fmt.Errorf("vault template example: %w", err)
	}
	return nil
}

// bundledWorkflowSourceDir resolves a bundled workflow by name for init --from / template init.
// Prefers templates/<name> when present (fixtures, materialized bundles), else WorkflowsRootDir/<name>.
func bundledWorkflowSourceDir(repoRoot, name string) string {
	legacy := filepath.Join(repoRoot, "templates", name)
	if st, err := os.Stat(legacy); err == nil && st.IsDir() {
		return legacy
	}
	return filepath.Join(infrastructure.WorkflowsRootDir(repoRoot), name)
}

func resolveInitFromSource(repoRoot, from string) (srcDir string, isBlank bool, err error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return "", false, fmt.Errorf("empty --from")
	}
	if from == "blank" {
		return "", true, nil
	}
	if strings.Contains(from, "://") {
		return "", false, fmt.Errorf("--from must be a template source (blank, bundled name, or local directory path), not a URL")
	}
	bundled := bundledWorkflowSourceDir(repoRoot, from)
	if st, e := os.Stat(bundled); e == nil && st.IsDir() {
		return bundled, false, nil
	}
	if p := infrastructure.FindNestedWorkflowDirByLeafName(repoRoot, from); p != "" {
		return p, false, nil
	}
	if p := infrastructure.FindBundledWorkflowAuthoringDirByLeafName(repoRoot, from); p != "" {
		return p, false, nil
	}
	abs, e := filepath.Abs(from)
	if e != nil {
		return "", false, e
	}
	if st, e := os.Stat(abs); e == nil && st.IsDir() {
		return abs, false, nil
	}
	return "", false, fmt.Errorf("unknown --from source %q — use blank, a bundled workflow name (e.g. init, run, run-apply, run-apply-validate, secretstore), workflows/<name>, src/core/workflows/**/<name>, a nested directory under compile.workflows roots, or another filesystem path to a workflow directory", from)
}

func writeWorkflowYAML(path string, wf *domain.Workflow) error {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func patchWorkflowConfigName(cfgPath, name string) error {
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	wf, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	wf.Name = name
	return writeWorkflowYAML(cfgPath, wf)
}

func applyInitWorkflowFlags(cfgPath, resolver, runtime, strategy string) error {
	resolver = strings.TrimSpace(resolver)
	runtime = strings.TrimSpace(runtime)
	strategy = strings.TrimSpace(strategy)
	if resolver == "" && runtime == "" && strategy == "" {
		return nil
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	wf, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	if resolver != "" {
		wf.DefaultResolver = resolver
		wf.Resolver = resolver
	}
	if runtime != "" {
		wf.DefaultRuntime = runtime
		wf.Runtime = runtime
	}
	if strategy != "" {
		wf.Strategy = strategy
		if wf.Strategies == nil {
			wf.Strategies = []string{strategy}
		} else if !stringSliceContains(wf.Strategies, strategy) {
			wf.Strategies = append(wf.Strategies, strategy)
		}
	}
	return writeWorkflowYAML(cfgPath, wf)
}

func stringSliceContains(ss []string, s string) bool {
	for _, x := range ss {
		if strings.TrimSpace(x) == s {
			return true
		}
	}
	return false
}

func maybeWarnUnusedWorkflowsRoot(projectDir string) {
	if infrastructure.UsesBundledAssetLayout(projectDir) || infrastructure.DockpipeAuthoringSourceTree(projectDir) {
		return
	}
	root := infrastructure.WorkflowsRootDir(projectDir)
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return
	}
	if infrastructure.WorkflowsDirHasDockpipeWorkflow(root) {
		return
	}
	if !infrastructure.WorkflowsDirHasFiles(root) {
		return
	}
	rel := root
	if r, err := filepath.Rel(projectDir, root); err == nil && !strings.HasPrefix(r, "..") {
		rel = r
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] warning: %q exists but has no DockPipe workflow folders (no <name>/config.yml). "+
		"This path is often used for GitHub Actions or other tools. New workflows will still be created here. "+
		"To use a different directory set DOCKPIPE_WORKFLOWS_DIR or run: dockpipe init <name> --workflows-dir <path>\n", rel)
}

func ensureDefaultStarterWorkflow(repoRoot, projectDir string) error {
	wfRoot := infrastructure.WorkflowsRootDir(projectDir)
	if infrastructure.WorkflowsDirHasDockpipeWorkflow(wfRoot) {
		return nil
	}
	return createNamedWorkflow(repoRoot, projectDir, "example", "init", "", "", "")
}

func createNamedWorkflow(repoRoot, projectDir, name, fromSource, resolver, runtime, strategy string) error {
	maybeWarnUnusedWorkflowsRoot(projectDir)
	wfBase := infrastructure.WorkflowsRootDir(projectDir)
	td := filepath.Join(wfBase, name)
	if st, err := os.Stat(td); err == nil && st.IsDir() {
		return fmt.Errorf("workflow directory already exists: %s", td)
	}
	if !infrastructure.DockpipeAuthoringSourceTree(projectDir) && !infrastructure.UsesBundledAssetLayout(projectDir) {
		leg := filepath.Join(projectDir, "templates", name)
		if st, err := os.Stat(leg); err == nil && st.IsDir() {
			return fmt.Errorf("workflow %q already exists at %s (legacy templates/ layout); remove or rename before creating under workflows/", name, leg)
		}
	}
	if err := os.MkdirAll(td, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(td, "config.yml")

	srcDir, isBlank, err := resolveInitFromSource(repoRoot, fromSource)
	if err != nil {
		return err
	}
	if isBlank {
		wf := &domain.Workflow{
			Name:        name,
			Description: "Dockpipe workflow — edit me.",
		}
		if err := writeWorkflowYAML(cfgPath, wf); err != nil {
			return err
		}
	} else {
		if err := copyDir(srcDir, td); err != nil {
			return err
		}
		if err := patchWorkflowConfigName(cfgPath, name); err != nil {
			return err
		}
	}
	if err := applyInitWorkflowFlags(cfgPath, resolver, runtime, strategy); err != nil {
		return err
	}
	showRel := td
	if r, err := filepath.Rel(projectDir, td); err == nil && !strings.HasPrefix(r, "..") {
		showRel = r
	}
	if isBlank {
		fmt.Fprintf(os.Stderr, "[dockpipe] Created %s/ (empty workflow — edit config.yml; use --from to copy a bundled template)\n", showRel)
	} else {
		fmt.Fprintf(os.Stderr, "[dockpipe] Created %s/ (from %s)\n", showRel, fromSource)
	}
	return nil
}
