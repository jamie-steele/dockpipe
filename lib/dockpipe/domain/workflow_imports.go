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
	return &Workflow{
		Name:            f.Name,
		Description:     f.Description,
		Run:             f.Run,
		Isolate:         f.Isolate,
		Act:             f.Act,
		Action:          f.Action,
		Resolver:        f.Resolver,
		DefaultResolver: f.DefaultResolver,
		Runtime:         f.Runtime,
		DefaultRuntime:  f.DefaultRuntime,
		Runtimes:        f.Runtimes,
		Strategy:        f.Strategy,
		Strategies:      f.Strategies,
		Vars:            f.Vars,
		Steps:           steps,
	}, nil
}

func parseWorkflowFileRecursive(data []byte, baseDir string, readFile func(string) ([]byte, error), stack []string, depth int) (*workflowFile, error) {
	if depth > maxImportDepth {
		return nil, fmt.Errorf("imports: max depth %d exceeded", maxImportDepth)
	}
	var f workflowFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if len(f.Imports) == 0 {
		return &f, nil
	}
	if readFile == nil {
		return nil, fmt.Errorf("workflow has imports: load from a file (e.g. dockpipe --workflow-file ./dockpipe.yml)")
	}
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "."
	}
	mergedVars := map[string]string{}
	var stepParts []stepOrGroupYAML
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
		stepParts = append(stepParts, sub.Steps...)
	}
	for k, v := range f.Vars {
		mergedVars[k] = v
	}
	out := f
	out.Vars = mergedVars
	out.Steps = append(stepParts, f.Steps...)
	out.Imports = nil
	return &out, nil
}
