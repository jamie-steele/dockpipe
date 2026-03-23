package eval

import (
	"strings"
	"testing"
)

func TestSummarizeReader(t *testing.T) {
	s := `{"calibrated":0.8,"escalated":false,"early_stop":false,"skipped_nodes":2,"schema":"dorkpipe.metrics.v2"}
{"calibrated":0.6,"escalated":true,"early_stop":true,"skipped_nodes":0,"schema":"dorkpipe.metrics.v2"}
`
	st, err := SummarizeReader(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	if st.Lines != 2 {
		t.Fatalf("lines %d", st.Lines)
	}
	if st.AvgCalibrated < 0.69 || st.AvgCalibrated > 0.71 {
		t.Fatalf("avg calibrated %v", st.AvgCalibrated)
	}
	if st.EscalationRate != 0.5 || st.EarlyStopRate != 0.5 {
		t.Fatalf("rates esc=%v early=%v", st.EscalationRate, st.EarlyStopRate)
	}
	if st.AvgSkipped != 1 {
		t.Fatalf("avg skipped %v", st.AvgSkipped)
	}
}
