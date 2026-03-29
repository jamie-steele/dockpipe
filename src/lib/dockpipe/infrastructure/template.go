package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// DockerfileDir returns the directory that contains Dockerfile for a template isolate name.
// Searches compile.resolvers roots (nested) and compile.bundles roots, then templates/core (materialized bundle).
func DockerfileDir(repoRoot, name string) string {
	core := CoreDir(repoRoot)
	var candidates []string
	if !UsesBundledAssetLayout(repoRoot) {
		for _, leaf := range nestedResolverLeafDirs(repoRoot, name, ResolverCompileRootsCached(repoRoot)) {
			candidates = append(candidates, filepath.Join(leaf, "assets", "images", name))
		}
		for _, br := range BundleCompileRootsCached(repoRoot) {
			candidates = append(candidates, filepath.Join(br, name, "assets", "images", name))
		}
	}
	candidates = append(candidates,
		filepath.Join(core, "resolvers", name, "assets", "images", name),
		filepath.Join(core, "bundles", name, "assets", "images", name),
		filepath.Join(core, "assets", "images", name),
	)
	for _, d := range candidates {
		if st, err := os.Stat(filepath.Join(d, "Dockerfile")); err == nil && !st.IsDir() {
			return d
		}
	}
	return filepath.Join(core, "assets", "images", name)
}

// TemplateBuild maps template name → image name and Dockerfile directory for docker build.
func TemplateBuild(repoRoot, name string) (image string, dockerfileDir string, ok bool) {
	dir := func(n string) string {
		return DockerfileDir(repoRoot, n)
	}
	switch name {
	case "base-dev":
		return "dockpipe-base-dev", dir("base-dev"), true
	case "dev":
		return "dockpipe-dev", dir("dev"), true
	case "agent-dev", "claude":
		return "dockpipe-claude", dir("claude"), true
	case "codex":
		return "dockpipe-codex", dir("codex"), true
	case "vscode":
		return "dockpipe-vscode", dir("vscode"), true
	case "ollama":
		return "dockpipe-ollama", dir("ollama"), true
	case "steam-flatpak":
		return "dockpipe-steam-flatpak", dir("steam-flatpak"), true
	default:
		return "", "", false
	}
}

// MaybeVersionTag appends :version from repoRoot/version for dockpipe-* images only.
func MaybeVersionTag(repoRoot, image string) string {
	if image == "" {
		return image
	}
	if strings.Contains(image, ":") {
		return image
	}
	b, err := os.ReadFile(filepath.Join(repoRoot, "version"))
	if err != nil {
		return image
	}
	ver := strings.TrimSpace(string(b))
	if ver == "" {
		return image
	}
	if strings.HasPrefix(image, "dockpipe-") {
		return image + ":" + ver
	}
	return image
}
