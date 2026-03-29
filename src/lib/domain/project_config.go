package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DockpipeProjectConfigFileName is the repo-root JSON file for project-level DockPipe settings
// (compile source lists, future package registry hints). Optional — compile uses built-in defaults when absent.
const DockpipeProjectConfigFileName = "dockpipe.config.json"

// DockpipeProjectConfig is repo-root metadata (JSON). Schema may grow; unknown keys are ignored by encoding/json.
type DockpipeProjectConfig struct {
	Schema   int                    `json:"schema,omitempty"`
	Compile  DockpipeCompileConfig  `json:"compile,omitempty"`
	Secrets  DockpipeSecretsConfig  `json:"secrets,omitempty"`
	Packages DockpipePackagesConfig `json:"packages,omitempty"`
}

// DockpipeSecretsConfig points at host-side secret mapping (e.g. 1Password op inject), not secrets themselves.
type DockpipeSecretsConfig struct {
	// OpInjectTemplate is a repo-relative or absolute path to an env file with op:// lines (e.g. .env.op.template).
	OpInjectTemplate *string `json:"op_inject_template,omitempty"`
	// Notes is optional human-readable context for maintainers (shown by dockpipe doctor when present).
	Notes *string `json:"notes,omitempty"`
}

// DockpipeCompileConfig lists directories (repo-relative or absolute) used by `dockpipe package compile`.
// Pointer slices distinguish JSON "key absent" (nil → use CLI defaults) from "empty array" (non-nil, len 0 → compile nothing from that category).
type DockpipeCompileConfig struct {
	CoreFrom  *string   `json:"core_from,omitempty"` // optional override for compile core --from
	Workflows *[]string `json:"workflows,omitempty"` // single entry point: same roots used for workflow tarballs and resolver discovery (+ src/core/resolvers, templates/core/resolvers)
	Resolvers *[]string `json:"resolvers,omitempty"` // deprecated: merged into effective resolver roots if present (prefer compile.workflows only)
	Bundles   *[]string `json:"bundles,omitempty"`   // deprecated: merged into compile.workflows (same config.yml walk)
}

// DockpipePackagesConfig holds optional defaults for packaged workflows/resolvers and tarball resolution.
type DockpipePackagesConfig struct {
	// TarballDir is a repo-relative directory containing dockpipe-workflow-*.tar.gz (after package build store).
	// When unset, release/artifacts is used if that directory exists. Resolution also checks
	// <workdir>/.dockpipe/internal/packages/workflows/ first.
	TarballDir *string `json:"tarball_dir,omitempty"`
	// Namespace: default author/org label for compile (package.yml) when workflow/resolver metadata omits it;
	// when set, tarball resolution prefers archives whose config.yml namespace matches.
	Namespace *string `json:"namespace,omitempty"`
}

// LoadDockpipeProjectConfig reads dockpipe.config.json from repoRoot. Returns (nil, nil) if the file is missing.
func LoadDockpipeProjectConfig(repoRoot string) (*DockpipeProjectConfig, error) {
	p := filepath.Join(repoRoot, DockpipeProjectConfigFileName)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c DockpipeProjectConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return &c, nil
}

// FindProjectRootWithDockpipeConfig walks up from startDir until it finds a directory
// containing DockpipeProjectConfigFileName. Returns the absolute path to that directory.
// If the file is not found in any parent, returns abs(startDir) so callers can still use
// cwd-based defaults (e.g. compile without a config file).
func FindProjectRootWithDockpipeConfig(startDir string) (string, error) {
	startAbs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for d := startAbs; ; {
		p := filepath.Join(d, DockpipeProjectConfigFileName)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return startAbs, nil
}

// ResolveOpInjectTemplatePath returns the absolute path to the op inject template when secrets.op_inject_template is set.
func ResolveOpInjectTemplatePath(cfg *DockpipeProjectConfig, repoRoot string) (string, bool) {
	if cfg == nil || cfg.Secrets.OpInjectTemplate == nil {
		return "", false
	}
	p := strings.TrimSpace(*cfg.Secrets.OpInjectTemplate)
	if p == "" {
		return "", false
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), true
	}
	return filepath.Join(repoRoot, filepath.Clean(p)), true
}

// DefaultDockpipeProjectConfigBytes returns indented JSON for a new project (dockpipe init).
// Paths are repo-relative; compile skips any that do not exist on disk.
func DefaultDockpipeProjectConfigBytes() ([]byte, error) {
	wf := []string{"workflows"}
	opT := ".env.op.template"
	notes := "1Password op inject mapping (op:// references). Keep vault paths here; do not commit plaintext secrets."
	cfg := DockpipeProjectConfig{
		Schema: 1,
		Compile: DockpipeCompileConfig{
			Workflows: &wf,
		},
		Secrets: DockpipeSecretsConfig{
			OpInjectTemplate: &opT,
			Notes:            &notes,
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}
