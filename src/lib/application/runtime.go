package application

import (
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// EffectiveRuntimeProfileName returns the isolation **runtime** profile name (templates/core/runtimes/<name>).
// Precedence: CLI --runtime, then workflow runtime.
func EffectiveRuntimeProfileName(opts *CliOpts, wf *domain.Workflow, stepsMode bool) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Runtime); s != "" {
			return s
		}
	}
	if wf == nil {
		return ""
	}
	return strings.TrimSpace(wf.Runtime)
}

// EffectiveResolverProfileName returns the **resolver** (tool adapter) profile name (templates/core/resolvers/<name>).
// Precedence: CLI --resolver, then workflow resolver.
func EffectiveResolverProfileName(opts *CliOpts, wf *domain.Workflow, stepsMode bool) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Resolver); s != "" {
			return s
		}
	}
	if wf == nil {
		return ""
	}
	return strings.TrimSpace(wf.Resolver)
}

// EffectiveLegacyIsolateName returns workflow isolate: when no explicit runtime/resolver names were set.
// Used to pair runtimes/<name> + resolvers/<name> for legacy single-field workflows.
func EffectiveLegacyIsolateName(wf *domain.Workflow) string {
	if wf == nil {
		return ""
	}
	return strings.TrimSpace(wf.Isolate)
}

// ProfileLabelForEnv prefers resolver name for branch/env display, then runtime name.
func ProfileLabelForEnv(runtimeName, resolverName string) string {
	if s := strings.TrimSpace(resolverName); s != "" {
		return s
	}
	return strings.TrimSpace(runtimeName)
}

func loadMergedIsolationProfile(repoRoot, projectRoot, runtimeName, resolverName string) (map[string]string, error) {
	base, baseErr := infrastructure.LoadIsolationProfile(repoRoot, runtimeName, resolverName)
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return base, baseErr
	}
	repoAbs, _ := filepath.Abs(repoRoot)
	projectAbs, _ := filepath.Abs(projectRoot)
	if filepath.Clean(repoAbs) == filepath.Clean(projectAbs) {
		return base, baseErr
	}
	project, projectErr := infrastructure.LoadIsolationProfile(projectRoot, runtimeName, resolverName)
	if projectErr != nil {
		return base, baseErr
	}
	if baseErr != nil {
		return project, nil
	}
	for k, v := range project {
		base[k] = v
	}
	return base, nil
}

func templateBuildForRun(repoRoot, projectRoot, name string) (image, dockerfileDir, contextDir string, ok bool) {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot != "" {
		if im, dir, found := templateBuildAppFn(projectRoot, name); found && dockerfileExists(dir) {
			return im, dir, projectRoot, true
		}
	}
	im, dir, found := templateBuildAppFn(repoRoot, name)
	return im, dir, repoRoot, found
}

func dockerfileExists(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "Dockerfile"))
	return err == nil && !st.IsDir()
}

// ValidateRuntimeAllowlist is a no-op in the simplified authored workflow model.
func ValidateRuntimeAllowlist(wf *domain.Workflow, runtimeName string) error {
	return nil
}
