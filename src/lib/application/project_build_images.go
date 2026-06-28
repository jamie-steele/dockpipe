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

func prebuildCompiledImageArtifacts(workdir string) (int, error) {
	workflowDir, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return 0, err
	}
	matches, err := filepath.Glob(filepath.Join(workflowDir, "dockpipe-workflow-*.tar.gz"))
	if err != nil {
		return 0, err
	}
	total := 0
	for _, tgz := range matches {
		n, err := prebuildImageArtifactsFromWorkflowTarball(workdir, tgz)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func prebuildImageArtifactsFromWorkflowTarball(workdir, tgz string) (int, error) {
	paths, err := packagebuild.ListTarGzMemberPaths(tgz)
	if err != nil {
		return 0, err
	}
	var extractedRoot string
	ensureExtracted := func() (string, error) {
		if extractedRoot != "" {
			return extractedRoot, nil
		}
		root, err := packagebuild.EnsureTarballExtractedCache(tgz, infrastructure.TarballExtractCacheRoot(workdir))
		if err != nil {
			return "", err
		}
		extractedRoot = root
		return extractedRoot, nil
	}
	total := 0
	for _, entry := range paths {
		if !isImageArtifactTarEntry(entry) {
			continue
		}
		b, err := packagebuild.ReadFileFromTarGz(tgz, entry)
		if err != nil {
			return total, err
		}
		var artifact domain.ImageArtifactManifest
		if err := json.Unmarshal(b, &artifact); err != nil {
			return total, fmt.Errorf("%s:%s: %w", tgz, entry, err)
		}
		buildRoot := workdir
		if !imageArtifactBuildPathsExist(buildRoot, &artifact) {
			root, err := ensureExtracted()
			if err != nil {
				return total, err
			}
			buildRoot = filepath.Join(root, imageArtifactPackageRootFromTarEntry(entry))
		}
		built, err := prebuildCompiledImageArtifactFromRoot(workdir, buildRoot, &artifact)
		if err != nil {
			return total, fmt.Errorf("%s:%s: %w", tgz, entry, err)
		}
		if built {
			total++
		}
	}
	return total, nil
}

func isImageArtifactTarEntry(entry string) bool {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	return strings.HasSuffix(entry, "/"+domain.RuntimeManifestDirName+"/"+domain.ImageArtifactFileName) ||
		(strings.Contains(entry, "/"+domain.RuntimeManifestDirName+"/"+domain.StepArtifactsDirName+"/") && strings.HasSuffix(entry, ".image-artifact.json"))
}

func imageArtifactPackageRootFromTarEntry(entry string) string {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	needle := "/" + domain.RuntimeManifestDirName + "/"
	idx := strings.Index(entry, needle)
	if idx <= 0 {
		return "."
	}
	return entry[:idx]
}

func imageArtifactBuildPathsExist(root string, artifact *domain.ImageArtifactManifest) bool {
	if artifact == nil || artifact.Build == nil {
		return true
	}
	dockerfilePath := absFromRepoRoot(root, strings.TrimSpace(artifact.Build.Dockerfile))
	contextPath := absFromRepoRoot(root, strings.TrimSpace(artifact.Build.Context))
	if strings.TrimSpace(dockerfilePath) == "" || strings.TrimSpace(contextPath) == "" {
		return false
	}
	df, err := os.Stat(dockerfilePath)
	if err != nil || df.IsDir() {
		return false
	}
	ctx, err := os.Stat(contextPath)
	return err == nil && ctx.IsDir()
}

func prebuildCompiledImageArtifact(workdir string, artifact *domain.ImageArtifactManifest) (bool, error) {
	return prebuildCompiledImageArtifactFromRoot(workdir, workdir, artifact)
}

func prebuildCompiledImageArtifactFromRoot(workdir, buildRoot string, artifact *domain.ImageArtifactManifest) (bool, error) {
	if artifact == nil || strings.TrimSpace(artifact.Source) != "build" || artifact.Build == nil {
		return false, nil
	}
	ref := strings.TrimSpace(artifact.ImageRef)
	if ref == "" {
		return false, nil
	}
	exists, err := dockerImageExistsAppFn(ref)
	if err != nil {
		return false, err
	}
	if exists {
		artifact.ArtifactState = "materialized"
		if err := persistImageArtifactIndexRecord(workdir, artifact); err != nil {
			return false, err
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] image: ready materialized image artifact %s (%s)\n", firstNonEmptyString(strings.TrimSpace(artifact.ImageKey), ref), ref)
		return false, nil
	}
	dockerfilePath := absFromRepoRoot(buildRoot, strings.TrimSpace(artifact.Build.Dockerfile))
	contextPath := absFromRepoRoot(buildRoot, strings.TrimSpace(artifact.Build.Context))
	if strings.TrimSpace(dockerfilePath) == "" || strings.TrimSpace(contextPath) == "" {
		return false, fmt.Errorf("build image artifact %s is missing dockerfile/context", firstNonEmptyString(strings.TrimSpace(artifact.ImageKey), ref))
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] image: materializing image artifact %s (%s)\n", firstNonEmptyString(strings.TrimSpace(artifact.ImageKey), ref), ref)
	if err := dockerBuildAppFn(ref, filepath.Dir(dockerfilePath), contextPath); err != nil {
		return false, err
	}
	artifact.ArtifactState = "materialized"
	if err := persistImageArtifactIndexRecord(workdir, artifact); err != nil {
		return false, err
	}
	return true, nil
}

func persistImageArtifactIndexRecord(workdir string, artifact *domain.ImageArtifactManifest) error {
	if artifact == nil || strings.TrimSpace(artifact.Fingerprint) == "" {
		return nil
	}
	root, err := infrastructure.ImageArtifactIndexDir(workdir)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "by-fingerprint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := infrastructure.SanitizePackageStateScope(strings.TrimPrefix(strings.TrimSpace(artifact.Fingerprint), "sha256:")) + ".json"
	b, err := marshalArtifactJSON(artifact)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), b, 0o644)
}
