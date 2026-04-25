package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	RuntimeManifestKind       = "dockpipe-runtime-manifest"
	ImageArtifactManifestKind = "docker-image-artifact"
	RuntimeManifestDirName    = ".dockpipe"
	RuntimeManifestFileName   = "runtime.effective.json"
	ImageArtifactFileName     = "image-artifact.json"
	StepArtifactsDirName      = "steps"
)

type CompiledRuntimeManifest struct {
	Schema               int                    `json:"schema" yaml:"schema"`
	Kind                 string                 `json:"kind" yaml:"kind"`
	WorkflowName         string                 `json:"workflow_name,omitempty" yaml:"workflow_name,omitempty"`
	PackageName          string                 `json:"package_name,omitempty" yaml:"package_name,omitempty"`
	StepID               string                 `json:"step_id,omitempty" yaml:"step_id,omitempty"`
	RuntimeProfile       string                 `json:"runtime_profile,omitempty" yaml:"runtime_profile,omitempty"`
	ResolverProfile      string                 `json:"resolver_profile,omitempty" yaml:"resolver_profile,omitempty"`
	PolicyProfile        string                 `json:"policy_profile,omitempty" yaml:"policy_profile,omitempty"`
	PolicySources        PolicySources          `json:"policy_sources,omitempty" yaml:"policy_sources,omitempty"`
	PolicyFingerprint    string                 `json:"policy_fingerprint,omitempty" yaml:"policy_fingerprint,omitempty"`
	ImageFingerprint     string                 `json:"image_fingerprint,omitempty" yaml:"image_fingerprint,omitempty"`
	Security             CompiledSecurityPolicy `json:"security" yaml:"security"`
	Image                CompiledImageSelection `json:"image" yaml:"image"`
	EnforcementSummaries []string               `json:"enforcement_summaries,omitempty" yaml:"enforcement_summaries,omitempty"`
	RuleIDs              []string               `json:"rule_ids,omitempty" yaml:"rule_ids,omitempty"`
}

type PolicySources struct {
	EngineDefault    bool   `json:"engine_default,omitempty" yaml:"engine_default,omitempty"`
	RuntimeBaseline  string `json:"runtime_baseline,omitempty" yaml:"runtime_baseline,omitempty"`
	PolicyProfile    string `json:"policy_profile,omitempty" yaml:"policy_profile,omitempty"`
	WorkflowOverride bool   `json:"workflow_override,omitempty" yaml:"workflow_override,omitempty"`
	StepOverride     bool   `json:"step_override,omitempty" yaml:"step_override,omitempty"`
}

type CompiledSecurityPolicy struct {
	Preset  string                   `json:"preset,omitempty" yaml:"preset,omitempty"`
	Network CompiledNetworkPolicy    `json:"network,omitempty" yaml:"network,omitempty"`
	FS      CompiledFilesystemPolicy `json:"filesystem,omitempty" yaml:"filesystem,omitempty"`
	Process CompiledProcessPolicy    `json:"process,omitempty" yaml:"process,omitempty"`
}

type CompiledNetworkPolicy struct {
	Mode        string   `json:"mode,omitempty" yaml:"mode,omitempty"`
	Enforcement string   `json:"enforcement,omitempty" yaml:"enforcement,omitempty"`
	Allow       []string `json:"allow,omitempty" yaml:"allow,omitempty"`
	Block       []string `json:"block,omitempty" yaml:"block,omitempty"`
	InternalDNS bool     `json:"internal_dns,omitempty" yaml:"internal_dns,omitempty"`
}

type CompiledFilesystemPolicy struct {
	Root          string   `json:"root,omitempty" yaml:"root,omitempty"`
	Writes        string   `json:"writes,omitempty" yaml:"writes,omitempty"`
	WritablePaths []string `json:"writable_paths,omitempty" yaml:"writable_paths,omitempty"`
	TempPaths     []string `json:"temp_paths,omitempty" yaml:"temp_paths,omitempty"`
}

type CompiledProcessPolicy struct {
	User            string                 `json:"user,omitempty" yaml:"user,omitempty"`
	NoNewPrivileges bool                   `json:"no_new_privileges,omitempty" yaml:"no_new_privileges,omitempty"`
	DropCaps        []string               `json:"drop_caps,omitempty" yaml:"drop_caps,omitempty"`
	AddCaps         []string               `json:"add_caps,omitempty" yaml:"add_caps,omitempty"`
	PIDLimit        int                    `json:"pid_limit,omitempty" yaml:"pid_limit,omitempty"`
	Resources       CompiledResourceLimits `json:"resources,omitempty" yaml:"resources,omitempty"`
}

