package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

// EffectiveStrategyName returns CLI --strategy if set, else workflow.strategy.
func EffectiveStrategyName(opts *CliOpts, wf *domain.Workflow) string {
	if opts != nil && strings.TrimSpace(opts.Strategy) != "" {
		return strings.TrimSpace(opts.Strategy)
	}
	if wf != nil {
		return strings.TrimSpace(wf.Strategy)
	}
	return ""
}

// ValidateStrategyAllowlist errors if strategies: is non-empty and name is not listed.
func ValidateStrategyAllowlist(wf *domain.Workflow, name string) error {
	if wf == nil || len(wf.Strategies) == 0 || name == "" {
		return nil
	}
	for _, s := range wf.Strategies {
		if strings.TrimSpace(s) == name {
			return nil
		}
	}
	return fmt.Errorf("strategy %q is not allowed by this workflow (strategies: %v)", name, wf.Strategies)
}

// ResolveStrategyFilePath returns the path to the strategy KEY=value file:
// per-workflow strategies/<name>, then templates/core/strategies/<name>, then legacy templates/strategies/<name>.
func ResolveStrategyFilePath(repoRoot, wfRoot, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("strategy name is empty")
	}
	candidates := []string{}
	if wfRoot != "" {
		candidates = append(candidates, filepath.Join(wfRoot, "strategies", name))
	}
	candidates = append(candidates, filepath.Join(repoRoot, "templates", "core", "strategies", name))
	candidates = append(candidates, filepath.Join(repoRoot, "templates", "strategies", name))
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("strategy file not found for %q (tried %v); shared strategies live under templates/core/strategies/ — upgrade dockpipe or refresh the bundled cache if your install is stale", name, candidates)
}

// LoadStrategyAssignments loads and parses a named strategy file.
func LoadStrategyAssignments(repoRoot, wfRoot, name string) (domain.StrategyAssignments, string, error) {
	path, err := ResolveStrategyFilePath(repoRoot, wfRoot, name)
	if err != nil {
		return domain.StrategyAssignments{}, "", err
	}
	m, err := infrastructure.LoadStrategyFile(path)
	if err != nil {
		return domain.StrategyAssignments{}, path, err
	}
	return domain.FromStrategyMap(m), path, nil
}

// ResolveStrategyScriptPaths turns repo-relative strategy paths into absolute paths like workflow run:/act:.
func ResolveStrategyScriptPaths(rels []string, wfRoot, repoRoot string) []string {
	out := make([]string, 0, len(rels))
	for _, rel := range rels {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		out = append(out, infrastructure.ResolveWorkflowScript(rel, wfRoot, repoRoot))
	}
	return out
}

// StrategyAfterHandlesBundledCommit reports whether any after script is the bundled commit-worktree.sh.
func StrategyAfterHandlesBundledCommit(afterAbs []string, repoRoot string) bool {
	for _, p := range afterAbs {
		if infrastructure.IsBundledCommitWorktree(p, repoRoot) {
			return true
		}
	}
	return false
}

// ValidateNoDuplicateClone errors when workflow.run lists clone-worktree.sh while the strategy already provides clone.
func ValidateNoDuplicateClone(wf *domain.Workflow, wfRoot, repoRoot string, strategyProvidesClone bool, stratBeforeAbs []string) error {
	if wf == nil {
		return nil
	}
	var stratHasClone bool
	for _, p := range stratBeforeAbs {
		if strings.HasSuffix(p, "clone-worktree.sh") {
			stratHasClone = true
			break
		}
	}
	if !strategyProvidesClone && !stratHasClone {
		return nil
	}
	for _, r := range wf.Run {
		ap := infrastructure.ResolveWorkflowScript(r, wfRoot, repoRoot)
		if strings.HasSuffix(ap, "clone-worktree.sh") {
			return fmt.Errorf("workflow run lists clone-worktree.sh but strategy already runs clone; remove clone from workflow run: (hint: rely on strategy git-worktree or remove duplicate from run:)")
		}
	}
	return nil
}

// ActWouldBeBundledCommit resolves a workflow/resolver act path and reports bundled commit-worktree.sh.
func ActWouldBeBundledCommit(actRel string, wfRoot, repoRoot string) bool {
	actRel = strings.TrimSpace(actRel)
	if actRel == "" {
		return false
	}
	ap := infrastructure.ResolveWorkflowScript(actRel, wfRoot, repoRoot)
	return infrastructure.IsBundledCommitWorktree(ap, repoRoot)
}

// ResolverActWouldBeBundledCommit resolves resolver-relative act path.
func ResolverActWouldBeBundledCommit(actFromResolver, repoRoot string) bool {
	actFromResolver = strings.TrimSpace(actFromResolver)
	if actFromResolver == "" {
		return false
	}
	p := filepath.Join(repoRoot, actFromResolver)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return false
	}
	return infrastructure.IsBundledCommitWorktree(p, repoRoot)
}

// RunStrategyAfterScripts runs after hooks: bundled commit uses CommitOnHost; other paths use RunHostScript.
func RunStrategyAfterScripts(afterAbs []string, repoRoot string, envMap map[string]string, envSlice []string, opts *CliOpts) error {
	workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
	for _, p := range afterAbs {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("strategy after script not found: %s: %w", p, err)
		}
		if infrastructure.IsBundledCommitWorktree(p, repoRoot) {
			if err := infrastructure.CommitOnHost(workHost, envMap["DOCKPIPE_COMMIT_MESSAGE"], firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], opts.BundleOut), strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1"); err != nil {
				return err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] Strategy after: %s\n", p)
		if err := runHostScriptAppFn(p, envSlice); err != nil {
			return err
		}
	}
	return nil
}
