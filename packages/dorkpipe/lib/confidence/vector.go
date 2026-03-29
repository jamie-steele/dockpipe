// Package confidence holds multi-signal scores and math helpers (no worker imports).
package confidence

import (
	"math"
)

// Vector holds optional per-dimension scores in [0,1]. Nil pointer = unknown for this node.
type Vector struct {
	NodeSelf         *float64 `json:"node_self,omitempty"`
	Agreement        *float64 `json:"agreement,omitempty"`
	RetrievalSupport *float64 `json:"retrieval_support,omitempty"`
	Verifier         *float64 `json:"verifier,omitempty"`
	ToolSuccess      *float64 `json:"tool_success,omitempty"`
	Calibrated       float64  `json:"calibrated"` // filled by aggregator merge
}

// F64 allocates a float for YAML/JSON-friendly optional fields.
func F64(v float64) *float64 { return &v }

// HarmonicMean is risk-averse: a single weak score pulls the mean down.
func HarmonicMean(vals []float64) float64 {
	if len(vals) == 0 {
		return math.NaN()
	}
	sumInv := 0.0
	for _, v := range vals {
		v = math.Max(clamp01(v), 1e-9)
		sumInv += 1.0 / v
	}
	return float64(len(vals)) / sumInv
}

// Clamp01 coerces to [0,1].
func Clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func clamp01(x float64) float64 { return Clamp01(x) }
