package domain

import (
	"fmt"
	"os"
	"strings"

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
	// Kind hints for tooling: workflow | resolver | core | assets | bundle | package (optional).
	// kind: package — umbrella metadata for a maintainer tree (e.g. dockpipe/agent) whose child resolvers
	// live under resolvers/; use includes_resolvers for optional resolver profile names (store installs stay per-resolver).
	Kind string `yaml:"kind,omitempty"`
	// Provider: optional platform / vendor id for filtering and store facets (e.g. cloudflare, aws, github).
	// Use a short stable label, not a URL — see docs/package-model.md.
	Provider string `yaml:"provider,omitempty"`
	// Capability: dotted capability id this resolver package provides (e.g. cli.codex, app.vscode). See docs/capabilities.md.
	Capability string `yaml:"capability,omitempty"`
	// PrimitiveYAMLDeprecated is the deprecated YAML key "primitive" — merged into Capability after parse.
	PrimitiveYAMLDeprecated string `yaml:"primitive,omitempty"`
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
	// RequiresCapabilities: for kind workflow — dotted capability ids this workflow expects (e.g. cli.codex).
	// See docs/capabilities.md. Complements requires_resolvers (profile names).
	RequiresCapabilities []string `yaml:"requires_capabilities,omitempty"`
	// RequiresPrimitivesYAMLDeprecated is the deprecated YAML key "requires_primitives" — merged into RequiresCapabilities after parse.
	RequiresPrimitivesYAMLDeprecated []string `yaml:"requires_primitives,omitempty"`
	// RequiresResolvers hints default or required resolver profile names for a workflow package (optional).
	RequiresResolvers []string `yaml:"requires_resolvers,omitempty"`
	// IncludesResolvers lists resolver profile names under resolvers/ for kind: package (umbrella metadata only; not a single tarball).
	IncludesResolvers []string `yaml:"includes_resolvers,omitempty"`
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
	NormalizePackageManifestYAMLAliases(&m)
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
	if err := ValidateNamespace(m.Namespace); err != nil {
		return err
	}
	if err := ValidateProvider(m.Provider); err != nil {
		return err
	}
	if err := ValidateCapabilityID(m.Capability); err != nil {
		return err
	}
	for _, p := range m.RequiresCapabilities {
		if err := ValidateCapabilityID(p); err != nil {
			return err
		}
	}
	// kind-specific required fields kept minimal — capability / requires_capabilities are optional metadata.
	return nil
}

// NormalizePackageManifestYAMLAliases merges deprecated primitive / requires_primitives keys into Capability / RequiresCapabilities.
func NormalizePackageManifestYAMLAliases(m *PackageManifest) {
	if m == nil {
		return
	}
	if strings.TrimSpace(m.Capability) == "" {
		m.Capability = strings.TrimSpace(m.PrimitiveYAMLDeprecated)
	}
	if len(m.RequiresCapabilities) == 0 && len(m.RequiresPrimitivesYAMLDeprecated) > 0 {
		m.RequiresCapabilities = append([]string(nil), m.RequiresPrimitivesYAMLDeprecated...)
	}
}

// ValidateProvider checks optional provider metadata (platform/vendor id for filtering).
func ValidateProvider(s string) error {
	return validateOptionalMetadataString(s, "provider")
}

// ValidateCapabilityID checks optional dotted capability id (e.g. cli.codex) — see docs/capabilities.md.
func ValidateCapabilityID(s string) error {
	return validateOptionalMetadataString(s, "capability")
}

// ValidatePrimitive is deprecated: use ValidateCapabilityID. Kept for transitional call sites.
func ValidatePrimitive(s string) error {
	return ValidateCapabilityID(s)
}

func validateOptionalMetadataString(s, field string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) > 256 {
		return fmt.Errorf("%s: length exceeds 256", field)
	}
	for _, r := range s {
		if r < 0x20 {
			return fmt.Errorf("%s: control characters not allowed", field)
		}
	}
	return nil
}
