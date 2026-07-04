// Package aggregator merges node results and decides escalation.
package aggregator

import (
	"math"

	"dorkpipe.orchestrator/confidence"
	"dorkpipe.orchestrator/spec"
	"dorkpipe.orchestrator/workers"
)

// Summary is the combined view after phase 1 (and optionally after codex).
type Summary struct {
	Vector  confidence.Vector
	Results []*workers.Result
}

// Combine aggregates multi-signal vectors (harmonic per dimension + weighted calibrated blend).
func Combine(results []*workers.Result, pol spec.Policy) Summary {
	return Summary{
		Vector:  mergeVectors(results, pol),
		Results: results,
	}
}

// ShouldEscalateToCodex returns true when aggregate calibrated score is below policy threshold.
func ShouldEscalateToCodex(sum Summary, pol spec.Policy) bool {
	th := pol.EscalateConfidenceBelow
	if th <= 0 {
		th = 0.75
	}
	return sum.Vector.Calibrated < th && !math.IsNaN(sum.Vector.Calibrated)
}
