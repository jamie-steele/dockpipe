// Package scheduler builds parallel execution levels (topological layers).
package scheduler

import (
	"fmt"
	"sort"
	"strings"

	"dorkpipe.orchestrator/spec"
)

// Levels returns batches of node IDs that can run in parallel.
// If excludeCodexEscalate is true, nodes with kind=codex and EscalateOnly=true are omitted (phase 2).
func Levels(d *spec.Doc, excludeCodexEscalate bool) ([][]string, error) {
	included := make(map[string]*spec.Node)
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if excludeCodexEscalate && strings.EqualFold(n.Kind, "codex") && n.EscalateOnly {
			continue
		}
		included[n.ID] = n
	}
	indeg := make(map[string]int)
	for id := range included {
		indeg[id] = 0
	}
	for id, n := range included {
		for _, dep := range n.Needs {
			if _, ok := included[dep]; !ok {
				continue
			}
			indeg[id]++
		}
	}
	var levels [][]string
	remaining := len(included)
	for remaining > 0 {
		var batch []string
		for id := range included {
			if indeg[id] == 0 {
				batch = append(batch, id)
			}
		}
		if len(batch) == 0 {
			return nil, fmt.Errorf("scheduler: deadlock (cycle or inconsistent deps)")
		}
		sort.Strings(batch)
		levels = append(levels, batch)
		for _, id := range batch {
			indeg[id] = -1
			remaining--
		}
		for _, id := range batch {
			for nid, n := range included {
				if indeg[nid] < 0 {
					continue
				}
				for _, dep := range n.Needs {
					if dep == id {
						indeg[nid]--
					}
				}
			}
		}
	}
	return levels, nil
}

// EscalationCodexNodes returns codex nodes with escalate_only (for phase 2).
func EscalationCodexNodes(d *spec.Doc) []string {
	var ids []string
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if strings.EqualFold(n.Kind, "codex") && n.EscalateOnly {
			ids = append(ids, n.ID)
		}
	}
	sort.Strings(ids)
	return ids
}