type CompiledResourceLimits struct {
	CPU    string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`
}

type CompiledImageSelection struct {
	Source         string                  `json:"source,omitempty" yaml:"source,omitempty"`
	Ref            string                  `json:"ref,omitempty" yaml:"ref,omitempty"`
	AutoBuild      string                  `json:"auto_build,omitempty" yaml:"auto_build,omitempty"`
	PullPolicy     string                  `json:"pull_policy,omitempty" yaml:"pull_policy,omitempty"`
	Build          *CompiledImageBuildSpec `json:"build,omitempty" yaml:"build,omitempty"`
	ExpectedDigest string                  `json:"expected_digest,omitempty" yaml:"expected_digest,omitempty"`
}

type CompiledImageBuildSpec struct {
	Context    string            `json:"context,omitempty" yaml:"context,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Target     string            `json:"target,omitempty" yaml:"target,omitempty"`
	Platform   string            `json:"platform,omitempty" yaml:"platform,omitempty"`
	Args       map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
}

type ImageArtifactManifest struct {
	Schema                      int                     `json:"schema" yaml:"schema"`
	Kind                        string                  `json:"kind" yaml:"kind"`
	WorkflowName                string                  `json:"workflow_name,omitempty" yaml:"workflow_name,omitempty"`
	PackageName                 string                  `json:"package_name,omitempty" yaml:"package_name,omitempty"`
	StepID                      string                  `json:"step_id,omitempty" yaml:"step_id,omitempty"`
	ImageKey                    string                  `json:"image_key,omitempty" yaml:"image_key,omitempty"`
	Source                      string                  `json:"source,omitempty" yaml:"source,omitempty"`
	ArtifactState               string                  `json:"artifact_state,omitempty" yaml:"artifact_state,omitempty"`
	Fingerprint                 string                  `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	SourceFingerprint           string                  `json:"source_fingerprint,omitempty" yaml:"source_fingerprint,omitempty"`
	SecurityManifestFingerprint string                  `json:"security_manifest_fingerprint,omitempty" yaml:"security_manifest_fingerprint,omitempty"`
	ImageRef                    string                  `json:"image_ref,omitempty" yaml:"image_ref,omitempty"`
	ExpectedDigest              string                  `json:"expected_digest,omitempty" yaml:"expected_digest,omitempty"`
	ResolvedRef                 string                  `json:"resolved_ref,omitempty" yaml:"resolved_ref,omitempty"`
	ImageID                     string                  `json:"image_id,omitempty" yaml:"image_id,omitempty"`
	RepoDigest                  string                  `json:"repo_digest,omitempty" yaml:"repo_digest,omitempty"`
	Build                       *CompiledImageBuildSpec `json:"build,omitempty" yaml:"build,omitempty"`
}

var (
	validNetworkModes     = map[string]struct{}{"": {}, "offline": {}, "allowlist": {}, "restricted": {}, "internet": {}}
	validEnforcementModes = map[string]struct{}{"": {}, "native": {}, "proxy": {}, "advisory": {}}
	validFilesystemRoots  = map[string]struct{}{"": {}, "readonly": {}, "writable": {}}
	validFilesystemWrites = map[string]struct{}{"": {}, "workspace-only": {}, "declared": {}}
	validProcessUsers     = map[string]struct{}{"": {}, "auto": {}, "root": {}, "non-root": {}}
	validImageSources     = map[string]struct{}{"": {}, "auto": {}, "build": {}, "registry": {}}
	validAutoBuildModes   = map[string]struct{}{"": {}, "if-missing": {}, "if-stale": {}, "never": {}}
	validPullPolicies     = map[string]struct{}{"": {}, "if-missing": {}, "never": {}}
	validPolicyProfiles   = map[string]struct{}{"": {}, "secure-default": {}, "internet-client": {}, "build-online": {}, "sidecar-client": {}}
	validArtifactStates   = map[string]struct{}{"": {}, "materialized": {}, "referenced": {}}
)

func ValidateCompiledRuntimeManifest(m *CompiledRuntimeManifest) error {
	if m == nil {
		return nil
	}
	if m.Schema <= 0 {
		return fmt.Errorf("schema must be > 0")
	}
	if strings.TrimSpace(m.Kind) == "" {
		m.Kind = RuntimeManifestKind
	}
	if m.Kind != RuntimeManifestKind {
		return fmt.Errorf("kind %q must be %q", m.Kind, RuntimeManifestKind)
	}
	if err := validateEnum("policy_profile", m.PolicyProfile, validPolicyProfiles); err != nil {
		return err
	}
	if err := ValidateCompiledSecurityPolicy(&m.Security); err != nil {
		return err
	}
	if err := ValidateCompiledImageSelection(&m.Image); err != nil {
		return err
	}
	return nil
}

func ValidateCompiledSecurityPolicy(p *CompiledSecurityPolicy) error {
	if p == nil {
		return nil
	}
	if err := validateEnum("preset", p.Preset, validPolicyProfiles); err != nil {
		return err
	}
	if err := validateEnum("network.mode", p.Network.Mode, validNetworkModes); err != nil {
		return err
	}
	if err := validateEnum("network.enforcement", p.Network.Enforcement, validEnforcementModes); err != nil {
		return err
	}
	if p.Network.Mode == "offline" && (len(p.Network.Allow) > 0 || len(p.Network.Block) > 0) {
		return fmt.Errorf("network.mode offline cannot be combined with allow/block rules")
	}
	if p.Network.Mode == "allowlist" && len(p.Network.Allow) == 0 {
		return fmt.Errorf("network.mode allowlist requires at least one allow rule")
	}
	switch p.Network.Mode {
	case "offline", "internet":
		if p.Network.Enforcement != "" && p.Network.Enforcement != "native" {
			return fmt.Errorf("network.mode %s requires native enforcement", p.Network.Mode)
		}
	case "allowlist", "restricted":
		if p.Network.Enforcement == "native" {
			return fmt.Errorf("network.mode %s cannot use native enforcement", p.Network.Mode)
		}
	}
	if err := validateEnum("filesystem.root", p.FS.Root, validFilesystemRoots); err != nil {
		return err
	}
	if err := validateEnum("filesystem.writes", p.FS.Writes, validFilesystemWrites); err != nil {
		return err
	}
	if err := validateEnum("process.user", p.Process.User, validProcessUsers); err != nil {
		return err
	}
	if p.Process.PIDLimit < 0 {
		return fmt.Errorf("process.pid_limit must be >= 0")
	}
	return nil
}

func ValidateCompiledImageSelection(i *CompiledImageSelection) error {
	if i == nil {
		return nil
	}
	if err := validateEnum("image.source", i.Source, validImageSources); err != nil {
		return err
	}
	if err := validateEnum("image.auto_build", i.AutoBuild, validAutoBuildModes); err != nil {
		return err
	}
	if err := validateEnum("image.pull_policy", i.PullPolicy, validPullPolicies); err != nil {
		return err
	}
	switch i.Source {
	case "build":
		if i.Build == nil {
			return fmt.Errorf("image.source build requires build settings")
		}
		if strings.TrimSpace(i.Build.Context) == "" {
			return fmt.Errorf("image.build.context is required for build source")
		}
		if strings.TrimSpace(i.Build.Dockerfile) == "" {
			return fmt.Errorf("image.build.dockerfile is required for build source")
		}
	case "registry":
		if strings.TrimSpace(i.Ref) == "" {
			return fmt.Errorf("image.source registry requires ref")
		}
		if i.AutoBuild != "" {
			return fmt.Errorf("image.source registry cannot use auto_build")
		}
	}
	return nil
}

func ValidateImageArtifactManifest(m *ImageArtifactManifest) error {
	if m == nil {
		return nil
	}
	if m.Schema <= 0 {
		return fmt.Errorf("schema must be > 0")
	}
	if strings.TrimSpace(m.Kind) == "" {
		m.Kind = ImageArtifactManifestKind
	}
	if m.Kind != ImageArtifactManifestKind {
		return fmt.Errorf("kind %q must be %q", m.Kind, ImageArtifactManifestKind)
	}
	if err := validateEnum("source", m.Source, validImageSources); err != nil {
		return err
	}
	if err := validateEnum("artifact_state", m.ArtifactState, validArtifactStates); err != nil {
		return err
	}
	if m.Source == "build" && m.Build == nil {
		return fmt.Errorf("build source requires build settings")
	}
	if m.Source == "registry" && strings.TrimSpace(m.ImageRef) == "" {
		return fmt.Errorf("registry source requires image_ref")
	}
	if strings.TrimSpace(m.Fingerprint) == "" {
		return fmt.Errorf("fingerprint is required")
	}
	return nil
}

func validateEnum(field, value string, allowed map[string]struct{}) error {
	value = strings.TrimSpace(value)
	if _, ok := allowed[value]; ok {
		return nil
	}
	return fmt.Errorf("%s %q is invalid", field, value)
}

func FingerprintJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func RuntimeManifestPathForStep(stepID string) string {
	return StepArtifactsDirName + "/" + sanitizeStepArtifactID(stepID) + ".runtime.effective.json"
}

func ImageArtifactPathForStep(stepID string) string {
	return StepArtifactsDirName + "/" + sanitizeStepArtifactID(stepID) + ".image-artifact.json"
}

func sanitizeStepArtifactID(stepID string) string {
	stepID = strings.TrimSpace(stepID)
	if stepID == "" {
		return "step"
	}
	var out []rune
	lastDash := false
	for _, r := range stepID {
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_'
		if ok {
			out = append(out, r)
			lastDash = false
			continue
		}
		if !lastDash {
			out = append(out, '-')
			lastDash = true
		}
	}
	s := strings.Trim(strings.ToLower(string(out)), "-")
	if s == "" {
		return "step"
	}
	return s
}
