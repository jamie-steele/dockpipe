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
