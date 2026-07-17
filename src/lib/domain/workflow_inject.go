package domain

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowInjectEntry names an explicit compile-time dependency (workflow package or resolver profile).
// A bare string in YAML is shorthand for workflow: <name>.
type WorkflowInjectEntry struct {
	Workflow string `yaml:"workflow,omitempty"`
	Resolver string `yaml:"resolver,omitempty"`
	Package  string `yaml:"package,omitempty"`
}

// WorkflowManifestName returns the workflow/package id to resolve under compile.workflows, or "" if none.
func (e WorkflowInjectEntry) WorkflowManifestName() string {
	w := strings.TrimSpace(e.Workflow)
	p := strings.TrimSpace(e.Package)
	if w != "" {
		return w
	}
	return p
}

// WorkflowInjectList is the YAML inject: sequence — explicit dependency declarations for compile closure
// (see closureWorkflowOrderAndResolvers). This is not file inclusion; use imports: for merging YAML files.
type WorkflowInjectList []WorkflowInjectEntry

func (w *WorkflowInjectList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("inject: must be a sequence")
	}
	for _, item := range n.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			var s string
			if err := item.Decode(&s); err != nil {
				return err
			}
			s = strings.TrimSpace(s)
			if s != "" {
				*w = append(*w, WorkflowInjectEntry{Workflow: s})
			}
		case yaml.MappingNode:
			var e WorkflowInjectEntry
			if err := item.Decode(&e); err != nil {
				return fmt.Errorf("inject: %w", err)
			}
			*w = append(*w, e)
		default:
			return fmt.Errorf("inject: each entry must be a string or mapping")
		}
	}
	return nil
}
