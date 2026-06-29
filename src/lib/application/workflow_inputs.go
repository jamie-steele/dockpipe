package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"

	"gopkg.in/yaml.v3"
)

type typedWorkflowInputField struct {
	Path         string
	EnvName      string
	DefaultValue string
	Description  string
}

func resolveWorkflowInputsEnv(wf *domain.Workflow, wfConfig, projectRoot string, env map[string]string) (map[string]string, error) {
	if wf == nil || len(wf.Inputs) == 0 {
		return nil, nil
	}
	fields, err := workflowTypedInputFields(wf, wfConfig, projectRoot, nil)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(wf.Inputs))
	for key, binding := range wf.Inputs {
		field, ok := fields[strings.TrimSpace(key)]
		if !ok {
			return nil, fmt.Errorf("workflow input %q does not match any typed field from workflow types", key)
		}
		out[field.EnvName] = resolveInputBinding(binding, env, field.DefaultValue)
	}
	return out, nil
}

func resolveStepInputsEnv(wf *domain.Workflow, wfConfig, projectRoot string, step domain.Step, env map[string]string) (map[string]string, error) {
	if len(step.Inputs) == 0 {
		return nil, nil
	}
	fields, err := workflowTypedInputFields(wf, wfConfig, projectRoot, &step)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(step.Inputs))
	for key, binding := range step.Inputs {
		field, ok := fields[strings.TrimSpace(key)]
		if !ok {
			return nil, fmt.Errorf("step input %q does not match any typed field from workflow types", key)
		}
		out[field.EnvName] = resolveInputBinding(binding, env, field.DefaultValue)
	}
	return out, nil
}

func resolveInputBinding(binding domain.InputBinding, env map[string]string, fallback string) string {
	if from := strings.TrimSpace(binding.From); from != "" {
		if env != nil {
			if v, ok := env[from]; ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
		if strings.TrimSpace(binding.Value) != "" {
			return binding.Value
		}
		return fallback
	}
	if strings.TrimSpace(binding.Value) != "" {
		return binding.Value
	}
	return fallback
}

func workflowTypedInputFields(wf *domain.Workflow, wfConfig, projectRoot string, step *domain.Step) (map[string]typedWorkflowInputField, error) {
	if wf == nil {
		return nil, nil
	}
	configPath, err := workflowTypedInputsConfigPath(wfConfig, projectRoot)
	if err != nil {
		return nil, err
	}
	records := buildCatalogWorkflowInputsForStepWithProjectRoot(configPath, projectRoot, wf, step)
	if len(records) == 0 {
		return nil, workflowTypedInputsMissingError(wf, step)
	}
	out := map[string]typedWorkflowInputField{}
	flattenTypedWorkflowInputRecords("", records, out)
	return out, nil
}

func workflowTypedInputsConfigPath(wfConfig, projectRoot string) (string, error) {
	cfgPath := strings.TrimSpace(wfConfig)
	if cfgPath == "" {
		return "", fmt.Errorf("workflow uses inputs: config path is empty")
	}
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		projectRoot = mustGetwd()
	}
	return infrastructure.WorkflowConfigOnDiskPath(projectRoot, cfgPath)
}

func workflowTypedInputsMissingError(wf *domain.Workflow, step *domain.Step) error {
	if wf == nil {
		return fmt.Errorf("workflow uses inputs: but no typed workflow inputs could be resolved")
	}
	if len(wf.Types) > 0 {
		return fmt.Errorf("workflow uses inputs but no typed workflow inputs could be resolved from types")
	}
	resolverName := strings.TrimSpace(wf.Resolver)
	runtimeName := strings.TrimSpace(wf.Runtime)
	if step != nil {
		if v := strings.TrimSpace(step.Resolver); v != "" {
			resolverName = v
		}
		if v := strings.TrimSpace(step.Runtime); v != "" {
			runtimeName = v
		}
	}
	if resolverName != "" || runtimeName != "" {
		return fmt.Errorf("workflow uses inputs: but no typed workflow inputs could be resolved from runtime=%q resolver=%q", runtimeName, resolverName)
	}
	return fmt.Errorf("workflow uses inputs: but has neither explicit types: nor a typed runtime/resolver config")
}

