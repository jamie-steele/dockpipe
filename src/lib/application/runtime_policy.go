package application

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func applyCompiledRuntimePolicy(runOpts *infrastructure.RunOpts, wfConfig, wfRoot string) (*domain.CompiledRuntimeManifest, error) {
	rm, err := loadCompiledRuntimeManifestForWorkflow(wfConfig, wfRoot)
	if err != nil {
		return nil, err
	}
	if rm == nil {
		return nil, nil
	}
	if runOpts != nil {
		if err := applyCompiledRuntimeManifest(runOpts, rm); err != nil {
			return nil, err
		}
	}
	return rm, nil
}

func applyCompiledRuntimeManifest(runOpts *infrastructure.RunOpts, rm *domain.CompiledRuntimeManifest) error {
	if runOpts == nil || rm == nil {
		return nil
	}
	if err := applyCompiledNetworkPolicy(runOpts, rm.Security.Network); err != nil {
		return err
	}
	applyCompiledFilesystemPolicy(runOpts, rm.Security.FS)
	applyCompiledProcessPolicy(runOpts, rm.Security.Process)
	return nil
}

func applyCompiledNetworkPolicy(runOpts *infrastructure.RunOpts, policy domain.CompiledNetworkPolicy) error {
	switch strings.TrimSpace(policy.Mode) {
	case "offline":
		runOpts.NetworkMode = "none"
	case "allowlist", "restricted":
		if strings.TrimSpace(policy.Enforcement) == "proxy" {
			if err := applyCompiledProxyNetworkPolicy(runOpts, policy); err != nil {
				return err
			}
		}
	}
	return nil
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
		if summary := compiledNetworkRuleSummary(rm.Security.Network); summary != "" {
			parts = append(parts, summary)
		}
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

func applyCompiledProxyNetworkPolicy(runOpts *infrastructure.RunOpts, policy domain.CompiledNetworkPolicy) error {
	if runOpts == nil {
		return nil
	}
	httpProxy := firstNonEmptyString(
		strings.TrimSpace(os.Getenv("DOCKPIPE_POLICY_PROXY_URL")),
		strings.TrimSpace(os.Getenv("DOCKPIPE_NETWORK_PROXY_URL")),
		strings.TrimSpace(os.Getenv("HTTP_PROXY")),
		strings.TrimSpace(os.Getenv("http_proxy")),
	)
	if httpProxy == "" {
		return fmt.Errorf("network policy requires proxy enforcement but no proxy URL is configured (set DOCKPIPE_POLICY_PROXY_URL or DOCKPIPE_NETWORK_PROXY_URL)")
	}
	httpsProxy := firstNonEmptyString(
		strings.TrimSpace(os.Getenv("DOCKPIPE_POLICY_PROXY_HTTPS_URL")),
		strings.TrimSpace(os.Getenv("DOCKPIPE_NETWORK_PROXY_HTTPS_URL")),
		strings.TrimSpace(os.Getenv("HTTPS_PROXY")),
		strings.TrimSpace(os.Getenv("https_proxy")),
		httpProxy,
	)
	noProxy := mergeNoProxyValues(
		strings.TrimSpace(os.Getenv("DOCKPIPE_POLICY_PROXY_NO_PROXY")),
		strings.TrimSpace(os.Getenv("DOCKPIPE_NETWORK_PROXY_NO_PROXY")),
		strings.TrimSpace(os.Getenv("NO_PROXY")),
		strings.TrimSpace(os.Getenv("no_proxy")),
		"localhost,127.0.0.1,::1",
	)
	runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "HTTP_PROXY", httpProxy)
	runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "http_proxy", httpProxy)
	runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "HTTPS_PROXY", httpsProxy)
	runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "https_proxy", httpsProxy)
	if noProxy != "" {
		runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "NO_PROXY", noProxy)
		runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "no_proxy", noProxy)
	}
	runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "DOCKPIPE_POLICY_NETWORK_MODE", strings.TrimSpace(policy.Mode))
	runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "DOCKPIPE_POLICY_NETWORK_ENFORCEMENT", "proxy")
	if len(policy.Allow) > 0 {
		runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "DOCKPIPE_POLICY_NETWORK_ALLOW", strings.Join(policy.Allow, ","))
	}
	if len(policy.Block) > 0 {
		runOpts.ExtraEnv = upsertEnvPair(runOpts.ExtraEnv, "DOCKPIPE_POLICY_NETWORK_BLOCK", strings.Join(policy.Block, ","))
	}
	return nil
}

