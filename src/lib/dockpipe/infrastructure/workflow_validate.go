package infrastructure

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"

	"dockpipe/src/lib/dockpipe/domain"
)

//go:embed schema/workflow.schema.json
var workflowSchemaJSON string

// ValidateWorkflowYAML parses and validates a workflow file (YAML structure + JSON Schema).
func ValidateWorkflowYAML(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	var target string
	if filepath.IsAbs(path) {
		target = path
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		target = filepath.Join(wd, path)
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	wf, err := LoadWorkflow(target)
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	if err := domain.ValidateWorkflowTypeField(wf); err != nil {
		return err
	}
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("workflow.schema.json", strings.NewReader(workflowSchemaJSON)); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	sch, err := compiler.Compile("workflow.schema.json")
	if err != nil {
		return fmt.Errorf("schema compile: %w", err)
	}
	if err := sch.Validate(raw); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}
