package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func maybeSkipDockerBuildForWorkflow(repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx string) (bool, string, error) {
	return maybeSkipDockerBuildForArtifact(repoRoot, repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx, dockerImageExistsAppFn)
}

func maybeSkipDockerBuildForStep(stateWorkdir, repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx string) (bool, string, error) {
	return maybeSkipDockerBuildForArtifact(stateWorkdir, repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx, dockerImageExistsFn)
}

func maybeSkipDockerBuildForArtifact(stateWorkdir, repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx string, imageExistsFn func(string) (bool, error)) (bool, string, error) {
	if strings.TrimSpace(image) == "" || strings.TrimSpace(buildDir) == "" || strings.TrimSpace(buildCtx) == "" {
		return false, "", nil
	}
	policyFingerprint, err := runtimePolicyFingerprintForRun(wfConfig, wfRoot)
	if err != nil {
		return false, "", err
	}
	artifact, err := loadCompiledImageArtifactForWorkflow(wfConfig, wfRoot)
	if err != nil {
		return false, "", err
	}
	if artifact == nil {
		artifact, err = loadCachedImageArtifactForIsolate(stateWorkdir, image)
		if err != nil {
			return false, "", err
		}
	}
	if artifact == nil || artifact.Build == nil || artifact.Source != "build" {
		return false, "", nil
	}
	if strings.TrimSpace(artifact.SecurityManifestFingerprint) != strings.TrimSpace(policyFingerprint) {
		return false, "", nil
	}
	expected, err := buildImageArtifactManifest(repoRoot, "", "", artifact.ImageKey, image, buildDir, buildCtx, policyFingerprint)
	if err != nil {
		return false, "", err
	}
	if strings.TrimSpace(artifact.ImageRef) != strings.TrimSpace(image) || strings.TrimSpace(artifact.Fingerprint) != strings.TrimSpace(expected.Fingerprint) {
		return false, "", nil
	}
	ok, err := imageExistsFn(image)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "compiled image artifact found but local image is missing", nil
	}
	return true, fmt.Sprintf("using cached image artifact %s", artifact.ImageKey), nil
}

func runtimePolicyFingerprintForRun(wfConfig, wfRoot string) (string, error) {
	rm, err := loadCompiledRuntimeManifestForWorkflow(wfConfig, wfRoot)
	if err != nil {
		return "", err
	}
	if rm != nil && strings.TrimSpace(rm.PolicyFingerprint) != "" {
		return strings.TrimSpace(rm.PolicyFingerprint), nil
	}
	return defaultRuntimePolicyFingerprint()
}

func loadCompiledRuntimeManifestForWorkflow(wfConfig, wfRoot string) (*domain.CompiledRuntimeManifest, error) {
	if tarPath, entry, ok := infrastructure.SplitTarWorkflowURI(wfConfig); ok {
		manifestEntry := filepath.ToSlash(filepath.Join(filepath.Dir(entry), domain.RuntimeManifestDirName, domain.RuntimeManifestFileName))
		b, err := packagebuild.ReadFileFromTarGz(tarPath, manifestEntry)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, nil
			}
			return nil, err
		}
		var m domain.CompiledRuntimeManifest
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		return &m, nil
	}
	if strings.TrimSpace(wfRoot) == "" {
		return nil, nil
	}
	p := filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.RuntimeManifestFileName)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m domain.CompiledRuntimeManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func defaultRuntimePolicyFingerprint() (string, error) {
	return domain.FingerprintJSON(domain.CompiledSecurityPolicy{
		Preset: "secure-default",
		Network: domain.CompiledNetworkPolicy{
			Mode:        "restricted",
			Enforcement: "advisory",
			InternalDNS: true,
		},
		FS: domain.CompiledFilesystemPolicy{
			Root:      "readonly",
			Writes:    "workspace-only",
			TempPaths: []string{"/tmp"},
		},
		Process: domain.CompiledProcessPolicy{
			User:            "non-root",
			NoNewPrivileges: true,
			DropCaps:        []string{"ALL"},
			PIDLimit:        256,
		},
	})
}

func persistCachedImageArtifactForIsolate(stateWorkdir, image string, artifact *domain.ImageArtifactManifest) error {
	if artifact == nil {
		return nil
	}
	root, err := infrastructure.StateRoot(stateWorkdir)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "internal", "cache", "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := infrastructure.SanitizePackageStateScope(image) + ".json"
	b, err := marshalArtifactJSON(artifact)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), b, 0o644)
}

func loadCachedImageArtifactForIsolate(stateWorkdir, image string) (*domain.ImageArtifactManifest, error) {
	root, err := infrastructure.StateRoot(stateWorkdir)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(root, "internal", "cache", "images", infrastructure.SanitizePackageStateScope(image)+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m domain.ImageArtifactManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func loadCompiledImageArtifactForWorkflow(wfConfig, wfRoot string) (*domain.ImageArtifactManifest, error) {
	if tarPath, entry, ok := infrastructure.SplitTarWorkflowURI(wfConfig); ok {
		artifactEntry := filepath.ToSlash(filepath.Join(filepath.Dir(entry), domain.RuntimeManifestDirName, domain.ImageArtifactFileName))
		b, err := packagebuild.ReadFileFromTarGz(tarPath, artifactEntry)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, nil
			}
			return nil, err
		}
		var m domain.ImageArtifactManifest
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		return &m, nil
	}
	if strings.TrimSpace(wfRoot) == "" {
		return nil, nil
	}
	p := filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.ImageArtifactFileName)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m domain.ImageArtifactManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
