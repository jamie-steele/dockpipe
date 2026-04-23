package application

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func applyCompiledRuntimePolicy(runOpts *infrastructure.RunOpts, wfConfig, wfRoot string) (string, error) {
	if runOpts == nil {
		return "", nil
	}
	rm, err := loadCompiledRuntimeManifestForWorkflow(wfConfig, wfRoot)
	if err != nil {
		return "", err
	}
	if rm == nil {
		return "", nil
	}
	applyCompiledRuntimeManifest(runOpts, rm)
	return summarizeCompiledRuntimeManifest(rm), nil
}

func applyCompiledRuntimeManifest(runOpts *infrastructure.RunOpts, rm *domain.CompiledRuntimeManifest) {
	if runOpts == nil || rm == nil {
		return
	}
	applyCompiledNetworkPolicy(runOpts, rm.Security.Network)
	applyCompiledFilesystemPolicy(runOpts, rm.Security.FS)
	applyCompiledProcessPolicy(runOpts, rm.Security.Process)
}

func applyCompiledNetworkPolicy(runOpts *infrastructure.RunOpts, policy domain.CompiledNetworkPolicy) {
	if strings.TrimSpace(policy.Mode) == "offline" {
		runOpts.NetworkMode = "none"
	}
}

func applyCompiledFilesystemPolicy(runOpts *infrastructure.RunOpts, policy domain.CompiledFilesystemPolicy) {
	if strings.TrimSpace(policy.Root) == "readonly" {
		runOpts.ReadOnlyRootFS = true
	}
	for _, p := range append([]string{}, policy.TempPaths...) {
		if tmpfsPathForPolicy(p) == "" {
			continue
		}
		runOpts.TmpfsPaths = appendIfMissing(runOpts.TmpfsPaths, tmpfsPathForPolicy(p))
	}
	for _, p := range policy.WritablePaths {
		if tmpfsPath := tmpfsPathForPolicy(p); tmpfsPath != "" {
			runOpts.TmpfsPaths = appendIfMissing(runOpts.TmpfsPaths, tmpfsPath)
		}
	}
}

func applyCompiledProcessPolicy(runOpts *infrastructure.RunOpts, policy domain.CompiledProcessPolicy) {
	switch strings.TrimSpace(policy.User) {
	case "root":
		runOpts.ContainerUser = "0:0"
	}
	if policy.NoNewPrivileges {
		runOpts.SecurityOpt = appendIfMissing(runOpts.SecurityOpt, "no-new-privileges")
	}
	for _, c := range policy.DropCaps {
		c = strings.TrimSpace(c)
		if c != "" {
			runOpts.CapDrop = appendIfMissing(runOpts.CapDrop, c)
		}
	}
	for _, c := range policy.AddCaps {
		c = strings.TrimSpace(c)
		if c != "" {
			runOpts.CapAdd = appendIfMissing(runOpts.CapAdd, c)
		}
	}
	if policy.PIDLimit > 0 {
		runOpts.PIDLimit = policy.PIDLimit
	}
	if cpu := strings.TrimSpace(policy.Resources.CPU); cpu != "" {
		runOpts.CPULimit = cpu
	}
	if mem := strings.TrimSpace(policy.Resources.Memory); mem != "" {
		runOpts.MemoryLimit = mem
	}
}

func summarizeCompiledRuntimeManifest(rm *domain.CompiledRuntimeManifest) string {
	if rm == nil {
		return ""
	}
	var parts []string
	if strings.TrimSpace(rm.Security.Network.Mode) != "" {
		parts = append(parts, fmt.Sprintf("network=%s", rm.Security.Network.Mode))
	}
	if strings.TrimSpace(rm.Security.FS.Root) == "readonly" {
		parts = append(parts, "root=readonly")
	}
	if len(rm.Security.FS.TempPaths) > 0 {
		parts = append(parts, "tmpfs="+strings.Join(rm.Security.FS.TempPaths, ","))
	}
	if rm.Security.Process.NoNewPrivileges {
		parts = append(parts, "no-new-privileges")
	}
	if len(rm.Security.Process.DropCaps) > 0 {
		parts = append(parts, "cap-drop="+strings.Join(rm.Security.Process.DropCaps, ","))
	}
	if rm.Security.Process.PIDLimit > 0 {
		parts = append(parts, fmt.Sprintf("pids=%d", rm.Security.Process.PIDLimit))
	}
	if cpu := strings.TrimSpace(rm.Security.Process.Resources.CPU); cpu != "" {
		parts = append(parts, "cpu="+cpu)
	}
	if mem := strings.TrimSpace(rm.Security.Process.Resources.Memory); mem != "" {
		parts = append(parts, "memory="+mem)
	}
	if len(parts) == 0 {
		return ""
	}
	return "runtime policy: " + strings.Join(parts, ", ")
}

func tmpfsPathForPolicy(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		return ""
	}
	if path == "/work" || strings.HasPrefix(path, "/work/") {
		return ""
	}
	return path
}

func appendIfMissing(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}
