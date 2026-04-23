// Package domain holds workflow config types and parsing — no I/O.
package domain

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Workflow type identifiers (optional YAML workflow_type). The engine does not branch on these;
// they classify workflows for tooling, docs, and future composition. Use lowercase + hyphens.
const (
	WorkflowTypeSecretstore = "secretstore"
	// DefaultOutputsEnvRel is the default step outputs path when outputs: is omitted (forward slashes for YAML).
	DefaultOutputsEnvRel = "bin/.dockpipe/outputs.env"
)

var workflowTypePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// RunSpec is a string or YAML list of strings (e.g. run: script.sh vs run: [a, b]).
type RunSpec []string

func (r *RunSpec) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		var s string
		if err := n.Decode(&s); err != nil {
			return err
		}
		*r = []string{s}
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		var ss []string
		if err := n.Decode(&ss); err != nil {
			return err
		}
		*r = ss
		return nil
	}
	return fmt.Errorf("expected string or sequence for run")
}

// Workflow is templates/<name>/config.yml (bundled or user project) or templates/core/resolvers/<name>/config.yml (delegate).
type Workflow struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	// Category: optional launcher/UI hint (e.g. "app" = show in Pipeon Basic mode as a launchable GUI/tool).
	Category string `yaml:"category,omitempty"`
	// Icon: optional package-owned artwork path for launcher/tooling surfaces. Relative paths resolve
	// next to this workflow config.yml.
	Icon string `yaml:"icon,omitempty"`
	// WorkflowType: optional classifier (e.g. secretstore). Engine ignores; scripts and UIs may use it.
	WorkflowType string `yaml:"workflow_type,omitempty"`
	// Namespace: optional author/org label for packages and tooling (see ValidateNamespace).
	Namespace string `yaml:"namespace,omitempty"`
	// Capability: optional legacy YAML field (ignored by validation).
	Capability string `yaml:"capability,omitempty"`
	// PrimitiveYAMLDeprecated is the deprecated YAML key "primitive" — merged into Capability when parsing.
	PrimitiveYAMLDeprecated string  `yaml:"primitive,omitempty"`
	Run                     RunSpec `yaml:"run,omitempty"`
	Isolate                 string  `yaml:"isolate,omitempty"`
	Act                     string  `yaml:"act,omitempty"`
	Action                  string  `yaml:"action,omitempty"`
	Resolver                string  `yaml:"resolver,omitempty"`
	DefaultResolver         string  `yaml:"default_resolver,omitempty"`
	// Runtime: default runtime profile name (templates/core/runtimes/<name>); preferred over default_resolver for env selection.
	Runtime string `yaml:"runtime,omitempty"`
	// DefaultRuntime: YAML default_runtime — same role as default_resolver when runtime is unset.
	DefaultRuntime string `yaml:"default_runtime,omitempty"`
	// Runtimes: optional explicit allowlist when more than one substrate is allowed (e.g. docker and cli). If omitted, the runner derives an allowlist from runtime / default_runtime when set.
	Runtimes []string `yaml:"runtimes,omitempty"`
	// Strategy: default named strategy (templates/core/strategies/<name> or templates/<wf>/strategies/<name>).
	Strategy string `yaml:"strategy,omitempty"`
	// Strategies: optional allowlist of strategy names; if non-empty, --strategy / workflow.strategy must be listed.
	Strategies []string `yaml:"strategies,omitempty"`
	// Vault names which backend runs secret→env injection for this workflow (see docs/vault.md).
	// "op" = 1Password CLI (op inject). "none" / "off" = skip op inject for this workflow.
	// Omit to use dockpipe.config.json secrets.vault when set, else best-effort inject when the template exists.
	Vault string `yaml:"vault,omitempty"`
	// DockerPreflight: when false, skip EnsureDockerReachable before steps when no step uses the container runner.
	// Use only for workflows where every step is skip_container and host run:/pre_script scripts do not invoke Docker.
	DockerPreflight *bool `yaml:"docker_preflight,omitempty"`
	// CompileHooks: shell commands run by `dockpipe package compile workflow` from the workflow source directory
	// after validation and before the package tarball is written (e.g. go build, code generation).
	CompileHooks []string `yaml:"compile_hooks,omitempty"`
	// Types: optional PipeLang type declarations used by tooling/materialization.
	Types []string `yaml:"types,omitempty"`
	// Inject: explicit compile closure dependencies (workflow/package ids and resolver profile names).
	// Unlike imports:, this does not merge YAML — it only guides package compile for-workflow ordering.
	Inject   WorkflowInjectList     `yaml:"inject,omitempty"`
	Vars     map[string]string      `yaml:"vars,omitempty"`
	Compose  WorkflowComposeConfig  `yaml:"compose,omitempty"`
	Security WorkflowSecurityConfig `yaml:"security,omitempty"`
	Steps    []Step                 `yaml:"steps,omitempty"`
}

