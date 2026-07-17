package application

import (
	"fmt"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

func mergeWorkflowContainerConfig(base, override domain.WorkflowContainerConfig) domain.WorkflowContainerConfig {
	if override.IsEmpty() {
		return base
	}
	out := base
	if v := strings.TrimSpace(override.WorkdirHost); v != "" {
		out.WorkdirHost = v
	}
	if v := strings.TrimSpace(override.WorkPath); v != "" {
		out.WorkPath = v
	}
	if len(override.Mounts) > 0 {
		out.Mounts = append(append([]domain.WorkflowContainerMount{}, base.Mounts...), override.Mounts...)
	}
	return out
}

func resolveWorkflowContainerConfig(cfg domain.WorkflowContainerConfig, hostBase, workHost, workPath string, cliMounts []string) (string, string, []string, error) {
	if v := strings.TrimSpace(cfg.WorkdirHost); v != "" {
		workHost = resolveContainerHostPath(hostBase, v)
	}
	if v := strings.TrimSpace(cfg.WorkPath); v != "" {
		workPath = strings.Trim(strings.ReplaceAll(v, "\\", "/"), "/")
	}
	mounts, err := workflowContainerMountSpecs(cfg.Mounts, hostBase)
	if err != nil {
		return "", "", nil, err
	}
	return workHost, workPath, append(mounts, cliMounts...), nil
}

func workflowContainerMountSpecs(mounts []domain.WorkflowContainerMount, hostBase string) ([]string, error) {
	if len(mounts) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		host := resolveContainerHostPath(hostBase, mount.Host)
		guest := strings.TrimSpace(mount.Guest)
		if host == "" || guest == "" {
			return nil, fmt.Errorf("container mounts require both host and guest")
		}
		spec := host + ":" + guest
		if mode := strings.TrimSpace(mount.Mode); mode != "" {
			spec += ":" + mode
		}
		out = append(out, spec)
	}
	return out, nil
}

func resolveContainerHostPath(hostBase, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	hostBase = strings.TrimSpace(hostBase)
	if hostBase == "" {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(hostBase, value))
}
