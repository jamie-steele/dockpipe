package engine

import (
	"math"
	"strings"

	"dockpipe/src/lib/dorkpipe/confidence"
	"dockpipe/src/lib/dorkpipe/spec"
	"dockpipe/src/lib/dorkpipe/workers"
)

// ApplyParallelAgreement sets Vector.Agreement for nodes sharing parallel_group in the same level.
func ApplyParallelAgreement(levelIDs []string, batch map[string]*workers.Result, d *spec.Doc) {
	groups := make(map[string][]*workers.Result)
	for _, id := range levelIDs {
		r := batch[id]
		if r == nil || r.Skipped || r.Err != nil {
			continue
		}
		n := d.NodeByID(id)
		if n == nil {
			continue
		}
		g := strings.TrimSpace(n.ParallelGroup)
		if g == "" {
			continue
		}
		groups[g] = append(groups[g], r)
	}
	for _, rs := range groups {
		if len(rs) < 2 {
			continue
		}
		var vals []float64
		for _, r := range rs {
			if r.Vector.NodeSelf != nil {
				vals = append(vals, *r.Vector.NodeSelf)
			}
		}
		if len(vals) < 2 {
			continue
		}
		ag := agreementFromVariance(vals)
		for _, r := range rs {
			r.Vector.Agreement = confidence.F64(ag)
		}
	}
}

func agreementFromVariance(vals []float64) float64 {
	var mean float64
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))
	var varsum float64
	for _, v := range vals {
		d := v - mean
		varsum += d * d
	}
	variance := varsum / float64(len(vals))
	ag := 1.0 - math.Min(1.0, variance*4.0)
	if ag < 0 {
		return 0
	}
	return ag
}