type WorkflowComposeConfig struct {
	File             string            `yaml:"file,omitempty"`
	Project          string            `yaml:"project,omitempty"`
	ProjectDirectory string            `yaml:"project_directory,omitempty"`
	AutodownEnv      string            `yaml:"autodown_env,omitempty"`
	Exports          map[string]string `yaml:"exports,omitempty"`
	Services         []string          `yaml:"services,omitempty"`
}

type WorkflowSecurityConfig struct {
	Network WorkflowNetworkConfig `yaml:"network,omitempty"`
}

type WorkflowNetworkConfig struct {
	Mode        string   `yaml:"mode,omitempty"`
	Enforcement string   `yaml:"enforcement,omitempty"`
	Allow       []string `yaml:"allow,omitempty"`
	Block       []string `yaml:"block,omitempty"`
}

// AnyContainerStep reports whether any step runs inside Docker (skip_container is false or omitted).
func (w *Workflow) AnyContainerStep() bool {
	for _, s := range w.Steps {
		if !s.SkipContainer {
			return true
		}
		if hostBuiltinNeedsDocker(strings.TrimSpace(s.HostBuiltin)) {
			return true
		}
	}
	return false
}

// NeedsDockerReachable reports whether we should run EnsureDockerReachable before executing steps.
// True when any step uses the container runner, or when any step has run:/pre_script (host scripts
// may invoke docker directly — e.g. templates/core/resolvers/vscode/config.yml is skip_container-only but runs docker on the host).
// If docker_preflight: false is set on the workflow, this returns false when no step uses the container runner
// (opt-out for host-only workflows; scripts under run: must not require Docker).
func (w *Workflow) NeedsDockerReachable() bool {
	if w.AnyContainerStep() {
		return true
	}
	if w.DockerPreflight != nil && !*w.DockerPreflight {
		return false
	}
	for _, s := range w.Steps {
		if len(s.RunPaths()) > 0 {
			return true
		}
		if strings.TrimSpace(s.Resolver) != "" {
			// May be host-isolate (docker on host) or a template name via resolver file — run preflight.
			return true
		}
	}
	return false
}

// Step is one entry under steps:.
type Step struct {
	// ID is optional; used in logs (e.g. [merge] lines). If empty, runner uses "step N".
	ID            string            `yaml:"id,omitempty"`
	Run           RunSpec           `yaml:"run,omitempty"`
	PreScript     string            `yaml:"pre_script,omitempty"`
	Isolate       string            `yaml:"isolate,omitempty"`
	Act           string            `yaml:"act,omitempty"`
	Action        string            `yaml:"action,omitempty"`
	Cmd           string            `yaml:"cmd,omitempty"`
	Command       string            `yaml:"command,omitempty"`
	Outputs       string            `yaml:"outputs,omitempty"`
	SkipContainer bool              `yaml:"skip_container,omitempty"`
	Vars          map[string]string `yaml:"vars,omitempty"`
	// Blocking is YAML is_blocking: when false, this step joins a parallel batch with adjacent
	// non-blocking steps. Inputs = env after last blocking step + this step’s vars/pre-scripts only;
	// outputs merge in order after the whole batch (see src/lib/README.md).
	Blocking *bool `yaml:"is_blocking,omitempty"`
	// CaptureStdout: host path (relative to DOCKPIPE_WORKDIR or cwd) to tee container stdout into.
	CaptureStdout string `yaml:"capture_stdout,omitempty"`
	// Manifest: host path to write a small JSON manifest after the step (exit_code, duration_ms, id).
	Manifest string `yaml:"manifest,omitempty"`
	// Runtime: optional runtime profile name (templates/core/runtimes/<name>); preferred over resolver for pairing.
	Runtime string `yaml:"runtime,omitempty"`
	// Resolver: legacy alias for the same shared profile (not files next to this workflow).
	// Loads DOCKPIPE_RESOLVER_* / DOCKPIPE_RUNTIME_* when unset on the step.
	Resolver string `yaml:"resolver,omitempty"`
	// WorkflowName: packaged-workflow target name when runtime: package.
	WorkflowName string `yaml:"workflow,omitempty"`
	// Capability: optional legacy per-step field (ignored by the runner).
	Capability string `yaml:"capability,omitempty"`
	// Package: namespace for runtime: package — must match the nested workflow's namespace: (see ResolvePackagedWorkflowConfigPath).
	Package string `yaml:"package,omitempty"`
	// HostBuiltin: optional engine step for skip_container workflows — runs a built-in host action instead of run:/pre_script.
	// Allowed values: package_build_store (same as dockpipe package build store; uses PACKAGE_STORE_OUT, PACKAGE_STORE_ONLY, PACKAGE_STORE_VERSION from merged env).
	HostBuiltin string `yaml:"host_builtin,omitempty"`
}

