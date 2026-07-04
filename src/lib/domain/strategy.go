package domain

import "strings"

// StrategyAssignments is derived from strategies/<name> KEY=value files (same style as resolvers).
//   - DOCKPIPE_STRATEGY_BEFORE: comma-separated repo-relative host scripts run before the workflow body.
//   - DOCKPIPE_STRATEGY_AFTER: comma-separated host scripts run after successful workflow completion.
//   - DOCKPIPE_STRATEGY_KIND: optional tag (e.g. git) for documentation only.
type StrategyAssignments struct {
	Before []string
	After  []string
	Kind   string
}

// FromStrategyMap extracts dockpipe strategy fields from a parsed assignment map.
func FromStrategyMap(m map[string]string) StrategyAssignments {
	before := splitCommaPaths(m["DOCKPIPE_STRATEGY_BEFORE"])
	after := splitCommaPaths(m["DOCKPIPE_STRATEGY_AFTER"])
	return StrategyAssignments{
		Before: before,
		After:  after,
		Kind:   strings.TrimSpace(m["DOCKPIPE_STRATEGY_KIND"]),
	}
}

func splitCommaPaths(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