func catalogWorkflowTypeEntries(cfgPath, projectRoot string, wf *domain.Workflow, step *domain.Step) ([]catalogWorkflowTypeEntry, error) {
	if wf == nil {
		return nil, nil
	}
	moduleRoot := filepath.Dir(cfgPath)
	if len(wf.Types) > 0 {
		out := make([]catalogWorkflowTypeEntry, 0, len(wf.Types))
		for _, spec := range wf.Types {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			out = append(out, catalogWorkflowTypeEntry{Spec: spec, ModuleRoot: moduleRoot})
		}
		return out, nil
	}

	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		var err error
		projectRoot, err = domain.FindProjectRootWithDockpipeConfig(moduleRoot)
		if err != nil {
			return nil, err
		}
	}
	runtimeName := strings.TrimSpace(wf.Runtime)
	resolverName := strings.TrimSpace(wf.Resolver)
	if step != nil {
		if v := strings.TrimSpace(step.Runtime); v != "" {
			runtimeName = v
		}
		if v := strings.TrimSpace(step.Resolver); v != "" {
			resolverName = v
		}
	}

	var out []catalogWorkflowTypeEntry
	if runtimeName != "" {
		entries, err := typedEntriesFromRuntimeConfig(projectRoot, runtimeName)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	if resolverName != "" {
		entries, err := typedEntriesFromResolverConfig(projectRoot, resolverName)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	return dedupeCatalogWorkflowTypeEntries(out), nil
}

func typedEntriesFromResolverConfig(projectRoot, resolverName string) ([]catalogWorkflowTypeEntry, error) {
	profilePath, err := infrastructure.ResolveResolverFilePath(projectRoot, resolverName)
	if err != nil {
		return nil, nil
	}
	return typedEntriesFromAdjacentConfig(profilePath)
}

func typedEntriesFromRuntimeConfig(projectRoot, runtimeName string) ([]catalogWorkflowTypeEntry, error) {
	profilePath, err := infrastructure.ResolveRuntimeFilePath(projectRoot, runtimeName)
	if err != nil {
		return nil, nil
	}
	return typedEntriesFromAdjacentConfig(profilePath)
}

func typedEntriesFromAdjacentConfig(profilePath string) ([]catalogWorkflowTypeEntry, error) {
	dir := filepath.Dir(profilePath)
	for _, base := range []string{"types.yml", "config.yml"} {
		configPath := filepath.Join(dir, base)
		b, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		var doc workflowTypeMapDoc
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", configPath, err)
		}
		moduleRoot := filepath.Dir(configPath)
		out := make([]catalogWorkflowTypeEntry, 0, len(doc.Types))
		for _, spec := range doc.Types {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			out = append(out, catalogWorkflowTypeEntry{Spec: spec, ModuleRoot: moduleRoot})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, nil
}

func dedupeCatalogWorkflowTypeEntries(in []catalogWorkflowTypeEntry) []catalogWorkflowTypeEntry {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]catalogWorkflowTypeEntry, 0, len(in))
	for _, entry := range in {
		spec := strings.TrimSpace(entry.Spec)
		root := strings.TrimSpace(entry.ModuleRoot)
		if spec == "" || root == "" {
			continue
		}
		key := root + "\x00" + spec
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, catalogWorkflowTypeEntry{Spec: spec, ModuleRoot: root})
	}
	return out
}

func flattenTypedWorkflowInputRecords(prefix string, records []catalogWorkflowInputRecord, out map[string]typedWorkflowInputField) {
	for _, record := range records {
		name := strings.TrimSpace(record.FieldName)
		if name == "" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if envName := strings.TrimSpace(record.EnvName); envName != "" {
			out[path] = typedWorkflowInputField{
				Path:         path,
				EnvName:      envName,
				DefaultValue: record.DefaultValue,
				Description:  record.Description,
			}
		}
		if len(record.Children) > 0 {
			flattenTypedWorkflowInputRecords(path, record.Children, out)
		}
	}
}
