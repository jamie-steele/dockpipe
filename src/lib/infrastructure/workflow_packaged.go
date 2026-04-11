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
// Canonical substrates: dockerimage, dockerfile, package (nested workflow entry). Legacy names
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
	case "dockerimage", "dockerfile", "package":
		return low
	default:
		return s
	}
}
