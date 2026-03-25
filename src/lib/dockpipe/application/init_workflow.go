package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"

	"gopkg.in/yaml.v3"
)

const dockpipeProjectReadme = `# Dockpipe project

- **scripts/** — Run and act scripts.
- **images/** — Optional project Dockerfiles (e.g. **images/example/** copied from **templates/core/assets/images/example/**). Bundled framework images resolve via **DockerfileDir** (**resolvers/** / **bundles/** / **assets/images/**).
- **workflows/** — Default home for named workflows (**config.yml** per folder); **dockpipe init &lt;name&gt;** creates **workflows/&lt;name&gt;/** (override with **--workflows-dir** or **DOCKPIPE_WORKFLOWS_DIR**).
- **templates/core/** — Shared **runtimes/**, **resolvers/**, **strategies/**, **assets/** (**agnostic scripts**, **images/**, **compose/**), **bundles/** (domain asset packs) (from **dockpipe init**).
- **templates/&lt;name&gt;/** — Legacy named workflows; still resolved if **workflows/** does not define the same name.
- **dockpipe.config.json** (optional) — Repo-root JSON: **compile** source lists and optional **secrets.op_inject_template** (path to e.g. **.env.op.template**); omit to use built-in compile defaults.
`

// agentsSelfAnalysisMarker is embedded once in AGENTS.md so re-init does not duplicate the section.
const agentsSelfAnalysisMarker = "<!-- dockpipe: self-analysis handoff -->"

func agentsSelfAnalysisSection() string {
	return agentsSelfAnalysisMarker + "\n\n## Self-analysis handoff\n\n" +
		"Generated outputs: **`.dockpipe/paste-this-prompt.txt`**, **`.dockpipe/orchestrator-cursor-prompt.md`**, **`.dorkpipe/self-analysis/`**, and optionally **`.dorkpipe/run.json`**.\n\n" +
		"### Agent workflow (read before repo-wide work)\n\n" +
		"1. **Discover:** Read **AGENTS.md** and load these paths if present.\n" +
		"2. **Freshness:** Compare to **git HEAD**, file dates, **VERSION**, or **`.dorkpipe/run.json`**.\n" +
		"3. **Use** current analysis as primary context; if **stale**, tell the user refresh is recommended before big changes; **do not** auto-refresh.\n" +
		"4. **Refresh** only when the user asks; then run **`make self-analysis`**, **`make self-analysis-host`**, or **`make self-analysis-stack`** (or **`dockpipe --workflow dorkpipe-self-analysis --workdir . --`**).\n" +
		"5. **Isolation:** Analysis runs in a **Docker** isolate (see workflow **isolate:** image). **`dorkpipe-self-analysis-stack`** uses **docker compose** on the host for Postgres/Ollama, then the same isolate step.\n\n" +
		"**Cursor:** Copy **`.cursor/rules/dockpipe-agents.mdc`** from the dockpipe source tree for always-on IDE rules aligned with **AGENTS.md**.\n"
}

// isSelfAnalysisWorkflowSource reports whether --from copied the repo self-analysis workflow
// (dorkpipe-self-analysis, -host, -stack, or a path ending with that name prefix).
func isSelfAnalysisWorkflowSource(srcDir, fromSource string) bool {
	base := filepath.Base(filepath.Clean(srcDir))
	if strings.HasPrefix(base, "dorkpipe-self-analysis") {
		return true
	}
	fs := strings.TrimSpace(fromSource)
	return strings.HasPrefix(fs, "dorkpipe-self-analysis")
}

// ensureAgentsSelfAnalysisPointer appends a stable handoff section to AGENTS.md when missing.
// Returns whether the file was written or updated.
func ensureAgentsSelfAnalysisPointer(projectDir string) (bool, error) {
	path := filepath.Join(projectDir, "AGENTS.md")
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	existing := string(b)
	if strings.Contains(existing, agentsSelfAnalysisMarker) {
		return false, nil
	}
	body := agentsSelfAnalysisSection() + "\n"
	if existing == "" {
		data := "# AGENTS.md\n\n" + body
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			return false, err
		}
		return true, nil
	}
	sep := "\n"
	if !strings.HasSuffix(existing, "\n") {
		sep = "\n\n"
	}
	if err := os.WriteFile(path, []byte(existing+sep+body), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// ensureProjectScaffold creates scripts/, images/, templates/, merges bundled templates/core,
// and adds README.md / dockpipe.config.json when missing. Idempotent for an existing repo tree.
func ensureProjectScaffold(repoRoot, projectDir string) error {
	_ = os.MkdirAll(filepath.Join(projectDir, "scripts"), 0o755)
	_ = os.MkdirAll(filepath.Join(projectDir, "images"), 0o755)
	_ = os.MkdirAll(filepath.Join(projectDir, "templates"), 0o755)
	if err := mergeBundledTemplatesCore(repoRoot, projectDir); err != nil {
		return fmt.Errorf("templates/core: %w", err)
	}
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
	stagingWf := filepath.Join(repoRoot, ".staging", "workflows", from)
	if st, e := os.Stat(stagingWf); e == nil && st.IsDir() {
		return stagingWf, false, nil
	}
	abs, e := filepath.Abs(from)
	if e != nil {
		return "", false, e
	}
	if st, e := os.Stat(abs); e == nil && st.IsDir() {
		return abs, false, nil
	}
	return "", false, fmt.Errorf("unknown --from source %q — use blank, a bundled workflow name (e.g. init, run, run-apply, run-apply-validate, secretstore), workflows/<name> or .staging/workflows/<name> in a dockpipe checkout, or another filesystem path to a workflow directory", from)
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
	rel := root
	if r, err := filepath.Rel(projectDir, root); err == nil && !strings.HasPrefix(r, "..") {
		rel = r
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] warning: %q exists but has no DockPipe workflow folders (no <name>/config.yml). "+
		"This path is often used for GitHub Actions or other tools. New workflows will still be created here. "+
		"To use a different directory set DOCKPIPE_WORKFLOWS_DIR or run: dockpipe init <name> --workflows-dir <path>\n", rel)
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
	if !isBlank {
		_ = copyFileMaybe(filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "scripts", "example-run.sh"), filepath.Join(projectDir, "scripts/example-run.sh"))
		_ = copyFileMaybe(filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "scripts", "example-act.sh"), filepath.Join(projectDir, "scripts/example-act.sh"))
		_ = copyDirMaybe(filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "images", "example"), filepath.Join(projectDir, "images/example"))
	}
	if !isBlank && isSelfAnalysisWorkflowSource(srcDir, fromSource) {
		changed, err := ensureAgentsSelfAnalysisPointer(projectDir)
		if err != nil {
			return fmt.Errorf("AGENTS.md: %w", err)
		}
		if changed {
			fmt.Fprintf(os.Stderr, "[dockpipe] Appended self-analysis handoff to AGENTS.md\n")
		}
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
