package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// runEmbeddedResolverWorkflow runs bundled delegate YAML for DOCKPIPE_*_WORKFLOW (multi-step) with merged env.
// Used when DOCKPIPE_RESOLVER_WORKFLOW is set so resolvers delegate (e.g. claude, codex, code-server, cursor-dev, vscode under templates/core/resolvers/<name>/config.yml).
// runStepsFn must be the steps runner (e.g. runStepsAppFn) — passed in to avoid an init cycle with run_steps.go.
func runEmbeddedResolverWorkflow(
	workflowName string,
	repoRoot string,
	envMap map[string]string,
	opts *CliOpts,
	cliArgs []string,
	locked map[string]bool,
	dataVol, dataDir string,
	resolverName string,
	templateName string,
	runStepsFn func(runStepsOpts) error,
) error {
	return runEmbeddedResolverWorkflowWithLoad(loadWorkflowAppFn, runStepsFn, workflowName, repoRoot, envMap, opts, cliArgs, locked, dataVol, dataDir, resolverName, templateName)
}

func runEmbeddedResolverWorkflowWithLoad(
	loadWF func(string) (*domain.Workflow, error),
	runStepsFn func(runStepsOpts) error,
	workflowName string,
	repoRoot string,
	envMap map[string]string,
	opts *CliOpts,
	cliArgs []string,
	locked map[string]bool,
	dataVol, dataDir string,
	resolverName string,
	templateName string,
) error {
	name := strings.TrimSpace(workflowName)
	if name == "" {
		return fmt.Errorf("resolver workflow name is empty")
	}
	wd := ""
	if opts != nil && strings.TrimSpace(opts.Workdir) != "" {
		wd = opts.Workdir
	} else {
		wd, _ = os.Getwd()
	}
	projectRoot := wd
	if ap, err := filepath.Abs(wd); err == nil {
		projectRoot = ap
	}
	wfPath, err := infrastructure.ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir(repoRoot, wd, name)
	if err != nil {
		return fmt.Errorf("resolver workflow %q: %w", name, err)
	}
	subWf, err := loadWF(wfPath)
	if err != nil {
		return fmt.Errorf("parse resolver workflow %q: %w", name, err)
	}
	if len(subWf.Steps) == 0 {
		return fmt.Errorf("resolver workflow %q has no steps", name)
	}
	wfRoot := filepath.Dir(wfPath)
	if err := buildWorkflowEnvInto(envMap, subWf, wfRoot, repoRoot, opts); err != nil {
		return err
	}
	envSlice := domain.EnvMapToSlice(envMap)
	if subWf.NeedsDockerReachable() {
		if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Resolver workflow: %s\n", name)
	return runStepsFn(runStepsOpts{
		wf:             subWf,
		wfRoot:         wfRoot,
		repoRoot:       repoRoot,
		projectRoot:    projectRoot,
		cliArgs:        cliArgs,
		envMap:         envMap,
		envSlice:       envSlice,
		locked:         locked,
		userIsolate:    "",
		userAct:        "",
		firstStepExtra: nil,
		opts:           opts,
		dataVol:        dataVol,
		dataDir:        dataDir,
		resolver:       resolverName,
		templateName:   templateName,
	})
}
