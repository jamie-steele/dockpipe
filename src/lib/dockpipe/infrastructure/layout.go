package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// ShipyardDir is the top-level directory name for the materialized bundle layout
// (…/core/…, …/workflows/…) under RepoRoot. Authoring checkouts use this name for repo-local
// maintainer workflows (CI, quick iteration) — distinct from stable templates/ and future CDN bundles.
const ShipyardDir = "shipyard"

// EmbeddedTemplatesPrefix is the path prefix of bundled workflow/core files inside dockpipe.BundledFS (see embed.go).
const EmbeddedTemplatesPrefix = "src/templates"

// authoringTemplatesRoot returns the directory containing workflow dirs and core/ for authoring checkouts.
// Prefers src/templates when src/templates/core exists (dockpipe source tree); otherwise templates/ (user projects from dockpipe init).
func authoringTemplatesRoot(repoRoot string) string {
	src := filepath.Join(repoRoot, "src", "templates")
	if st, err := os.Stat(filepath.Join(src, "core")); err == nil && st.IsDir() {
		return src
	}
	return filepath.Join(repoRoot, "templates")
}

// UsesBundledAssetLayout reports whether repoRoot is the materialized embedded bundle:
// it contains …/core (and typically …/workflows). Authoring checkouts use
// src/templates/ or templates/ and …/core/ instead.
func UsesBundledAssetLayout(repoRoot string) bool {
	p := filepath.Join(repoRoot, ShipyardDir, "core")
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// CoreDir returns .../src/templates/core or .../templates/core (authoring) or .../<ShipyardDir>/core (materialized bundle).
func CoreDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, ShipyardDir, "core")
	}
	return filepath.Join(authoringTemplatesRoot(repoRoot), "core")
}

// DefaultUserWorkflowsDirRel is the default directory (under repo root) for named workflows in normal projects.
// The dockpipe source tree and the materialized bundle use different roots (see WorkflowsRootDir).
const DefaultUserWorkflowsDirRel = "workflows"

// StagingWorkflowsDirRel is committed maintainer / experimental workflows (dockpipe repo only; merged into the same embed + materialized tree).
const StagingWorkflowsDirRel = ".staging/workflows"

// StagingWorkflowsDir returns <repoRoot>/.staging/workflows (may not exist in downstream projects).
func StagingWorkflowsDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".staging", "workflows")
}

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

// DockpipeAuthoringSourceTree is true when repoRoot is a dockpipe git checkout (src/templates/core present).
func DockpipeAuthoringSourceTree(repoRoot string) bool {
	st, err := os.Stat(filepath.Join(repoRoot, "src", "templates", "core"))
	return err == nil && st.IsDir()
}

// WorkflowsRootDir returns the directory containing named workflow folders (each with config.yml):
// materialized bundle → shipyard/workflows; dockpipe source → repo workflows/ when present, else src/templates;
// normal projects → workflows/ (or override).
func WorkflowsRootDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, ShipyardDir, "workflows")
	}
	if DockpipeAuthoringSourceTree(repoRoot) {
		wf := filepath.Join(repoRoot, DefaultUserWorkflowsDirRel)
		if WorkflowsDirHasDockpipeWorkflow(wf) {
			return wf
		}
		return filepath.Join(repoRoot, "src", "templates")
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
