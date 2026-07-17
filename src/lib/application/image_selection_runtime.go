package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

func applyCompiledImageSelectionInputs(repoRoot, wfRoot string, rm *domain.CompiledRuntimeManifest, image, buildDir, buildCtx string) (string, string, string) {
	if rm == nil {
		return image, buildDir, buildCtx
	}
	switch strings.TrimSpace(rm.Image.Source) {
	case "build":
		if strings.TrimSpace(rm.Image.Ref) != "" {
			image = strings.TrimSpace(rm.Image.Ref)
		}
		if rm.Image.Build != nil {
			if dockerfilePath := strings.TrimSpace(rm.Image.Build.Dockerfile); dockerfilePath != "" {
				buildDir = filepath.Dir(absRuntimeBuildPath(repoRoot, wfRoot, dockerfilePath))
			}
			if contextPath := strings.TrimSpace(rm.Image.Build.Context); contextPath != "" {
				buildCtx = absRuntimeBuildPath(repoRoot, wfRoot, contextPath)
			}
		}
	case "registry":
		if strings.TrimSpace(rm.Image.Ref) != "" {
			image = strings.TrimSpace(rm.Image.Ref)
			buildDir = ""
			buildCtx = ""
		}
	}
	return image, buildDir, buildCtx
}

func absRuntimeBuildPath(repoRoot, wfRoot, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if strings.TrimSpace(wfRoot) != "" {
		candidate := filepath.Join(wfRoot, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return absFromRepoRoot(repoRoot, path)
}

func ensureCompiledRegistryImageForWorkflow(rm *domain.CompiledRuntimeManifest) (string, error) {
	return ensureCompiledRegistryImage(rm.Image, rm.Security.Network, dockerImageExistsAppFn, dockerPullAppFn)
}

func ensureCompiledRegistryImageForStep(rm *domain.CompiledRuntimeManifest) (string, error) {
	return ensureCompiledRegistryImage(rm.Image, rm.Security.Network, dockerImageExistsFn, dockerPullFn)
}

func ensureCompiledRegistryImage(sel domain.CompiledImageSelection, network domain.CompiledNetworkPolicy, existsFn func(string) (bool, error), pullFn func(string) error) (string, error) {
	if strings.TrimSpace(sel.Source) != "registry" || strings.TrimSpace(sel.Ref) == "" {
		return "", nil
	}
	ref := strings.TrimSpace(sel.Ref)
	ok, err := existsFn(ref)
	if err != nil {
		return "", err
	}
	if ok {
		if strings.TrimSpace(sel.ExpectedDigest) != "" {
			return fmt.Sprintf("using local registry image %s (digest-pinned)", ref), nil
		}
		return fmt.Sprintf("using local registry image %s", ref), nil
	}
	if strings.TrimSpace(sel.PullPolicy) != "if-missing" {
		return "", fmt.Errorf("registry image %s is not present locally and pull_policy=%q does not allow pulling during run", ref, firstNonEmptyString(strings.TrimSpace(sel.PullPolicy), "never"))
	}
	if strings.TrimSpace(network.Mode) != "internet" {
		return "", fmt.Errorf("registry image %s is not present locally and compiled network policy %q does not allow pulling during run", ref, firstNonEmptyString(strings.TrimSpace(network.Mode), "offline"))
	}
	if err := pullFn(ref); err != nil {
		return "", fmt.Errorf("pull registry image %s: %w", ref, err)
	}
	return fmt.Sprintf("pulled registry image %s", ref), nil
}

func absFromRepoRoot(repoRoot, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || repoRoot == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(repoRoot, path)
}
