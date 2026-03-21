package domain

// ResolverAssignments is derived from resolvers/<name> KEY=value files.
// Contract (run-worktree resolvers — KEY=value files):
//   - DOCKPIPE_RESOLVER_TEMPLATE: built-in template name for docker image (see infrastructure.TemplateBuild).
//     Omit when DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RESOLVER_HOST_ISOLATE is set.
//   - DOCKPIPE_RESOLVER_WORKFLOW: optional; bundled workflow name under templates/<name>/config.yml (e.g. cursor-dev, vscode).
//     After pre-scripts, runs that workflow via the same multi-step runner — preferred over duplicating script paths.
//   - DOCKPIPE_RESOLVER_HOST_ISOLATE: optional; repo-relative script run on the host after pre-scripts instead of docker run.
//     Prefer DOCKPIPE_RESOLVER_WORKFLOW when a bundled workflow already exists.
//   - DOCKPIPE_RESOLVER_PRE_SCRIPT / DOCKPIPE_RESOLVER_ACTION: optional; override workflow run/act when using --resolver without --workflow.
//   - DOCKPIPE_RESOLVER_CMD: optional; default CLI binary name for docs / user prompts (not executed by the runner).
//   - DOCKPIPE_RESOLVER_ENV: optional; comma-separated env var names typically required for auth (documentation).
//   - DOCKPIPE_RESOLVER_EXPERIMENTAL=1: optional; marks resolver as experimental in docs.
type ResolverAssignments struct {
	Template      string
	Workflow      string
	HostIsolate   string
	PreScript     string
	Action        string
	Cmd           string
	EnvHint       string
	Experimental  bool
}

// FromResolverMap extracts dockpipe resolver fields from a parsed assignment map.
func FromResolverMap(m map[string]string) ResolverAssignments {
	exp := m["DOCKPIPE_RESOLVER_EXPERIMENTAL"] == "1" || m["DOCKPIPE_RESOLVER_EXPERIMENTAL"] == "true"
	return ResolverAssignments{
		Template:     m["DOCKPIPE_RESOLVER_TEMPLATE"],
		Workflow:     m["DOCKPIPE_RESOLVER_WORKFLOW"],
		HostIsolate:  m["DOCKPIPE_RESOLVER_HOST_ISOLATE"],
		PreScript:    m["DOCKPIPE_RESOLVER_PRE_SCRIPT"],
		Action:       m["DOCKPIPE_RESOLVER_ACTION"],
		Cmd:          m["DOCKPIPE_RESOLVER_CMD"],
		EnvHint:      m["DOCKPIPE_RESOLVER_ENV"],
		Experimental: exp,
	}
}
