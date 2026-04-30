package aggregator

import (
	"math"
	"testing"

	"dorkpipe.orchestrator/confidence"
	"dorkpipe.orchestrator/spec"
	"dorkpipe.orchestrator/workers"
)

func TestMergeVectorsHarmonic(t *testing.T) {
	a := &workers.Result{
		Vector: confidence.Vector{NodeSelf: confidence.F64(0.9), ToolSuccess: confidence.F64(1)},
	}
	b := &workers.Result{
		Vector: confidence.Vector{NodeSelf: confidence.F64(0.2), ToolSuccess: confidence.F64(1)},
	}
	v := mergeVectors([]*workers.Result{a, b}, spec.Policy{})
	if v.NodeSelf == nil {
		t.Fatal("expected node_self")
	}
	want := 2.0 / (1.0/0.9 + 1.0/0.2)
	if math.Abs(*v.NodeSelf-want) > 1e-6 {
		t.Fatalf("harmonic node_self got %v want %v", *v.NodeSelf, want)
	}
	if v.Calibrated <= 0 || v.Calibrated > 1 {
		t.Fatalf("calibrated %v", v.Calibrated)
	}
}

func TestMergeVectorsFailure(t *testing.T) {
	a := &workers.Result{Err: errX("x")}
	v := mergeVectors([]*workers.Result{a}, spec.Policy{})
	if v.Calibrated != 0 {
		t.Fatalf("want 0 got %v", v.Calibrated)
	}
}

func TestMergeVectorsSkipsSkipped(t *testing.T) {
	a := &workers.Result{
		Vector:  confidence.Vector{NodeSelf: confidence.F64(0.1), ToolSuccess: confidence.F64(1)},
		Skipped: true,
	}
	b := &workers.Result{
		Vector: confidence.Vector{NodeSelf: confidence.F64(0.9), ToolSuccess: confidence.F64(1)},
	}
	v := mergeVectors([]*workers.Result{a, b}, spec.Policy{})
	if v.NodeSelf == nil {
		t.Fatal("expected node_self from active node only")
	}
	if math.Abs(*v.NodeSelf-0.9) > 1e-6 {
		t.Fatalf("got %v want 0.9", *v.NodeSelf)
	}
}

type errX string

func (e errX) Error() string { return string(e) }