func (s *Step) UsesPackagedWorkflow() bool {
	return strings.TrimSpace(s.WorkflowName) != "" || strings.TrimSpace(s.Package) != ""
}

// RuntimeProfileName returns per-step isolation profile name (runtime: or resolver:).
func (s *Step) RuntimeProfileName() string {
	r := strings.TrimSpace(s.Runtime)
	if r != "" {
		return r
	}
	return strings.TrimSpace(s.Resolver)
}

// RuntimeProfileConflict is always false — both runtime: and resolver: may be set (merged profiles).
func (s *Step) RuntimeProfileConflict() bool {
	return false
}

// IsBlocking reports whether this step completes before the pipeline advances (default true).
func (s *Step) IsBlocking() bool {
	if s.Blocking == nil {
		return true
	}
	return *s.Blocking
}

// DisplayName returns id if set, otherwise "step <1-based index>".
func (s *Step) DisplayName(index int) string {
	if strings.TrimSpace(s.ID) != "" {
		return strings.TrimSpace(s.ID)
	}
	return fmt.Sprintf("step %d", index+1)
}

func (s *Step) RunPaths() []string {
	var out []string
	out = append(out, s.Run...)
	if s.PreScript != "" {
		out = append(out, s.PreScript)
	}
	return out
}

func (s *Step) ActPath() string {
	if s.Act != "" {
		return s.Act
	}
	return s.Action
}

func (s *Step) CmdLine() string {
	if s.Cmd != "" {
		return s.Cmd
	}
	return s.Command
}

func (s *Step) OutputsPath() string {
	if s.Outputs != "" {
		return s.Outputs
	}
	return DefaultOutputsEnvRel
}

// workflowFile is the on-disk shape: steps may mix plain steps and group wrappers.
type workflowFile struct {
	Name                    string                 `yaml:"name"`
	Description             string                 `yaml:"description,omitempty"`
	Category                string                 `yaml:"category,omitempty"`
	Icon                    string                 `yaml:"icon,omitempty"`
	WorkflowType            string                 `yaml:"workflow_type,omitempty"`
	Namespace               string                 `yaml:"namespace,omitempty"`
	Capability              string                 `yaml:"capability,omitempty"`
	PrimitiveYAMLDeprecated string                 `yaml:"primitive,omitempty"`
	Run                     RunSpec                `yaml:"run"`
	Isolate                 string                 `yaml:"isolate"`
	Act                     string                 `yaml:"act"`
	Action                  string                 `yaml:"action"`
	Resolver                string                 `yaml:"resolver"`
	DefaultResolver         string                 `yaml:"default_resolver"`
	Runtime                 string                 `yaml:"runtime,omitempty"`
	DefaultRuntime          string                 `yaml:"default_runtime,omitempty"`
	Runtimes                []string               `yaml:"runtimes,omitempty"`
	Strategy                string                 `yaml:"strategy,omitempty"`
	Strategies              []string               `yaml:"strategies,omitempty"`
	Vault                   string                 `yaml:"vault,omitempty"`
	DockerPreflight         *bool                  `yaml:"docker_preflight,omitempty"`
	CompileHooks            []string               `yaml:"compile_hooks,omitempty"`
	Types                   []string               `yaml:"types,omitempty"`
	Vars                    map[string]string      `yaml:"vars"`
	Compose                 WorkflowComposeConfig  `yaml:"compose,omitempty"`
	Security                WorkflowSecurityConfig `yaml:"security,omitempty"`
	Imports                 []string               `yaml:"imports,omitempty"`
	Inject                  WorkflowInjectList     `yaml:"inject,omitempty"`
	Steps                   []stepOrGroupYAML      `yaml:"steps"`
}

type stepOrGroupYAML struct {
	group *asyncGroupYAML
	step  *Step
}

type asyncGroupYAML struct {
	Mode  string `yaml:"mode"`
	Tasks []Step `yaml:"tasks"`
}

