package infrastructure

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

//go:embed schema/workflow.schema.json
var workflowSchemaJSON string

func workflowFileAbsUnderRoot(root, userRel string) (abs string, ok bool) {
	root = filepath.Clean(root)
	userRel = filepath.ToSlash(strings.TrimSpace(userRel))
	if userRel == "" || filepath.IsAbs(userRel) {
		return "", false
	}
	p := filepath.Clean(filepath.Join(root, filepath.FromSlash(userRel)))
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return p, true
}

func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// resolveWorkflowYAMLTarget resolves a user-supplied path to an absolute file path.
// Relative paths: try the current working directory first, then the dockpipe repo root
// (DOCKPIPE_REPO_ROOT or materialized bundle) when the file exists there — so e.g.
// workflows/foo/config.yml works from a subdirectory of the project.
func resolveWorkflowYAMLTarget(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(userPath) {
		return filepath.Abs(filepath.Clean(userPath))
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	c1, err := filepath.Abs(filepath.Clean(filepath.Join(wd, userPath)))
	if err != nil {
		return "", err
	}
	if isRegularFile(c1) {
		return c1, nil
	}
	rr, rerr := RepoRoot()
	if rerr != nil {
		return c1, nil
	}
	c2, ok := workflowFileAbsUnderRoot(rr, userPath)
	if !ok {
		return c1, nil
	}
	if c2 != c1 && isRegularFile(c2) {
		return c2, nil
	}
	return c1, nil
}

// defaultWorkflowValidateConfigPath returns the only workflows/*/config.yml under the
// workflows root when exactly one exists; otherwise a descriptive error.
func defaultWorkflowValidateConfigPath() (string, error) {
	rr, err := RepoRoot()
	if err != nil {
		return "", err
	}
	wfRoot := WorkflowsRootDir(rr)
	matches, err := filepath.Glob(filepath.Join(wfRoot, "*", "config.yml"))
	if err != nil {
		return "", err
	}
	var files []string
	for _, m := range matches {
		if isRegularFile(m) {
			files = append(files, m)
		}
	}
	sort.Strings(files)
	switch len(files) {
	case 0:
		return "", fmt.Errorf("no workflow config.yml found under %q; pass a path (e.g. workflows/mywf/config.yml)", wfRoot)
	case 1:
		return filepath.Abs(files[0])
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "multiple workflow configs under %q; pass one path:\n", wfRoot)
		for _, f := range files {
			if rel, err := filepath.Rel(rr, f); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
				fmt.Fprintf(&b, "  %s\n", filepath.ToSlash(rel))
			} else {
				fmt.Fprintf(&b, "  %s\n", f)
			}
		}
		return "", fmt.Errorf("%s", strings.TrimSuffix(b.String(), "\n"))
	}
}

// ResolveWorkflowYAMLPath resolves the path argument for workflow validation (relative to cwd or repo root, or empty for single-workflow default).
func ResolveWorkflowYAMLPath(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return defaultWorkflowValidateConfigPath()
	}
	return resolveWorkflowYAMLTarget(userPath)
}

// ValidateResolvedWorkflowYAML parses and validates a workflow file given an absolute path (YAML + JSON Schema).
func ValidateResolvedWorkflowYAML(absPath string) error {
	var data []byte
	var err error
	if strings.HasPrefix(absPath, "tar://") {
		tarPath, entry, ok := SplitTarWorkflowURI(absPath)
		if !ok {
			return fmt.Errorf("invalid tar workflow URI")
		}
		data, err = packagebuild.ReadFileFromTarGz(tarPath, entry)
	} else {
		data, err = os.ReadFile(absPath)
	}
	if err != nil {
		return err
	}
	wf, err := LoadWorkflow(absPath)
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	if err := domain.ValidateLoadedWorkflow(wf); err != nil {
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

// ValidateWorkflowYAML parses and validates a workflow file (YAML structure + JSON Schema).
// userPath may be empty to validate the sole workflows/*/config.yml under the repo workflows root when unambiguous.
func ValidateWorkflowYAML(userPath string) error {
	abs, err := ResolveWorkflowYAMLPath(userPath)
	if err != nil {
		return err
	}
	return ValidateResolvedWorkflowYAML(abs)
}
