package application

var policyEnvHints = []string{
	"DOCKPIPE_POLICY_PROXY_URL",
	"DOCKPIPE_POLICY_PROXY_HTTPS_URL",
	"DOCKPIPE_POLICY_PROXY_NO_PROXY",
	"DOCKPIPE_NETWORK_PROXY_URL",
	"DOCKPIPE_NETWORK_PROXY_HTTPS_URL",
	"DOCKPIPE_NETWORK_PROXY_NO_PROXY",
}

// mergePolicyProxyEnvFromHost forwards DockPipe-managed proxy policy env into the
// docker environment map when those values were resolved earlier in the workflow.
// This keeps proxy-backed enforcement generic and engine-owned without copying the
// entire host/workflow environment into every container.
func mergePolicyProxyEnvFromHost(dst, src map[string]string) {
	for _, key := range policyEnvHints {
		mergeEnvHintKeys(dst, src, key)
	}
}
