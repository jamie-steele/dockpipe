package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolverProfileNameForCapability returns templates/core/resolvers/<name> profile basename for a
// resolver package whose package.yml declares this capability. Empty string means no resolver
// implements the capability (e.g. workflow-only ids like workflow.*). Error if two resolvers declare the same id.
func ResolverProfileNameForCapability(workdir, repoRoot, capability string) (string, error) {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return "", nil
	}
	var matches []string
	add := func(resolversDir string) {
		entries, err := os.ReadDir(resolversDir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			p := filepath.Join(resolversDir, e.Name(), PackageManifestFilename)
			c, err := readResolverCapabilityFromPackageYML(p)
			if err != nil || c == "" {
				continue
			}
			if strings.TrimSpace(c) != capability {
				continue
			}
			matches = append(matches, e.Name())
		}
	}
	if pr, err := PackagesRoot(workdir); err == nil {
		add(filepath.Join(pr, "resolvers"))
	}
	if gr, err := GlobalPackagesRoot(); err == nil {
		add(filepath.Join(gr, "resolvers"))
	}
	cd := CoreDir(repoRoot)
	add(filepath.Join(cd, "resolvers"))

	if len(matches) == 0 {
		return "", nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("capability %q is declared by multiple resolver packages: %s — disambiguate in package.yml or use resolver: explicitly", capability, strings.Join(matches, ", "))
	}
	return matches[0], nil
}

// SubstrateRuntimeFromDockpipeCapability returns a bundled runtime profile name when the capability id
// uses the dockpipe.* namespace for isolation. Host-style ids map to dockerimage (same bundled profile tree).
func SubstrateRuntimeFromDockpipeCapability(capability string) string {
	capability = strings.TrimSpace(strings.ToLower(capability))
	if capability == "" || !strings.HasPrefix(capability, "dockpipe.") {
		return ""
	}
	rest := strings.TrimPrefix(capability, "dockpipe.")
	switch {
	case rest == "cli" || strings.HasPrefix(rest, "cli."):
		return "dockerimage"
	case rest == "powershell" || strings.HasPrefix(rest, "powershell."):
		return "dockerimage"
	case rest == "cmd" || strings.HasPrefix(rest, "cmd."):
		return "dockerimage"
	case rest == "docker" || strings.HasPrefix(rest, "docker."):
		return "dockerimage"
	case rest == "dockerfile" || strings.HasPrefix(rest, "dockerfile."):
		return "dockerfile"
	case rest == "package" || strings.HasPrefix(rest, "package."):
		return "package"
	case rest == "kubepod" || rest == "kube-pod" || strings.HasPrefix(rest, "kubepod.") || strings.HasPrefix(rest, "kube-pod."):
		return "dockerimage"
	case rest == "keystore" || strings.HasPrefix(rest, "keystore."):
		return "dockerimage"
	case rest == "cloud" || strings.HasPrefix(rest, "cloud."):
		return "dockerimage"
	default:
		return ""
	}
}
