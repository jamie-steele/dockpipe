package domain

// ResolverAssignments is derived from resolvers/<name> KEY=value files.
type ResolverAssignments struct {
	Template  string
	PreScript string
	Action    string
}

// FromResolverMap extracts dockpipe resolver fields from a parsed assignment map.
func FromResolverMap(m map[string]string) ResolverAssignments {
	return ResolverAssignments{
		Template:  m["DOCKPIPE_RESOLVER_TEMPLATE"],
		PreScript: m["DOCKPIPE_RESOLVER_PRE_SCRIPT"],
		Action:    m["DOCKPIPE_RESOLVER_ACTION"],
	}
}
