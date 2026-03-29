package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// BundledCacheParentDir is the directory under the user cache (e.g. ~/.cache on Linux) that holds
// versioned materialized bundles: <cache>/dockpipe/bundled-<version>/.
const BundledCacheParentDir = "dockpipe"

// BundledLayoutDir is the inner directory name for core/ and workflows/ under a materialized bundle root
// (…/bundled-<ver>/bundle/core, …/bundle/workflows). Not the old "shipyard" segment name.
const BundledLayoutDir = "bundle"

// EmbeddedTemplatesPrefix is the path prefix of bundled authoring core + workflows inside dockpipe.BundledFS (see embed.go).
// Source layout: src/core/{assets,resolvers,runtimes,strategies,workflows/…}; maps to materialized bundle/core and bundle/workflows.
const EmbeddedTemplatesPrefix = "src/core"

// UsesBundledAssetLayout reports whether repoRoot is the materialized embedded bundle:
// it contains …/bundle/core (and typically …/bundle/workflows). Authoring checkouts use
// src/core/ or templates/ + templates/core/ instead.
func UsesBundledAssetLayout(repoRoot string) bool {
	p := filepath.Join(repoRoot, BundledLayoutDir, "core")
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// CoreDir returns .../src/core (dockpipe source), .../templates/core (downstream init), or .../bundle/core (materialized bundle).
func CoreDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, BundledLayoutDir, "core")
	}
	srcCore := filepath.Join(repoRoot, "src", "core")
	if st, err := os.Stat(filepath.Join(srcCore, "runtimes")); err == nil && st.IsDir() {
		return srcCore
	}
	return filepath.Join(repoRoot, "templates", "core")
}

// DefaultUserWorkflowsDirRel is the default directory (under repo root) for named workflows in normal projects.
// The dockpipe source tree and the materialized bundle use different roots (see WorkflowsRootDir).
const DefaultUserWorkflowsDirRel = "workflows"

// workflowsDirRelProcess is set by the CLI for the current process (--workflows-dir); cleared after the command.
var workflowsDirRelProcess string

// SetWorkflowsDirForProcess sets a repo-relative or absolute workflows directory for this process (empty clears).
// Persist across commands with DOCKPIPE_WORKFLOWS_DIR instead.
func SetWorkflowsDirForProcess(rel string) {
	workflowsDirRelProcess = strings.TrimSpace(rel)
	if workflowsDirRelProcess != "" {
		workflowsDirRelProcess = filepath.Clean(workflowsDirRelProcess)
	}
}

func effectiveWorkflowsDirRel() string {
	if workflowsDirRelProcess != "" {
		return workflowsDirRelProcess
	}
	v := strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOWS_DIR"))
	if v != "" {
		return filepath.Clean(v)
	}
	return ""
}

// DockpipeAuthoringSourceTree is true when repoRoot is a dockpipe git checkout (src/core/runtimes present).
func DockpipeAuthoringSourceTree(repoRoot string) bool {
	st, err := os.Stat(filepath.Join(repoRoot, "src", "core", "runtimes"))
	return err == nil && st.IsDir()
}

// BundledWorkflowsAuthoringDir returns .../src/core/workflows when present (dockpipe source); empty string otherwise.
func BundledWorkflowsAuthoringDir(repoRoot string) string {
	if !DockpipeAuthoringSourceTree(repoRoot) {
		return ""
	}
	return filepath.Join(repoRoot, "src", "core", "workflows")
}

// WorkflowsRootDir returns the directory containing named workflow folders (each with config.yml):
// materialized bundle → bundle/workflows; dockpipe source → repo workflows/ when present, else src/core/workflows;
// normal projects → workflows/ (or override).
func WorkflowsRootDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, BundledLayoutDir, "workflows")
	}
	if DockpipeAuthoringSourceTree(repoRoot) {
		wf := filepath.Join(repoRoot, DefaultUserWorkflowsDirRel)
		if WorkflowsDirHasDockpipeWorkflow(wf) {
			return wf
		}
		if d := BundledWorkflowsAuthoringDir(repoRoot); d != "" {
			return d
		}
	}
	rel := effectiveWorkflowsDirRel()
	if rel == "" {
		rel = DefaultUserWorkflowsDirRel
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(repoRoot, rel)
}
