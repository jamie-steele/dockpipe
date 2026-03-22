package aggregator

import (
	"math"

	"dockpipe/lib/dorkpipe/confidence"
	"dockpipe/lib/dorkpipe/spec"
	"dockpipe/lib/dorkpipe/workers"
)

// mergeVectors aggregates per-node vectors: harmonic mean per dimension across nodes, then weighted blend into Calibrated.
func mergeVectors(results []*workers.Result, pol spec.Policy) confidence.Vector {
	active := activeResults(results)
	if len(active) == 0 {
		return confidence.Vector{Calibrated: 0}
	}
	if anyFailed(active) {
		return confidence.Vector{Calibrated: 0}
	}
	w := pol.MergeWeights
	if w.Empty() {
		w = spec.DefaultMergeWeights()
	}
	out := confidence.Vector{}
	dims := []struct {
		name string
		pick func(confidence.Vector) *float64
		set  func(*confidence.Vector, float64)
	}{
		{"node_self", func(v confidence.Vector) *float64 { return v.NodeSelf }, func(v *confidence.Vector, x float64) { v.NodeSelf = confidence.F64(x) }},
		{"agreement", func(v confidence.Vector) *float64 { return v.Agreement }, func(v *confidence.Vector, x float64) { v.Agreement = confidence.F64(x) }},
		{"retrieval_support", func(v confidence.Vector) *float64 { return v.RetrievalSupport }, func(v *confidence.Vector, x float64) { v.RetrievalSupport = confidence.F64(x) }},
		{"verifier", func(v confidence.Vector) *float64 { return v.Verifier }, func(v *confidence.Vector, x float64) { v.Verifier = confidence.F64(x) }},
		{"tool_success", func(v confidence.Vector) *float64 { return v.ToolSuccess }, func(v *confidence.Vector, x float64) { v.ToolSuccess = confidence.F64(x) }},
	}
	for _, d := range dims {
		var vals []float64
		for _, r := range active {
			if r == nil {
				continue
			}
			p := d.pick(r.Vector)
			if p != nil {
				vals = append(vals, confidence.Clamp01(*p))
			}
		}
		if len(vals) == 0 {
			continue
		}
		h := confidence.HarmonicMean(vals)
		if !math.IsNaN(h) {
			d.set(&out, h)
		}
	}
	out.Calibrated = weightedCalibrated(out, w)
	if math.IsNaN(out.Calibrated) {
		out.Calibrated = 0
	}
	return out
}

func activeResults(results []*workers.Result) []*workers.Result {
	var out []*workers.Result
	for _, r := range results {
		if r != nil && !r.Skipped {
			out = append(out, r)
		}
	}
	return out
}

func anyFailed(results []*workers.Result) bool {
	for _, r := range results {
		if r != nil && !r.Skipped && r.Err != nil {
			return true
		}
	}
	return false
}

func weightedCalibrated(v confidence.Vector, w spec.MergeWeights) float64 {
	var num, den float64
	add := func(p *float64, weight float64) {
		if p == nil || weight <= 0 {
			return
		}
		num += weight * confidence.Clamp01(*p)
		den += weight
	}
	add(v.NodeSelf, w.NodeSelf)
	add(v.Agreement, w.Agreement)
	add(v.RetrievalSupport, w.RetrievalSupport)
	add(v.Verifier, w.Verifier)
	add(v.ToolSuccess, w.ToolSuccess)
	if den <= 0 {
		return 0
	}
	return num / den
}
