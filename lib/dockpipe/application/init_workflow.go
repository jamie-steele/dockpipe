package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"

	"gopkg.in/yaml.v3"
)

const dockpipeProjectReadme = `# Dockpipe project

- **scripts/** — Run and act scripts.
- **images/** — Optional project Dockerfiles (e.g. **images/example/** copied from **templates/core/assets/images/example/**).
- **templates/** — Bundled-style workflows (**config.yml**), one folder per name (from upstream / **dockpipe init &lt;name&gt;**).
- **templates/core/** — Shared **runtimes/**, **resolvers/**, **strategies/**, **assets/** (**scripts/**, **images/**, **compose/**) (from **dockpipe init**).
- **dockpipe/workflows/** — Optional **repo-local** workflows (e.g. **dockpipe init --dogfood-***); **--workflow** checks here before **templates/**.
- **dockpipe.yml** (optional) — Repo-root workflow; use **dockpipe --workflow-file dockpipe.yml**.
`

const dockpipeDirReadme = `# dockpipe/

Optional **repo-local** workflows live under **workflows/** (one directory per workflow with **config.yml**). Resolution checks here before **templates/**.

To copy the bundled dogfood workflows from a dockpipe **source checkout** (set **DOCKPIPE_REPO_ROOT**; Codex presets ship under **dockpipe/workflows/** in-tree):

    export DOCKPIPE_REPO_ROOT="$(pwd)"
    make build
    ./bin/dockpipe init --dogfood-test --dogfood-codex-pav --dogfood-codex-security

Skips **workflows/<name>/** if it already exists. See **docs/cli-reference.md** (**dockpipe init**).

`

// ensureProjectScaffold creates scripts/, images/, templates/, merges bundled templates/core,
// and adds README.md / dockpipe.yml when missing. Idempotent for an existing repo tree.
func ensureProjectScaffold(repoRoot, projectDir string) error {
	_ = os.MkdirAll(filepath.Join(projectDir, "scripts"), 0o755)
	_ = os.MkdirAll(filepath.Join(projectDir, "images"), 0o755)
	_ = os.MkdirAll(filepath.Join(projectDir, "templates"), 0o755)
	if err := mergeBundledTemplatesCore(repoRoot, projectDir); err != nil {
		return fmt.Errorf("templates/core: %w", err)
	}
	_ = os.MkdirAll(filepath.Join(projectDir, "dockpipe", "workflows"), 0o755)
	dpReadme := filepath.Join(projectDir, "dockpipe", "README.md")
	if _, err := os.Stat(dpReadme); os.IsNotExist(err) {
		_ = os.WriteFile(dpReadme, []byte(dockpipeDirReadme), 0o644)
	}
	readme := filepath.Join(projectDir, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		_ = os.WriteFile(readme, []byte(dockpipeProjectReadme), 0o644)
	}
	dock := filepath.Join(projectDir, "dockpipe.yml")
	if _, err := os.Stat(dock); os.IsNotExist(err) {
		_ = os.WriteFile(dock, []byte(dockpipeYmlBoilerplate), 0o644)
	}
	return nil
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
	bundled := filepath.Join(infrastructure.WorkflowsRootDir(repoRoot), from)
	if st, e := os.Stat(bundled); e == nil && st.IsDir() {
		return bundled, false, nil
	}
	// Codex dogfood presets live under dockpipe/workflows/ in this repo (not templates/).
	alt := filepath.Join(repoRoot, infrastructure.BundledDockpipeDir, "workflows", from)
	if st, e := os.Stat(alt); e == nil && st.IsDir() {
		return alt, false, nil
	}
	abs, e := filepath.Abs(from)
	if e != nil {
		return "", false, e
	}
	if st, e := os.Stat(abs); e == nil && st.IsDir() {
		return abs, false, nil
	}
	return "", false, fmt.Errorf("unknown --from source %q — use blank, a bundled name (e.g. init, run, test, run-apply-validate, dogfood-codex-pav, dogfood-codex-security), or a path to an existing workflow directory", from)
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

func createNamedWorkflow(repoRoot, projectDir, name, fromSource, resolver, runtime, strategy string) error {
	td := filepath.Join(projectDir, "templates", name)
	if st, err := os.Stat(td); err == nil && st.IsDir() {
		return fmt.Errorf("templates/%s already exists", name)
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
	_ = copyFileMaybe(filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "scripts", "example-run.sh"), filepath.Join(projectDir, "scripts/example-run.sh"))
	_ = copyFileMaybe(filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "scripts", "example-act.sh"), filepath.Join(projectDir, "scripts/example-act.sh"))
	_ = copyDirMaybe(filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "images", "example"), filepath.Join(projectDir, "images/example"))
	fmt.Printf("Created templates/%s/ (from %s)\n", name, fromSource)
	return nil
}
