// Package eval summarizes DorkPipe metrics from .dorkpipe/metrics.jsonl (evaluation harness).
package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Stats aggregates lines written by the engine (schema dorkpipe.metrics.v2).
type Stats struct {
	Lines          int
	AvgCalibrated  float64
	EscalationRate float64
	EarlyStopRate  float64
	AvgSkipped     float64
}

type metricLine struct {
	Calibrated   float64 `json:"calibrated"`
	Escalated    bool    `json:"escalated"`
	EarlyStop    bool    `json:"early_stop"`
	SkippedNodes int     `json:"skipped_nodes"`
}

// SummarizeFile reads metrics.jsonl from path.
func SummarizeFile(path string) (Stats, error) {
	f, err := os.Open(path)
	if err != nil {
		return Stats{}, err
	}
	defer f.Close()
	return SummarizeReader(f)
}

// SummarizeReader parses metrics JSONL.
func SummarizeReader(r io.Reader) (Stats, error) {
	var s Stats
	var sumCal, sumSkip float64
	var esc, early int
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m metricLine
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		s.Lines++
		sumCal += m.Calibrated
		sumSkip += float64(m.SkippedNodes)
		if m.Escalated {
			esc++
		}
		if m.EarlyStop {
			early++
		}
	}
	if err := sc.Err(); err != nil {
		return s, err
	}
	if s.Lines == 0 {
		return s, fmt.Errorf("eval: no metric lines")
	}
	s.AvgCalibrated = sumCal / float64(s.Lines)
	s.AvgSkipped = sumSkip / float64(s.Lines)
	s.EscalationRate = float64(esc) / float64(s.Lines)
	s.EarlyStopRate = float64(early) / float64(s.Lines)
	return s, nil
}
