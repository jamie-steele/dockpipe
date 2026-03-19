package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// TemplateBuild maps template name → image name and Dockerfile directory under repo root.
func TemplateBuild(repoRoot, name string) (image string, dockerfileDir string, ok bool) {
	switch name {
	case "base-dev":
		return "dockpipe-base-dev", filepath.Join(repoRoot, "images/base-dev"), true
	case "dev":
		return "dockpipe-dev", filepath.Join(repoRoot, "images/dev"), true
	case "agent-dev", "claude":
		return "dockpipe-claude", filepath.Join(repoRoot, "images/claude"), true
	case "codex":
		return "dockpipe-codex", filepath.Join(repoRoot, "images/codex"), true
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
