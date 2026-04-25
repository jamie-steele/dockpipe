package confidence

import (
	"math"
	"testing"
)

func TestHarmonicMean(t *testing.T) {
	h := HarmonicMean([]float64{0.9, 0.2})
	want := 2.0 / (1.0/0.9 + 1.0/0.2)
	if math.Abs(h-want) > 1e-9 {
		t.Fatalf("got %v want %v", h, want)
	}
}
