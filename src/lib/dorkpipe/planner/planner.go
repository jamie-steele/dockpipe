// Package planner validates DAGs (no cycles, known kinds).
package planner

import (
	"fmt"
	"strings"

	"dockpipe/src/lib/dorkpipe/spec"
)

var allowedKinds = map[string]struct{}{
	"shell":         {},
	"dockpipe":      {},
	"ollama":        {},
	"verifier":      {},
	"pgvector":      {},
	"codex":         {},
	"deterministic": {}, // alias for shell
}

func normKind(k string) string {
	s := strings.ToLower(strings.TrimSpace(k))
	if s == "deterministic" {
		return "shell"
	}
	return s
}

// Validate checks kinds and that the graph is a DAG.
func Validate(d *spec.Doc) error {
	for i := range d.Nodes {
		n := &d.Nodes[i]
		k := strings.ToLower(strings.TrimSpace(n.Kind))
		if _, ok := allowedKinds[k]; !ok {
			return fmt.Errorf("planner: node %q has unknown kind %q", n.ID, n.Kind)
		}
		switch normKind(n.Kind) {
		case "shell":
			if strings.TrimSpace(n.Script) == "" {
				return fmt.Errorf("planner: node %q (shell) requires script", n.ID)
			}
		case "dockpipe":
			if len(n.DockpipeArgs) == 0 {
				return fmt.Errorf("planner: node %q (dockpipe) requires dockpipe_args", n.ID)
			}
		case "ollama":
			if strings.TrimSpace(n.Model) == "" || strings.TrimSpace(n.Prompt) == "" {
				return fmt.Errorf("planner: node %q (ollama) requires model and prompt", n.ID)
			}
		case "pgvector":
			if strings.TrimSpace(n.SQL) == "" {
				return fmt.Errorf("planner: node %q (pgvector) requires sql", n.ID)
			}
			if strings.TrimSpace(n.DatabaseURL) == "" && strings.TrimSpace(n.DatabaseURLEnv) == "" {
				return fmt.Errorf("planner: node %q (pgvector) requires database_url or database_url_env", n.ID)
			}
		case "codex":
			if len(n.DockpipeArgs) == 0 {
				return fmt.Errorf("planner: node %q (codex) requires dockpipe_args (dockpipe invocation with codex resolver)", n.ID)
			}
		}
	}
	if err := branchPolicyCheck(d); err != nil {
		return err
	}
	return cycleCheck(d)
}

func branchPolicyCheck(d *spec.Doc) error {
	judge := strings.TrimSpace(d.Policy.BranchJudge)
	for i := range d.Nodes {
		n := &d.Nodes[i]
		req := strings.TrimSpace(n.BranchRequired)
		if req == "" {
			continue
		}
		if judge == "" {
			return fmt.Errorf("planner: node %q has branch_required %q but policy.branch_judge is empty", n.ID, req)
		}
		if n.ID == judge {
			return fmt.Errorf("planner: branch_judge node %q cannot set branch_required", judge)
		}
	}
	if judge == "" {
		return nil
	}
	if d.NodeByID(judge) == nil {
		return fmt.Errorf("planner: policy.branch_judge %q is not a node id", judge)
	}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		req := strings.TrimSpace(n.BranchRequired)
		if req == "" {
			continue
		}
		ok := false
		for _, dep := range n.Needs {
			if dep == judge {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("planner: node %q has branch_required %q but must list needs: [%q]", n.ID, req, judge)
		}
	}
	return nil
}

func cycleCheck(d *spec.Doc) error {
	// DFS white/gray/black
	visited := make(map[string]int) // 0 unvisited, 1 visiting, 2 done
	var dfs func(string) error
	dfs = func(id string) error {
		switch visited[id] {
		case 1:
			return fmt.Errorf("planner: cycle detected involving %q", id)
		case 2:
			return nil
		}
		visited[id] = 1
		n := d.NodeByID(id)
		if n == nil {
			return fmt.Errorf("planner: internal error missing node %q", id)
		}
		for _, dep := range n.Needs {
			if err := dfs(dep); err != nil {
				return err
			}
		}
		visited[id] = 2
		return nil
	}
	for i := range d.Nodes {
		if d.Nodes[i].ID == "" {
			continue
		}
		if visited[d.Nodes[i].ID] == 0 {
			if err := dfs(d.Nodes[i].ID); err != nil {
				return err
			}
		}
	}
	return nil
}
