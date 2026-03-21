package domain

import "testing"

func TestFromResolverMapHostIsolate(t *testing.T) {
	ra := FromResolverMap(map[string]string{
		"DOCKPIPE_RESOLVER_HOST_ISOLATE": "scripts/cursor-dev-session.sh",
	})
	if ra.HostIsolate != "scripts/cursor-dev-session.sh" {
		t.Fatalf("HostIsolate: got %q", ra.HostIsolate)
	}
	if ra.Template != "" {
		t.Fatalf("Template should be empty when only host isolate is set, got %q", ra.Template)
	}
}

func TestFromResolverMapWorkflow(t *testing.T) {
	ra := FromResolverMap(map[string]string{
		"DOCKPIPE_RESOLVER_WORKFLOW": "vscode",
	})
	if ra.Workflow != "vscode" {
		t.Fatalf("Workflow: got %q", ra.Workflow)
	}
}
