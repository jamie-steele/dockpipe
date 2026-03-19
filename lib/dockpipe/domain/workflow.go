// Package domain holds workflow config types and parsing — no I/O.
package domain

import (
	"fmt"

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

// Workflow is templates/<name>/config.yml.
type Workflow struct {
	Name            string            `yaml:"name"`
	Run             RunSpec           `yaml:"run"`
	Isolate         string            `yaml:"isolate"`
	Act             string            `yaml:"act"`
	Action          string            `yaml:"action"`
	Resolver        string            `yaml:"resolver"`
	DefaultResolver string            `yaml:"default_resolver"`
	Vars            map[string]string `yaml:"vars"`
	Steps           []Step            `yaml:"steps"`
}

// Step is one entry under steps:.
type Step struct {
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

// ParseWorkflowYAML unmarshals workflow config from YAML bytes.
func ParseWorkflowYAML(data []byte) (*Workflow, error) {
	var w Workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	return &w, nil
}
