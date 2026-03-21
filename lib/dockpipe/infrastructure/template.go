package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// TemplateBuild maps template name → image name and Dockerfile directory under templates/core/assets/images/.
func TemplateBuild(repoRoot, name string) (image string, dockerfileDir string, ok bool) {
	coreImg := func(n string) string {
		return filepath.Join(repoRoot, "templates", "core", "assets", "images", n)
	}
	switch name {
	case "base-dev":
		return "dockpipe-base-dev", coreImg("base-dev"), true
	case "dev":
		return "dockpipe-dev", coreImg("dev"), true
	case "agent-dev", "claude":
		return "dockpipe-claude", coreImg("claude"), true
	case "codex":
		return "dockpipe-codex", coreImg("codex"), true
	case "vscode":
		return "dockpipe-vscode", coreImg("vscode"), true
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
