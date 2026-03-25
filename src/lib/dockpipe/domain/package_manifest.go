package domain

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PackageManifest is optional metadata for a DockPipe package (workflow slice, core slice, or asset pack).
// Stored as package.yml next to the package contents. Schema may evolve; extra YAML keys are ignored by the parser.
// Rich fields support store discovery, authoring, and dependency hints (workflows vs resolver packs).
type PackageManifest struct {
	Schema      int    `yaml:"schema"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Author      string `yaml:"author"`
	Website     string `yaml:"website"`
	License     string `yaml:"license"`
	// Kind hints for tooling: workflow | resolver | core | assets | bundle (optional).
	Kind string `yaml:"kind,omitempty"`
	// Namespace: optional author/org label (same rules as workflow namespace; optional).
	Namespace string `yaml:"namespace,omitempty"`
	// Tags and keywords: search / store facets (optional).
	Tags     []string `yaml:"tags,omitempty"`
	Keywords []string `yaml:"keywords,omitempty"`
	// MinDockpipeVersion is a semver constraint for the CLI/engine (optional).
	MinDockpipeVersion string `yaml:"min_dockpipe_version,omitempty"`
	// Repository is source URL (optional).
	Repository string `yaml:"repository,omitempty"`
	// Provides names capabilities for resolver-style packages (e.g. codex, claude-code).
	Provides []string `yaml:"provides,omitempty"`
	// RequiresResolvers hints default or required resolver profile names for a workflow package (optional).
	RequiresResolvers []string `yaml:"requires_resolvers,omitempty"`
	// Depends lists other package names this package expects (optional).
	Depends []string `yaml:"depends,omitempty"`
	// AllowClone: when true, dockpipe clone may copy this compiled package into an authoring tree (e.g. workflows/).
	// Omitted or false: clone is refused — use for commercial/binary-only drops where the publisher does not grant source export.
	AllowClone bool `yaml:"allow_clone,omitempty"`
	// Distribution is optional policy for humans and tooling: "source" (recoverable YAML/assets) or "binary" (no meaningful source in the artifact).
	// Binary releases should set allow_clone: false and ship only non-recoverable artifacts if reverse-engineering must be impractical.
	Distribution string `yaml:"distribution,omitempty"`
}

// ParsePackageManifest reads and parses package.yml from path.
func ParsePackageManifest(path string) (*PackageManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m PackageManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := ValidatePackageManifest(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// ValidatePackageManifest checks optional fields (e.g. namespace) after YAML decode.
func ValidatePackageManifest(m *PackageManifest) error {
	if m == nil {
		return nil
	}
	return ValidateNamespace(m.Namespace)
}
