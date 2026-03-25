// Package engine runs a DorkPipe DAG: parallel levels, aggregate, optional Codex escalation.
package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"dockpipe/src/lib/dorkpipe/aggregator"
	"dockpipe/src/lib/dorkpipe/planner"
	"dockpipe/src/lib/dorkpipe/scheduler"
	"dockpipe/src/lib/dorkpipe/spec"
	"dockpipe/src/lib/dorkpipe/workers"
)

// Run executes the full pipeline (real workers). subst seeds {{key}} replacements.
func Run(ctx context.Context, d *spec.Doc, ex *workers.Executor, subst map[string]string) error {
	if subst == nil {
		subst = make(map[string]string)
	}
	subst["__branch_winner__"] = ""
	if err := planner.Validate(d); err != nil {
		return err
	}
	levels, err := scheduler.Levels(d, true)
	if err != nil {
		return err
	}
	done := make(map[string]bool)
	var phase1 []*workers.Result
	var earlyStopped bool
	for li, level := range levels {
		sum := aggregator.Combine(phase1, d.Policy)
		var toRun []string
		for _, id := range level {
			if done[id] {
				continue
			}
			n := d.NodeByID(id)
			if n == nil {
				return fmt.Errorf("engine: missing node %q", id)
			}
			skip, reason, err := shouldSkipPhase1(n, d, sum, subst)
			if err != nil {
				return err
			}
			if skip {
				phase1 = append(phase1, skippedResult(n, reason))
				done[id] = true
				continue
			}
			toRun = append(toRun, id)
		}
		batchResults := make(map[string]*workers.Result)
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, id := range toRun {
			id := id
			n := d.NodeByID(id)
			wg.Add(1)
			go func() {
				defer wg.Done()
				res := ex.Run(ctx, n, subst)
				mu.Lock()
				batchResults[id] = res
				mu.Unlock()
			}()
		}
		wg.Wait()
		for _, id := range toRun {
			res := batchResults[id]
			if res == nil {
				continue
			}
			phase1 = append(phase1, res)
			done[id] = true
			if res.Err == nil && res.Stdout != "" {
				subst[id] = truncate(res.Stdout, 128*1024)
				if strings.TrimSpace(d.Policy.BranchJudge) == id {
					if w := parseBranchWinner(res.Stdout); w != "" {
						subst["__branch_winner__"] = w
					}
				}
				_ = workers.WriteOutputs(ex.Workdir, id, res.Stdout)
			}
		}
		ApplyParallelAgreement(toRun, batchResults, d)
		for _, id := range toRun {
			res := batchResults[id]
			if res == nil {
				continue
			}
			if res.Err != nil {
				return fmt.Errorf("node %q (%s): %w", res.NodeID, res.Kind, res.Err)
			}
		}
		if d.Policy.EarlyStopCalibratedAbove > 0 && len(phase1) > 0 {
			sum2 := aggregator.Combine(phase1, d.Policy)
			if sum2.Vector.Calibrated >= d.Policy.EarlyStopCalibratedAbove {
				earlyStopped = true
				for j := li + 1; j < len(levels); j++ {
					for _, id := range levels[j] {
						if done[id] {
							continue
						}
						n := d.NodeByID(id)
						if n == nil {
							continue
						}
						phase1 = append(phase1, skippedResult(n, "early_stop"))
						done[id] = true
					}
				}
				break
			}
		}
	}
	sum := aggregator.Combine(phase1, d.Policy)
	if !aggregator.ShouldEscalateToCodex(sum, d.Policy) {
		_ = writeProvenance(ex.Workdir, d, phase1, nil, sum.Vector, false, subst, earlyStopped)
		_ = appendMetricsJSONL(ex.Workdir, d.Name, sum.Vector.Calibrated, false, earlyStopped, phase1)
		return nil
	}
	escLevels, err := scheduler.EscalationLevels(d)
	if err != nil {
		return err
	}
	if len(escLevels) == 0 {
		_ = writeProvenance(ex.Workdir, d, phase1, nil, sum.Vector, false, subst, earlyStopped)
		_ = appendMetricsJSONL(ex.Workdir, d.Name, sum.Vector.Calibrated, false, earlyStopped, phase1)
		return nil
	}
	var all []*workers.Result
	all = append(all, phase1...)
	var esc []*workers.Result
	for _, level := range escLevels {
		var wg sync.WaitGroup
		mu := sync.Mutex{}
		for _, id := range level {
			id := id
			n := d.NodeByID(id)
			if n == nil {
				return fmt.Errorf("engine: missing escalation node %q", id)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				res := ex.Run(ctx, n, subst)
				mu.Lock()
				esc = append(esc, res)
				all = append(all, res)
				if res.Err == nil && res.Stdout != "" {
					subst[id] = truncate(res.Stdout, 128*1024)
					_ = workers.WriteOutputs(ex.Workdir, id, res.Stdout)
				}
				mu.Unlock()
			}()
		}
		wg.Wait()
	}
	escalRan := len(esc) > 0
	for _, res := range esc {
		if res.Err != nil {
			return fmt.Errorf("codex escalation node %q: %w", res.NodeID, res.Err)
		}
	}
	final := aggregator.Combine(all, d.Policy)
	_ = writeProvenance(ex.Workdir, d, phase1, esc, sum.Vector, escalRan, subst, earlyStopped)
	_ = appendMetricsJSONL(ex.Workdir, d.Name, final.Vector.Calibrated, escalRan, earlyStopped, all)
	return nil
}

func shouldSkipPhase1(n *spec.Node, d *spec.Doc, sum aggregator.Summary, subst map[string]string) (skip bool, reason string, err error) {
	if n.BranchRequired != "" {
		w := strings.TrimSpace(subst["__branch_winner__"])
		if w == "" {
			return false, "", fmt.Errorf("node %q: branch winner not set; ensure node %q outputs JSON with \"winner\" matching branch_required", n.ID, strings.TrimSpace(d.Policy.BranchJudge))
		}
		if n.BranchRequired != w {
			return true, "branch_not_selected", nil
		}
	}
	if n.RetrieveIfCalibratedBelow > 0 && sum.Vector.Calibrated >= n.RetrieveIfCalibratedBelow {
		return true, "retrieve_if", nil
	}
	return false, "", nil
}

func skippedResult(n *spec.Node, reason string) *workers.Result {
	return &workers.Result{
		NodeID:     n.ID,
		Kind:       "skipped",
		Skipped:    true,
		SkipReason: reason,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…(truncated)"
}
