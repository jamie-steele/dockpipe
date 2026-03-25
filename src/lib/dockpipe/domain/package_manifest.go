package domain

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PackageManifest is optional metadata for a DockPipe package (workflow slice, core slice, or asset pack).
// Stored as package.yml next to the package contents. Schema may evolve; unknown fields are ignored at parse time.
type PackageManifest struct {
	Schema      int    `yaml:"schema"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Author      string `yaml:"author"`
	Website     string `yaml:"website"`
	License     string `yaml:"license"`
	// Kind hints for tooling: workflow | core | assets | bundle (optional).
	Kind string `yaml:"kind,omitempty"`
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
	return &m, nil
}
