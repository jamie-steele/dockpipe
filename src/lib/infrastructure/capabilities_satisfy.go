package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SatisfiedCapabilityIDs returns dotted capability ids declared by resolver packages (package.yml capability:).
func SatisfiedCapabilityIDs(workdir, repoRoot string) map[string]struct{} {
	out := make(map[string]struct{})
	if pr, err := PackagesRoot(workdir); err == nil {
		mergeResolverCapabilitiesFromDir(filepath.Join(pr, "resolvers"), out)
	}
	if gr, err := GlobalPackagesRoot(); err == nil {
		mergeResolverCapabilitiesFromDir(filepath.Join(gr, "resolvers"), out)
	}
	cd := CoreDir(repoRoot)
	mergeResolverCapabilitiesFromDir(filepath.Join(cd, "resolvers"), out)
	return out
}

type yamlCapabilityOnly struct {
	Capability string `yaml:"capability"`
	Primitive  string `yaml:"primitive"`
}

func readResolverCapabilityFromPackageYML(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var m yamlCapabilityOnly
	if err := yaml.Unmarshal(b, &m); err != nil {
		return "", err
	}
	c := strings.TrimSpace(m.Capability)
	if c == "" {
		c = strings.TrimSpace(m.Primitive)
	}
	return c, nil
}

func mergeResolverCapabilitiesFromDir(resolversDir string, out map[string]struct{}) {
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
		out[c] = struct{}{}
	}
}

// CheckWorkflowPackageRequiresCapabilities ensures sibling package.yml requires_capabilities are
// satisfied by installed resolver packages. Skips when the workflow is loaded from a tar URI,
// when package.yml is missing, or when requires_capabilities is empty.
func CheckWorkflowPackageRequiresCapabilities(workdir, repoRoot, wfRoot, wfConfigPath string) error {
	if strings.HasPrefix(strings.TrimSpace(wfConfigPath), "tar://") {
		return nil
	}
	pmPath := filepath.Join(wfRoot, PackageManifestFilename)
	b, err := os.ReadFile(pmPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var pm struct {
		RequiresCapabilities []string `yaml:"requires_capabilities"`
		RequiresPrimitives   []string `yaml:"requires_primitives"`
	}
	if err := yaml.Unmarshal(b, &pm); err != nil {
		return fmt.Errorf("package.yml: %w", err)
	}
	req := pm.RequiresCapabilities
	if len(req) == 0 {
		req = pm.RequiresPrimitives
	}
	if len(req) == 0 {
		return nil
	}
	sat := SatisfiedCapabilityIDs(workdir, repoRoot)
	var missing []string
	for _, r := range req {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, ok := sat[r]; !ok {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"workflow package requires capabilities not satisfied by installed resolver packages: %s — install resolvers that declare these in package.yml (see docs/capabilities.md)",
		strings.Join(missing, ", "),
	)
}
