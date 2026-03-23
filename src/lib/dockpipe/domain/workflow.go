// Package domain holds workflow config types and parsing — no I/O.
package domain

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

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
	Name            string  `yaml:"name"`
	Description     string  `yaml:"description,omitempty"`
	// Category: optional launcher/UI hint (e.g. "app" = show in Pipeon Basic mode as a launchable GUI/tool).
	Category string `yaml:"category,omitempty"`
	Run             RunSpec `yaml:"run"`
	Isolate         string  `yaml:"isolate"`
	Act             string  `yaml:"act"`
	Action          string  `yaml:"action"`
	Resolver        string  `yaml:"resolver"`
	DefaultResolver string  `yaml:"default_resolver"`
	// Runtime: default runtime profile name (templates/core/runtimes/<name>); preferred over default_resolver for env selection.
	Runtime string `yaml:"runtime,omitempty"`
	// DefaultRuntime: YAML default_runtime — same role as default_resolver when runtime is unset.
	DefaultRuntime string `yaml:"default_runtime,omitempty"`
	// Runtimes: optional allowlist; if non-empty, the effective runtime must be listed.
	Runtimes []string `yaml:"runtimes,omitempty"`
	// Strategy: default named strategy (templates/core/strategies/<name> or templates/<wf>/strategies/<name>).
	Strategy string `yaml:"strategy,omitempty"`
	// Strategies: optional allowlist of strategy names; if non-empty, --strategy / workflow.strategy must be listed.
	Strategies []string `yaml:"strategies,omitempty"`
	// DockerPreflight: when false, skip EnsureDockerReachable before steps when no step uses the container runner.
	// Use only for workflows where every step is skip_container and host run:/pre_script scripts do not invoke Docker.
	DockerPreflight *bool             `yaml:"docker_preflight,omitempty"`
	Vars            map[string]string `yaml:"vars"`
	Steps           []Step            `yaml:"steps"`
}

// AnyContainerStep reports whether any step runs inside Docker (skip_container is false or omitted).
func (w *Workflow) AnyContainerStep() bool {
	for _, s := range w.Steps {
		if !s.SkipContainer {
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
	Run           RunSpec           `yaml:"run"`
	PreScript     string            `yaml:"pre_script"`
	Isolate       string            `yaml:"isolate"`
	Act           string            `yaml:"act"`
	Action        string            `yaml:"action"`
	Cmd           string            `yaml:"cmd"`
	Command       string            `yaml:"command"`
	Outputs       string            `yaml:"outputs"`
	SkipContainer bool              `yaml:"skip_container"`
	Vars          map[string]string `yaml:"vars"`
	// Blocking is YAML is_blocking: when false, this step joins a parallel batch with adjacent
	// non-blocking steps. Inputs = env after last blocking step + this step’s vars/pre-scripts only;
	// outputs merge in order after the whole batch (see src/lib/dockpipe/README.md).
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
	return ".dockpipe/outputs.env"
}

// workflowFile is the on-disk shape: steps may mix plain steps and group wrappers.
type workflowFile struct {
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description,omitempty"`
	Run             RunSpec           `yaml:"run"`
	Isolate         string            `yaml:"isolate"`
	Act             string            `yaml:"act"`
	Action          string            `yaml:"action"`
	Resolver        string            `yaml:"resolver"`
	DefaultResolver string            `yaml:"default_resolver"`
	Runtime         string            `yaml:"runtime,omitempty"`
	DefaultRuntime  string            `yaml:"default_runtime,omitempty"`
	Runtimes        []string          `yaml:"runtimes,omitempty"`
	Strategy        string            `yaml:"strategy,omitempty"`
	Strategies      []string          `yaml:"strategies,omitempty"`
	DockerPreflight *bool             `yaml:"docker_preflight,omitempty"`
	Vars            map[string]string `yaml:"vars"`
	Imports         []string          `yaml:"imports,omitempty"`
	Steps           []stepOrGroupYAML `yaml:"steps"`
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

// ParseWorkflowYAML unmarshals workflow config from YAML bytes (no imports).
// If the document contains imports:, use ParseWorkflowFromDisk with a readFile callback (see LoadWorkflow in infrastructure).
func ParseWorkflowYAML(data []byte) (*Workflow, error) {
	return ParseWorkflowFromDisk(data, ".", nil)
}
