package infrastructure

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// isNodeFamilyImage reports images that ship a non-root "node" user (see templates/core/assets/images/claude, codex).
// dockpipe-base-dev / dockpipe-dev do not use USER node; root host should not force -u node there.
func isNodeFamilyImage(image string) bool {
	i := strings.ToLower(image)
	return strings.Contains(i, "claude") ||
		strings.Contains(i, "codex") ||
		strings.Contains(i, "agent-dev")
}

// unixDockerUserSpec returns the value for docker run -u on Linux/macOS.
//
// When the dockpipe process runs as root (uid 0), mapping -u 0:0 makes CLIs such as Claude Code
// reject --dangerously-skip-permissions. For node-based images we default to user "node" unless
// overridden. See docs/cli-reference.md.
func unixDockerUserSpec(image string, stderr io.Writer) string {
	uid := getuidDockerFn()
	gid := getgidDockerFn()
	if uid != 0 || gid != 0 {
		return strconv.Itoa(uid) + ":" + strconv.Itoa(gid)
	}
	if strings.TrimSpace(os.Getenv("DOCKPIPE_FORCE_ROOT_CONTAINER")) == "1" {
		return "0:0"
	}
	if u := strings.TrimSpace(os.Getenv("DOCKPIPE_CONTAINER_USER")); u != "" {
		return u
	}
	if isNodeFamilyImage(image) {
		if stderr != nil {
			fmt.Fprintf(stderr, "[dockpipe] Host uid is 0; using container user \"node\" (override: DOCKPIPE_CONTAINER_USER=… or DOCKPIPE_FORCE_ROOT_CONTAINER=1).\n")
		}
		return "node"
	}
	return "0:0"
}

// windowsDockerUserSpec returns the value for docker run -u on Windows, or "" to omit -u.
//
// We do not default -u on Windows: passing -u node caused bind-mount stalls for some Docker Desktop
// setups. Omit -u so the image USER applies (see templates/core/assets/images/claude USER node).
//
// For Claude Code with --dangerously-skip-permissions (rejects root), set explicitly:
//
//	DOCKPIPE_WINDOWS_CONTAINER_USER=node
//
// Or "0" for root, "1000:1000", etc.
func windowsDockerUserSpec() string {
	u := strings.TrimSpace(os.Getenv("DOCKPIPE_WINDOWS_CONTAINER_USER"))
	if u == "-" || strings.EqualFold(u, "omit") {
		return ""
	}
	if u != "" {
		return u
	}
	return ""
}
