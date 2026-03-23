package infrastructure

import (
	"bytes"
	"testing"
)

// TestIsNodeFamilyImage classifies dockpipe claude/codex/agent-dev images vs base/alpine.
func TestIsNodeFamilyImage(t *testing.T) {
	if !isNodeFamilyImage("dockpipe-claude:latest") || !isNodeFamilyImage("dockpipe-codex:1") || !isNodeFamilyImage("dockpipe-agent-dev") {
		t.Fatal("expected node family")
	}
	if isNodeFamilyImage("dockpipe-base-dev:latest") || isNodeFamilyImage("alpine:3") {
		t.Fatal("expected non-node family")
	}
}

// TestUnixDockerUserSpec_nonRootHost maps host uid:gid into -u for node-family images on Unix.
func TestUnixDockerUserSpec_nonRootHost(t *testing.T) {
	oldU, oldG := getuidDockerFn, getgidDockerFn
	getuidDockerFn = func() int { return 1000 }
	getgidDockerFn = func() int { return 1000 }
	t.Cleanup(func() {
		getuidDockerFn = oldU
		getgidDockerFn = oldG
	})
	if got := unixDockerUserSpec("dockpipe-claude:latest", nil); got != "1000:1000" {
		t.Fatalf("got %q", got)
	}
}

func TestUnixDockerUserSpec_rootHost_nodeImage(t *testing.T) {
	oldU, oldG := getuidDockerFn, getgidDockerFn
	getuidDockerFn = func() int { return 0 }
	getgidDockerFn = func() int { return 0 }
	t.Cleanup(func() {
		getuidDockerFn = oldU
		getgidDockerFn = oldG
	})
	t.Setenv("DOCKPIPE_FORCE_ROOT_CONTAINER", "")
	t.Setenv("DOCKPIPE_CONTAINER_USER", "")

	var buf bytes.Buffer
	got := unixDockerUserSpec("dockpipe-claude:latest", &buf)
	if got != "node" {
		t.Fatalf("got %q want node", got)
	}
	if buf.Len() == 0 {
		t.Fatal("expected stderr notice")
	}
}

// TestUnixDockerUserSpec_rootHost_baseDev keeps 0:0 for base-dev when the host is root.
func TestUnixDockerUserSpec_rootHost_baseDev(t *testing.T) {
	oldU, oldG := getuidDockerFn, getgidDockerFn
	getuidDockerFn = func() int { return 0 }
	getgidDockerFn = func() int { return 0 }
	t.Cleanup(func() {
		getuidDockerFn = oldU
		getgidDockerFn = oldG
	})
	t.Setenv("DOCKPIPE_FORCE_ROOT_CONTAINER", "")
	t.Setenv("DOCKPIPE_CONTAINER_USER", "")

	if got := unixDockerUserSpec("dockpipe-base-dev:latest", nil); got != "0:0" {
		t.Fatalf("got %q want 0:0", got)
	}
}

// TestUnixDockerUserSpec_forceRoot honors DOCKPIPE_FORCE_ROOT_CONTAINER for node-family images.
func TestUnixDockerUserSpec_forceRoot(t *testing.T) {
	oldU, oldG := getuidDockerFn, getgidDockerFn
	getuidDockerFn = func() int { return 0 }
	getgidDockerFn = func() int { return 0 }
	t.Cleanup(func() {
		getuidDockerFn = oldU
		getgidDockerFn = oldG
	})
	t.Setenv("DOCKPIPE_FORCE_ROOT_CONTAINER", "1")
	t.Setenv("DOCKPIPE_CONTAINER_USER", "")

	if got := unixDockerUserSpec("dockpipe-claude:latest", nil); got != "0:0" {
		t.Fatalf("got %q", got)
	}
}

// TestWindowsDockerUserSpec covers DOCKPIPE_WINDOWS_CONTAINER_USER: omit, node, root, and empty.
func TestWindowsDockerUserSpec(t *testing.T) {
	t.Run("unset omits -u", func(t *testing.T) {
		t.Setenv("DOCKPIPE_WINDOWS_CONTAINER_USER", "")
		if got := windowsDockerUserSpec(); got != "" {
			t.Fatalf("got %q want empty", got)
		}
	})
	t.Run("omit", func(t *testing.T) {
		t.Setenv("DOCKPIPE_WINDOWS_CONTAINER_USER", "omit")
		if got := windowsDockerUserSpec(); got != "" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("explicit node", func(t *testing.T) {
		t.Setenv("DOCKPIPE_WINDOWS_CONTAINER_USER", "node")
		if got := windowsDockerUserSpec(); got != "node" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("explicit root", func(t *testing.T) {
		t.Setenv("DOCKPIPE_WINDOWS_CONTAINER_USER", "0")
		if got := windowsDockerUserSpec(); got != "0" {
			t.Fatalf("got %q", got)
		}
	})
}

// TestUnixDockerUserSpec_overrideUser honors DOCKPIPE_CONTAINER_USER when set.
func TestUnixDockerUserSpec_overrideUser(t *testing.T) {
	oldU, oldG := getuidDockerFn, getgidDockerFn
	getuidDockerFn = func() int { return 0 }
	getgidDockerFn = func() int { return 0 }
	t.Cleanup(func() {
		getuidDockerFn = oldU
		getgidDockerFn = oldG
	})
	t.Setenv("DOCKPIPE_FORCE_ROOT_CONTAINER", "")
	t.Setenv("DOCKPIPE_CONTAINER_USER", "1000:1000")

	if got := unixDockerUserSpec("dockpipe-claude:latest", nil); got != "1000:1000" {
		t.Fatalf("got %q", got)
	}
}