func (s *stepOrGroupYAML) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode || len(n.Content)%2 != 0 {
		return fmt.Errorf("steps: each entry must be a mapping")
	}
	var keys []string
	for i := 0; i < len(n.Content); i += 2 {
		var k string
		if err := n.Content[i].Decode(&k); err != nil {
			return fmt.Errorf("steps: %w", err)
		}
		keys = append(keys, k)
	}
	hasGroup := false
	for _, k := range keys {
		if k == "group" {
			hasGroup = true
			break
		}
	}
	if hasGroup {
		if len(keys) != 1 {
			return fmt.Errorf("steps: entry with `group` must contain only the `group` key (got %v)", keys)
		}
		var aux struct {
			Group asyncGroupYAML `yaml:"group"`
		}
		if err := n.Decode(&aux); err != nil {
			return fmt.Errorf("steps: group: %w", err)
		}
		if aux.Group.Mode != "async" {
			return fmt.Errorf("steps: group.mode must be \"async\", got %q", aux.Group.Mode)
		}
		if len(aux.Group.Tasks) == 0 {
			return fmt.Errorf("steps: group.tasks must contain at least one task")
		}
		s.group = &aux.Group
		return nil
	}
	st := new(Step)
	if err := n.Decode(st); err != nil {
		return fmt.Errorf("steps: %w", err)
	}
	s.step = st
	return nil
}

func flattenSteps(items []stepOrGroupYAML) ([]Step, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]Step, 0, len(items))
	bFalse := false
	for _, it := range items {
		if it.group != nil {
			for _, t := range it.group.Tasks {
				// Default blocking is true when omitted; inside async group, omission means async.
				if t.Blocking != nil && *t.Blocking {
					id := strings.TrimSpace(t.ID)
					if id == "" {
						return nil, fmt.Errorf("steps: task inside group.mode: async cannot set is_blocking: true")
					}
					return nil, fmt.Errorf("steps: task %q inside group.mode: async cannot set is_blocking: true", id)
				}
				tt := t
				tt.Blocking = &bFalse
				out = append(out, tt)
			}
			continue
		}
		if it.step != nil {
			out = append(out, *it.step)
			continue
		}
		return nil, fmt.Errorf("steps: internal parse error (empty entry)")
	}
	return out, nil
}

// ValidateWorkflowTypeField checks workflow_type when set (lowercase identifier, hyphens allowed).
func ValidateWorkflowTypeField(w *Workflow) error {
	if w == nil {
		return nil
	}
	t := strings.TrimSpace(w.WorkflowType)
	if t == "" {
		return nil
	}
	if !workflowTypePattern.MatchString(t) {
		return fmt.Errorf("workflow_type %q must match %s (e.g. secretstore, my-kind)", t, workflowTypePattern.String())
	}
	return nil
}

// ValidateWorkflowNamespaceField checks namespace when set (reserved words and pattern).
func ValidateWorkflowNamespaceField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateNamespace(w.Namespace)
}

// ValidateWorkflowVaultField checks workflow vault: when set, must be a supported token (see docs/vault.md).
func ValidateWorkflowVaultField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateVaultModeString(w.Vault)
}

// ValidateWorkflowCapabilityField checks capability when set (dotted id; see docs/capabilities.md).
func ValidateWorkflowCapabilityField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateCapabilityID(w.Capability)
}

// ValidateWorkflowPrimitiveField is deprecated: use ValidateWorkflowCapabilityField.
func ValidateWorkflowPrimitiveField(w *Workflow) error {
	return ValidateWorkflowCapabilityField(w)
}

// ValidateWorkflowMustDeclareCapability requires a non-empty capability id on the workflow (see docs/capabilities.md).
func ValidateWorkflowMustDeclareCapability(w *Workflow) error {
	if w == nil {
		return nil
	}
	if strings.TrimSpace(w.Capability) == "" {
		return fmt.Errorf("workflow must declare capability (dotted id, e.g. cli.codex — see docs/capabilities.md)")
	}
	return ValidateCapabilityID(w.Capability)
}

// ValidateWorkflowMustDeclarePrimitive is deprecated: use ValidateWorkflowMustDeclareCapability.
func ValidateWorkflowMustDeclarePrimitive(w *Workflow) error {
	return ValidateWorkflowMustDeclareCapability(w)
}

// ValidateLoadedWorkflow checks workflow fields required for run and dockpipe workflow validate.
func ValidateLoadedWorkflow(w *Workflow) error {
	if w == nil {
		return nil
	}
	if err := ValidateWorkflowTypeField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowNamespaceField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowVaultField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowComposeField(w); err != nil {
		return err
	}
	if err := ValidateWorkflowSecurityField(w); err != nil {
		return err
	}
	for i, s := range w.Steps {
		if err := ValidateCapabilityID(s.Capability); err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}
		if err := ValidateStepPackageInvocation(i, s); err != nil {
			return err
		}
		if err := ValidateStepHostBuiltin(i, s); err != nil {
			return err
		}
		if err := ValidateStepComposeBuiltin(i, s, w); err != nil {
			return err
		}
	}
	return nil
}