func compiledNetworkRuleSummary(policy domain.CompiledNetworkPolicy) string {
	var parts []string
	if len(policy.Allow) > 0 {
		parts = append(parts, "allow="+summarizePolicyPatterns(policy.Allow))
	}
	if len(policy.Block) > 0 {
		parts = append(parts, "block="+summarizePolicyPatterns(policy.Block))
	}
	return strings.Join(parts, ",")
}

func summarizePolicyPatterns(values []string) string {
	clean := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			clean = append(clean, v)
		}
	}
	switch len(clean) {
	case 0:
		return ""
	case 1:
		return clean[0]
	case 2:
		return clean[0] + "," + clean[1]
	default:
		return fmt.Sprintf("%s,%s,+%d", clean[0], clean[1], len(clean)-2)
	}
}

func compiledRuntimePolicyLogLines(rm *domain.CompiledRuntimeManifest) []string {
	if rm == nil {
		return nil
	}
	var lines []string
	if summary := summarizeCompiledRuntimeManifest(rm); summary != "" {
		lines = append(lines, summary)
	}
	lines = append(lines, compiledRuntimeEnforcementLogLines(rm)...)
	for _, note := range rm.EnforcementSummaries {
		note = strings.TrimSpace(note)
		if note != "" {
			lines = append(lines, "policy note: "+note)
		}
	}
	if len(rm.RuleIDs) > 0 {
		rules := make([]string, 0, len(rm.RuleIDs))
		for _, rule := range rm.RuleIDs {
			rule = strings.TrimSpace(rule)
			if rule != "" {
				rules = append(rules, rule)
			}
		}
		if len(rules) > 0 {
			lines = append(lines, "policy rules: "+strings.Join(rules, ", "))
		}
	}
	return lines
}

func compiledRuntimeEnforcementLogLines(rm *domain.CompiledRuntimeManifest) []string {
	if rm == nil {
		return nil
	}
	var lines []string
	mode := strings.TrimSpace(rm.Security.Network.Mode)
	enforcement := strings.TrimSpace(rm.Security.Network.Enforcement)
	if mode != "" {
		switch enforcement {
		case "native":
			lines = append(lines, fmt.Sprintf("policy enforcement: network %s is enforced natively by the Docker runtime", mode))
		case "proxy":
			lines = append(lines, fmt.Sprintf("policy enforcement: network %s requires a proxy-backed egress layer", mode))
		case "advisory":
			lines = append(lines, fmt.Sprintf("policy enforcement: network %s is advisory in this build; full egress filtering is not active yet", mode))
			if len(rm.Security.Network.Allow) > 0 || len(rm.Security.Network.Block) > 0 || mode == "allowlist" || mode == "restricted" {
				lines = append(lines, "policy coverage: domain allow/block rules are compiled for inspection but are not enforced natively by Docker")
			}
		}
	}
	return lines
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

func upsertEnvPair(values []string, key, value string) []string {
	key = strings.TrimSpace(key)
	if key == "" {
		return values
	}
	prefix := key + "="
	entry := key + "=" + strings.TrimSpace(value)
	for i, existing := range values {
		if strings.HasPrefix(strings.TrimSpace(existing), prefix) {
			values[i] = entry
			return values
		}
	}
	return append(values, entry)
}

func mergeNoProxyValues(values ...string) string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key := strings.ToLower(part)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, part)
		}
	}
	return strings.Join(out, ",")
}
