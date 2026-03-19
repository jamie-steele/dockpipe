package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

func buildWorkflowEnvInto(env map[string]string, wf *domain.Workflow, wfRoot, repoRoot string, opts *CliOpts) {
	if wf.Vars != nil {
		domain.MergeIfUnset(env, wf.Vars)
	}
	for _, p := range []string{filepath.Join(wfRoot, ".env"), filepath.Join(repoRoot, ".env")} {
		m, err := infrastructure.ParseEnvFile(p)
		if err == nil {
			domain.MergeIfUnset(env, m)
		}
	}
	for _, ef := range opts.EnvFiles {
		m, err := infrastructure.ParseEnvFile(ef)
		if err == nil {
			domain.MergeIfUnset(env, m)
		}
	}
	if v := os.Getenv("DOCKPIPE_ENV_FILE"); v != "" {
		m, err := infrastructure.ParseEnvFile(v)
		if err == nil {
			domain.MergeIfUnset(env, m)
		}
	}
	for _, vo := range opts.VarOverrides {
		k, val, _ := strings.Cut(vo, "=")
		env[strings.TrimSpace(k)] = strings.TrimSpace(val)
	}
}

func lockedKeys(vars []string) map[string]bool {
	m := make(map[string]bool)
	for _, vo := range vars {
		k, _, _ := strings.Cut(vo, "=")
		m[strings.TrimSpace(k)] = true
	}
	return m
}

func mergeCommitEnvFromLines(env map[string]string, lines []string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "DOCKPIPE_COMMIT_MESSAGE", "DOCKPIPE_WORK_BRANCH", "DOCKPIPE_BUNDLE_OUT", "GIT_PAT":
			env[k] = v
		}
	}
}

func applyBranchPrefix(env map[string]string, resolver, templateName string) {
	if env["DOCKPIPE_BRANCH_PREFIX"] != "" {
		return
	}
	if resolver != "" {
		env["DOCKPIPE_BRANCH_PREFIX"] = resolver
		return
	}
	env["DOCKPIPE_BRANCH_PREFIX"] = domain.BranchPrefixForTemplate(templateName)
}

// parallelMergeState tracks which step last set each key during a parallel batch output merge
// (declaration order → last wins; used for [merge] overwrite logs).
type parallelMergeState struct {
	keySource map[string]string
}

func newParallelMergeState() *parallelMergeState {
	return &parallelMergeState{keySource: make(map[string]string)}
}

// applyOutputsFile merges KEY=VAL from path into envMap and dockerEnv. Existing keys are
// overwritten (callers rely on this for “last merge wins”, including parallel aggregate order).
// If mergeState and mergeSource are set, logs when a key already exists (parallel aggregate).
func applyOutputsFile(path string, envMap, dockerEnv map[string]string, locked map[string]bool, mergeState *parallelMergeState, mergeSource string) {
	m, err := infrastructure.ParseEnvFile(path)
	if err != nil || len(m) == 0 {
		return
	}
	rel := path
	fmt.Fprintf(os.Stderr, "[dockpipe] Merging outputs from %s into environment (next step)\n", rel)
	for k, v := range m {
		if locked[k] {
			continue
		}
		if mergeState != nil && mergeSource != "" {
			if _, had := envMap[k]; had {
				prev := "environment before parallel batch"
				if ps, ok := mergeState.keySource[k]; ok {
					prev = ps
				}
				fmt.Fprintf(os.Stderr, "[dockpipe] [merge] variable %q overwritten by %s (previously set by %s)\n", k, mergeSource, prev)
			}
			mergeState.keySource[k] = mergeSource
		}
		envMap[k] = v
		dockerEnv[k] = v
	}
	_ = os.Remove(path)
}

func appendUniqueEnv(slice []string, pair string) []string {
	k, _, _ := strings.Cut(pair, "=")
	for _, e := range slice {
		if strings.HasPrefix(e, k+"=") {
			return slice
		}
	}
	return append(slice, pair)
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