func ValidateWorkflowComposeField(w *Workflow) error {
	if w == nil {
		return nil
	}
	c := w.Compose
	if strings.TrimSpace(c.File) == "" &&
		strings.TrimSpace(c.Project) == "" &&
		strings.TrimSpace(c.ProjectDirectory) == "" &&
		len(c.Exports) == 0 &&
		len(c.Services) == 0 {
		return nil
	}
	if strings.TrimSpace(c.File) == "" {
		return fmt.Errorf("compose.file is required when compose settings are present")
	}
	return nil
}

func ValidateWorkflowSecurityField(w *Workflow) error {
	if w == nil {
		return nil
	}
	return ValidateCompiledSecurityPolicy(&CompiledSecurityPolicy{
		Network: CompiledNetworkPolicy{
			Mode:        strings.TrimSpace(w.Security.Network.Mode),
			Enforcement: strings.TrimSpace(w.Security.Network.Enforcement),
			Allow:       append([]string(nil), w.Security.Network.Allow...),
			Block:       append([]string(nil), w.Security.Network.Block...),
		},
	})
}

func ValidateStepPackageInvocation(i int, s Step) error {
	if strings.EqualFold(strings.TrimSpace(s.Runtime), "package") {
		return fmt.Errorf("step %d: runtime: package is no longer supported; use workflow: <name> and package: <namespace>", i+1)
	}
	if !s.UsesPackagedWorkflow() {
		return nil
	}
	if strings.TrimSpace(s.WorkflowName) == "" {
		return fmt.Errorf("step %d: packaged workflow step requires workflow: <name>", i+1)
	}
	if strings.TrimSpace(s.Package) == "" {
		return fmt.Errorf("step %d: packaged workflow step requires package: <namespace>", i+1)
	}
	if strings.TrimSpace(s.Resolver) != "" {
		return fmt.Errorf("step %d: packaged workflow step uses workflow: <name>; do not also set resolver:", i+1)
	}
	if strings.TrimSpace(s.Isolate) != "" {
		return fmt.Errorf("step %d: packaged workflow step uses workflow/package; do not also set isolate:", i+1)
	}
	return nil
}

// ValidateStepHostBuiltin checks host_builtin steps (see Step.HostBuiltin).
func ValidateStepHostBuiltin(i int, s Step) error {
	b := strings.TrimSpace(s.HostBuiltin)
	if b == "" {
		return nil
	}
	if !s.SkipContainer {
		return fmt.Errorf("step %d: host_builtin %q requires skip_container: true", i+1, b)
	}
	if s.UsesPackagedWorkflow() {
		return fmt.Errorf("step %d: host_builtin is incompatible with packaged workflow steps", i+1)
	}
	if len(s.Run) > 0 || s.PreScript != "" {
		return fmt.Errorf("step %d: host_builtin cannot be combined with run: or pre_script", i+1)
	}
	switch b {
	case "package_build_store", "compose_up", "compose_down", "compose_ps":
		return nil
	default:
		return fmt.Errorf("step %d: unknown host_builtin %q (allowed: package_build_store, compose_up, compose_down, compose_ps)", i+1, b)
	}
}

func ValidateStepComposeBuiltin(i int, s Step, w *Workflow) error {
	if !hostBuiltinNeedsCompose(strings.TrimSpace(s.HostBuiltin)) {
		return nil
	}
	if w == nil || strings.TrimSpace(w.Compose.File) == "" {
		return fmt.Errorf("step %d: host_builtin %q requires workflow compose.file", i+1, strings.TrimSpace(s.HostBuiltin))
	}
	return nil
}

func hostBuiltinNeedsDocker(b string) bool {
	return hostBuiltinNeedsCompose(b)
}

func hostBuiltinNeedsCompose(b string) bool {
	switch strings.TrimSpace(b) {
	case "compose_up", "compose_down", "compose_ps":
		return true
	default:
		return false
	}
}

// ParseWorkflowYAML unmarshals workflow config from YAML bytes (no imports).
// If the document contains imports:, use ParseWorkflowFromDisk with a readFile callback (see LoadWorkflow in infrastructure).
func ParseWorkflowYAML(data []byte) (*Workflow, error) {
	return ParseWorkflowFromDisk(data, ".", nil)
}
