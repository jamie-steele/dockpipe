package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"
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
		case "DOCKPIPE_COMMIT_MESSAGE", "DOCKPIPE_WORK_BRANCH", "DOCKPIPE_BUNDLE_OUT", "DOCKPIPE_BUNDLE_ALL", "GIT_PAT":
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

// isLikelySecretEnvKey matches env var names that should not be cleared by an empty value from
// step outputs (e.g. OPENAI_API_KEY= in .dockpipe/outputs.env).
func isLikelySecretEnvKey(k string) bool {
	k = strings.ToUpper(strings.TrimSpace(k))
	if strings.HasSuffix(k, "_API_KEY") {
		return true
	}
	if strings.HasSuffix(k, "_TOKEN") {
		return true
	}
	switch k {
	case "GIT_PAT", "GITHUB_TOKEN", "GITHUB_PAT", "GITLAB_TOKEN":
		return true
	default:
		return false
	}
}

// skipOutputsOverwriteEmptySecret is true when outputs would set an empty value and wipe a
// non-empty host/workflow secret (mergeResolverAuthEnvFromHost and docker -e rely on envMap).
func skipOutputsOverwriteEmptySecret(k, v string, envMap map[string]string) bool {
	if strings.TrimSpace(v) != "" {
		return false
	}
	if strings.TrimSpace(envMap[k]) == "" {
		return false
	}
	return isLikelySecretEnvKey(k)
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
		if skipOutputsOverwriteEmptySecret(k, v, envMap) {
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

// appendUniqueEnv appends pair, or replaces an existing entry with the same key prefix (KEY=).
// Replacement matters when envSlice was built from os.Environ(): inherited DOCKPIPE_WORKDIR must not
// block --workdir, and explicit dockpipe-injected values must win.
func appendUniqueEnv(slice []string, pair string) []string {
	k, _, _ := strings.Cut(pair, "=")
	for i, e := range slice {
		if strings.HasPrefix(e, k+"=") {
			slice[i] = pair
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
