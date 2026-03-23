package scheduler

import (
	"fmt"
	"sort"
	"strings"

	"dockpipe/lib/dorkpipe/spec"
)

// EscalationLevels schedules only codex+escalate_only nodes; edges from phase-1 nodes are ignored.
func EscalationLevels(d *spec.Doc) ([][]string, error) {
	included := make(map[string]*spec.Node)
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if strings.EqualFold(n.Kind, "codex") && n.EscalateOnly {
			included[n.ID] = n
		}
	}
	if len(included) == 0 {
		return nil, nil
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
			return nil, fmt.Errorf("scheduler escalation: deadlock")
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
