package application

import "testing"

func TestMergePolicyProxyEnvFromHost(t *testing.T) {
	t.Run("copies dockpipe proxy settings from workflow env", func(t *testing.T) {
		dst := map[string]string{}
		src := map[string]string{
			"DOCKPIPE_POLICY_PROXY_URL":        "http://proxy-sidecar:8080",
			"DOCKPIPE_POLICY_PROXY_NO_PROXY":   "metadata.local",
			"DOCKPIPE_NETWORK_PROXY_HTTPS_URL": "http://proxy-sidecar:8443",
		}
		mergePolicyProxyEnvFromHost(dst, src)
		if dst["DOCKPIPE_POLICY_PROXY_URL"] != "http://proxy-sidecar:8080" {
			t.Fatalf("expected DOCKPIPE_POLICY_PROXY_URL copied, got %#v", dst)
		}
		if dst["DOCKPIPE_POLICY_PROXY_NO_PROXY"] != "metadata.local" {
			t.Fatalf("expected DOCKPIPE_POLICY_PROXY_NO_PROXY copied, got %#v", dst)
		}
		if dst["DOCKPIPE_NETWORK_PROXY_HTTPS_URL"] != "http://proxy-sidecar:8443" {
			t.Fatalf("expected DOCKPIPE_NETWORK_PROXY_HTTPS_URL copied, got %#v", dst)
		}
	})

	t.Run("explicit docker env still wins", func(t *testing.T) {
		dst := map[string]string{
			"DOCKPIPE_POLICY_PROXY_URL": "http://from-cli:8080",
		}
		src := map[string]string{
			"DOCKPIPE_POLICY_PROXY_URL": "http://from-workflow:8080",
		}
		mergePolicyProxyEnvFromHost(dst, src)
		if dst["DOCKPIPE_POLICY_PROXY_URL"] != "http://from-cli:8080" {
			t.Fatalf("expected existing docker env kept, got %#v", dst)
		}
	})
}
