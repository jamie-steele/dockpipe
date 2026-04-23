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
	return maybeSkipDockerBuildForArtifact(repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx, dockerImageExistsAppFn)
}

func maybeSkipDockerBuildForStep(repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx string) (bool, string, error) {
	return maybeSkipDockerBuildForArtifact(repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx, dockerImageExistsFn)
}

func maybeSkipDockerBuildForArtifact(repoRoot, wfConfig, wfRoot, image, buildDir, buildCtx string, imageExistsFn func(string) (bool, error)) (bool, string, error) {
	_ = repoRoot
	if strings.TrimSpace(image) == "" || strings.TrimSpace(buildDir) == "" || strings.TrimSpace(buildCtx) == "" {
		return false, "", nil
	}
	artifact, err := loadCompiledImageArtifactForWorkflow(wfConfig, wfRoot)
	if err != nil {
		return false, "", err
	}
	if artifact == nil || artifact.Build == nil || artifact.Source != "build" {
		return false, "", nil
	}
	if strings.TrimSpace(artifact.ImageRef) != strings.TrimSpace(image) {
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
