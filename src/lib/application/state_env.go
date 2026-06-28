package application

import (
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func preferredStateScope(parts ...string) string {
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			return part
		}
	}
	return "default"
}

func workflowStateScopeHint(opts *CliOpts, wfRoot string, wf *domain.Workflow, rtName, rsName string) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Workflow); s != "" {
			return s
		}
		if s := strings.TrimSpace(opts.WorkflowFile); s != "" {
			base := filepath.Base(filepath.Dir(s))
			if base == "." || base == string(filepath.Separator) {
				base = filepath.Base(s)
			}
			if base != "" && base != "." {
				return base
			}
		}
	}
	wfName := ""
	if wf != nil {
		wfName = strings.TrimSpace(wf.Name)
	}
	return preferredStateScope(filepath.Base(wfRoot), wfName, rsName, rtName)
}

func applyDockpipeStateEnv(envMap map[string]string, workdir, scope string) error {
	stateDir, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return err
	}
	scope = infrastructure.SanitizePackageStateScope(scope)
	packageStateDir, err := infrastructure.PackageStateDir(workdir, scope)
	if err != nil {
		return err
	}
	envMap[infrastructure.EnvStateDir] = stateDir
	envMap[infrastructure.EnvPackageID] = scope
	envMap[infrastructure.EnvPackageStateDir] = packageStateDir
	return nil
}

func applyCIArtifactEnv(envMap map[string]string, workdir string) error {
	if strings.TrimSpace(envMap["DOCKPIPE_CI_RAW_DIR"]) != "" && strings.TrimSpace(envMap["DOCKPIPE_CI_ANALYSIS_DIR"]) != "" {
		return nil
	}
	rawDir, analysisDir, err := ciArtifactDirs(workdir, strings.TrimSpace(envMap["DOCKPIPE_WORKFLOW_NAME"]))
	if err != nil {
		return err
	}
	if strings.TrimSpace(envMap["DOCKPIPE_CI_RAW_DIR"]) == "" {
		envMap["DOCKPIPE_CI_RAW_DIR"] = rawDir
	}
	if strings.TrimSpace(envMap["DOCKPIPE_CI_ANALYSIS_DIR"]) == "" {
		envMap["DOCKPIPE_CI_ANALYSIS_DIR"] = analysisDir
	}
	return nil
}

func applyWorkflowArtifactEnv(envMap map[string]string, workdir, workflowName string) error {
	sourceRoot := strings.TrimSpace(workdir)
	if sourceRoot == "" {
		return nil
	}
	sourceRoot, err := filepath.Abs(filepath.Clean(sourceRoot))
	if err != nil {
		return err
	}
	artifactRoot := strings.TrimSpace(envMap["DOCKPIPE_ARTIFACT_ROOT"])
	if artifactRoot == "" {
		artifactRoot, err = workflowArtifactRoot(sourceRoot, workflowName)
		if err != nil {
			return err
		}
		envMap["DOCKPIPE_ARTIFACT_ROOT"] = artifactRoot
	}
	if strings.TrimSpace(envMap["DOCKPIPE_SOURCE_ROOT"]) == "" {
		envMap["DOCKPIPE_SOURCE_ROOT"] = sourceRoot
	}
	return nil
}

func workflowArtifactRoot(workdir, workflowName string) (string, error) {
	stateDir, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return "", err
	}
	scope := sanitizeWorkflowStateScope(workflowName)
	return filepath.Join(stateDir, "workflows", scope, "artifacts"), nil
}

func ciArtifactDirs(workdir, workflowName string) (string, string, error) {
	stateDir, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(workflowName) != "" {
		scope := sanitizeWorkflowStateScope(workflowName)
		root := filepath.Join(stateDir, "workflows", scope, "dorkpipe")
		return filepath.Join(root, "ci-raw"), filepath.Join(root, "ci-analysis"), nil
	}
	root, err := infrastructure.PackageStateDir(workdir, "dorkpipe")
	if err != nil {
		return "", "", err
	}
	return filepath.Join(root, "ci", "raw"), filepath.Join(root, "ci", "analysis"), nil
}

func sanitizeWorkflowStateScope(scope string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(scope) {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}
