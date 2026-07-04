package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolvePackagedWorkflowConfigPath finds config.yml for workflowName whose namespace: field matches
// the requested namespace (e.g. package: dockpipe.cloudflare on the parent step). Searches on-disk
// unpacked packages and workflows/ (staging, templates, etc.) — not tar:// blobs yet.
func ResolvePackagedWorkflowConfigPath(repoRoot, workdir, workflowName, namespace string) (string, error) {
	workflowName = strings.TrimSpace(workflowName)
	namespace = strings.TrimSpace(namespace)
	if workflowName == "" {
		return "", fmt.Errorf("packaged workflow name is empty")
	}
	if namespace == "" {
		return "", fmt.Errorf("packaged workflow namespace is empty")
	}
	for _, p := range packagedWorkflowDiskCandidates(repoRoot, workdir, workflowName) {
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			continue
		}
		ns, err := readWorkflowNamespaceFromDiskPath(p)
		if err != nil {
			continue
		}
		if ns != namespace {
			continue
		}
		return p, nil
	}
	return "", fmt.Errorf("no workflow %q with namespace %q found (set namespace: in config.yml; unpacked packages under bin/.dockpipe/internal/packages/workflows/ or workflows/)", workflowName, namespace)
}

// ResolvePackageWorkflowConfigPath finds config.yml for workflowName inside a package whose
// nearest package.yml has name: packageName. This powers top-level `dockpipe --package <name>`
// without overloading workflow namespace:, which remains author/org metadata.
func ResolvePackageWorkflowConfigPath(repoRoot, workdir, workflowName, packageName string) (string, error) {
	workflowName = strings.TrimSpace(workflowName)
	packageName = strings.TrimSpace(packageName)
	if workflowName == "" {
		return "", fmt.Errorf("packaged workflow name is empty")
	}
	if packageName == "" {
		return "", fmt.Errorf("package name is empty")
	}
	for _, p := range packagedWorkflowDiskCandidates(repoRoot, workdir, workflowName) {
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			continue
		}
		names, err := readAncestorPackageNamesForPath(p)
		if err != nil || len(names) == 0 {
			continue
		}
		if !stringSliceContains(names, packageName) {
			continue
		}
		return p, nil
	}
	return "", fmt.Errorf("no workflow %q in package %q found (set name: in nearest package.yml and ensure the package is under configured workflow roots)", workflowName, packageName)
}

func packagedWorkflowDiskCandidates(repoRoot, workdir, name string) []string {
	var out []string
	if strings.TrimSpace(workdir) != "" {
		if pw, err := PackagesWorkflowsDir(workdir); err == nil {
			out = append(out, filepath.Join(pw, name, "config.yml"))
		}
	}
	if gw, err := GlobalPackagesWorkflowsDir(); err == nil {
		out = append(out, filepath.Join(gw, name, "config.yml"))
	}
	out = append(out, filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"))
	if !UsesBundledAssetLayout(repoRoot) {
		out = append(out, nestedWorkflowConfigCandidates(repoRoot, name, WorkflowCompileRootsCached(repoRoot))...)
	}
	if strings.TrimSpace(workdir) != "" && workdir != repoRoot && !UsesBundledAssetLayout(workdir) {
		out = append(out, nestedWorkflowConfigCandidates(workdir, name, WorkflowCompileRootsCached(workdir))...)
	}
	if !UsesBundledAssetLayout(repoRoot) && !DockpipeAuthoringSourceTree(repoRoot) {
		out = append(out, filepath.Join(repoRoot, "templates", name, "config.yml"))
	}
	return out
}

func readWorkflowNamespaceFromDiskPath(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return parseWorkflowNamespaceYAML(b)
}

func readAncestorPackageNamesForPath(path string) ([]string, error) {
	cur := filepath.Dir(path)
	var out []string
	for {
		manifest := filepath.Join(cur, PackageManifestFilename)
		if st, err := os.Stat(manifest); err == nil && !st.IsDir() {
			b, err := os.ReadFile(manifest)
			if err != nil {
				return nil, err
			}
			var top struct {
				Name string `yaml:"name"`
			}
			if err := yaml.Unmarshal(b, &top); err != nil {
				return nil, err
			}
			if name := strings.TrimSpace(top.Name); name != "" {
				out = append(out, name)
			}
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no ancestor package.yml for %s", path)
	}
	return out, nil
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func parseWorkflowNamespaceYAML(b []byte) (string, error) {
	var top struct {
		Namespace string `yaml:"namespace"`
	}
	if err := yaml.Unmarshal(b, &top); err != nil {
		return "", err
	}
	return strings.TrimSpace(top.Namespace), nil
}

// NormalizeRuntimeProfileName maps legacy names to bundled runtime profile dirs under core/runtimes/<name>.
// Canonical substrates: dockerimage, dockerfile, vmimage, package (nested workflow entry). Legacy names
// (docker, cli, powershell, cmd, kube-pod, kubepod, cloud, keystore) normalize to dockerimage.
func NormalizeRuntimeProfileName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	switch low {
	case "docker":
		return "dockerimage"
	case "cli", "powershell", "cmd", "kube-pod", "kubepod", "cloud", "keystore":
		return "dockerimage"
	case "dockerimage", "dockerfile", "vmimage", "package":
		return low
	default:
		return s
	}
}
