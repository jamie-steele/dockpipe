package domain

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxImportDepth = 32

// ParseWorkflowFromDisk parses workflow YAML, resolving imports: relative to baseDir using readFile.
// For documents without imports, readFile may be nil. When imports are present, readFile must read by host path.
func ParseWorkflowFromDisk(data []byte, baseDir string, readFile func(string) ([]byte, error)) (*Workflow, error) {
	f, err := parseWorkflowFileRecursive(data, baseDir, readFile, nil, 0)
	if err != nil {
		return nil, err
	}
	steps, err := flattenSteps(f.Steps)
	if err != nil {
		return nil, err
	}
	finallySteps, err := flattenSteps(f.Finally)
	if err != nil {
		return nil, err
	}
	return &Workflow{
		Name:            f.Name,
		Description:     f.Description,
		Category:        f.Category,
		Icon:            f.Icon,
		WorkflowType:    f.WorkflowType,
		Namespace:       f.Namespace,
		Run:             f.Run,
		Isolate:         f.Isolate,
		Act:             f.Act,
		Action:          f.Action,
		Resolver:        f.Resolver,
		Runtime:         f.Runtime,
		Strategy:        f.Strategy,
		Strategies:      f.Strategies,
		Vault:           f.Vault,
		DockerPreflight: f.DockerPreflight,
		CompileHooks:    f.CompileHooks,
		Types:           f.Types,
		View:            f.View,
		ModelPolicy:     f.ModelPolicy,
		Image:           f.Image,
		Container:       f.Container,
		Inject:          f.Inject,
		Inputs:          f.Inputs,
		Vars:            f.Vars,
		Compose:         f.Compose,
		Security:        f.Security,
		Steps:           steps,
		Finally:         finallySteps,
	}, nil
}

func parseWorkflowFileRecursive(data []byte, baseDir string, readFile func(string) ([]byte, error), stack []string, depth int) (*workflowFile, error) {
	if depth > maxImportDepth {
		return nil, fmt.Errorf("imports: max depth %d exceeded", maxImportDepth)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	var doc *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		doc = root.Content[0]
	} else {
		doc = &root
	}
	if err := rejectBannedWorkflowKeys(doc); err != nil {
		return nil, err
	}
	var f workflowFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if len(f.Imports) == 0 {
		return &f, nil
	}
	if readFile == nil {
		return nil, fmt.Errorf("workflow has imports: load from a file (e.g. dockpipe --workflow-file ./workflows/foo/config.yml)")
	}
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "."
	}
	mergedVars := map[string]string{}
	mergedInputs := map[string]InputBinding{}
	var mergedInject []WorkflowInjectEntry
	var stepParts []stepOrGroupYAML
	var finallyParts []stepOrGroupYAML
	for _, imp := range f.Imports {
		imp = strings.TrimSpace(imp)
		if imp == "" {
			continue
		}
		ip := filepath.Join(baseDir, filepath.Clean(imp))
		ap, err := filepath.Abs(ip)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", imp, err)
		}
		for _, s := range stack {
			if s == ap {
				return nil, fmt.Errorf("imports: circular import %s", ap)
			}
		}
		b, err := readFile(ip)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", imp, err)
		}
		sub, err := parseWorkflowFileRecursive(b, filepath.Dir(ip), readFile, append(stack, ap), depth+1)
		if err != nil {
			return nil, err
		}
		if sub.Vars != nil {
			for k, v := range sub.Vars {
				mergedVars[k] = v
			}
		}
		if sub.Inputs != nil {
			for k, v := range sub.Inputs {
				mergedInputs[k] = v
			}
		}
		mergedInject = append(mergedInject, sub.Inject...)
		stepParts = append(stepParts, sub.Steps...)
		finallyParts = append(finallyParts, sub.Finally...)
	}
	for k, v := range f.Inputs {
		mergedInputs[k] = v
	}
	for k, v := range f.Vars {
		mergedVars[k] = v
	}
	out := f
	out.Inputs = mergedInputs
	out.Vars = mergedVars
	out.Inject = append(mergedInject, f.Inject...)
	out.Steps = append(stepParts, f.Steps...)
	// Run importer-local cleanup before imported/base cleanup.
	out.Finally = append(f.Finally, finallyParts...)
	out.Imports = nil
	return &out, nil
}
