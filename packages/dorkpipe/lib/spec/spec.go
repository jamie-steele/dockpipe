// Package spec defines the DorkPipe DAG YAML format (orchestrator on top of DockPipe).
package spec

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc is the root document.
type Doc struct {
	Name   string `yaml:"name"`
	Policy Policy `yaml:"policy"`
	Nodes  []Node `yaml:"nodes"`
}

// Policy controls escalation and aggregation.
type Policy struct {
	// EscalateConfidenceBelow: run codex (escalate_only) when aggregate calibrated score is below this (0–1).
	EscalateConfidenceBelow float64 `yaml:"escalate_confidence_below"`
	// MergeWeights weights dimensions when computing Calibrated from per-dimension harmonic means (defaults: equal 0.2 each).
	MergeWeights MergeWeights `yaml:"merge_weights,omitempty"`
	// MinParallel is a soft hint for logging; scheduler always maximizes parallelism per level.
	MinParallel int `yaml:"min_parallel,omitempty"`
	// EarlyStopCalibratedAbove: after each level, if phase-1 aggregate calibrated is at or above this, skip remaining phase-1 nodes (0 = disabled).
	EarlyStopCalibratedAbove float64 `yaml:"early_stop_calibrated_above,omitempty"`
	// BranchJudge is the node id whose stdout JSON {"winner":"..."} selects the winning branch for branch_required nodes (empty = no branch competition).
	BranchJudge string `yaml:"branch_judge,omitempty"`
}

// MergeWeights configures the linear blend into Calibrated (dimensions are first aggregated across nodes with harmonic mean).
type MergeWeights struct {
	NodeSelf         float64 `yaml:"node_self,omitempty"`
	Agreement        float64 `yaml:"agreement,omitempty"`
	RetrievalSupport float64 `yaml:"retrieval_support,omitempty"`
	Verifier         float64 `yaml:"verifier,omitempty"`
	ToolSuccess      float64 `yaml:"tool_success,omitempty"`
}

// Empty reports unset weights (use defaults).
func (m MergeWeights) Empty() bool {
	return m.NodeSelf == 0 && m.Agreement == 0 && m.RetrievalSupport == 0 && m.Verifier == 0 && m.ToolSuccess == 0
}

// DefaultMergeWeights: equal weight on five observable dimensions.
func DefaultMergeWeights() MergeWeights {
	return MergeWeights{
		NodeSelf:         0.2,
		Agreement:        0.2,
		RetrievalSupport: 0.2,
		Verifier:         0.2,
		ToolSuccess:      0.2,
	}
}

// Node is one vertex in the DAG.
type Node struct {
	ID    string   `yaml:"id"`
	Kind  string   `yaml:"kind"`
	Needs []string `yaml:"needs"`

	// kind=shell — bash -c (workdir = DOCKPIPE_WORKDIR / cwd)
	Script string `yaml:"script,omitempty"`

	// kind=dockpipe — runs `dockpipe` with these args (must include leading flags; use "--" before cmd if needed)
	DockpipeArgs []string `yaml:"dockpipe_args,omitempty"`
	Workdir      string   `yaml:"workdir,omitempty"`

	// kind=ollama — local model request (currently Ollama-backed; interface allows additional providers).
	Model         string         `yaml:"model,omitempty"`
	Prompt        string         `yaml:"prompt,omitempty"`
	ModelProvider string         `yaml:"model_provider,omitempty"`
	ModelHost     string         `yaml:"model_host,omitempty"`
	OllamaHost    string         `yaml:"ollama_host,omitempty"` // legacy alias for model_host
	NumCtx        int            `yaml:"num_ctx,omitempty"`
	KeepAlive     string         `yaml:"keep_alive,omitempty"`
	Temperature   *float64       `yaml:"temperature,omitempty"`
	NumPredict    int            `yaml:"num_predict,omitempty"`
	TopP          *float64       `yaml:"top_p,omitempty"`
	ModelOptions  map[string]any `yaml:"model_options,omitempty"`

	// kind=pgvector — requires DATABASE_URL or database_url_env
	SQL            string `yaml:"sql,omitempty"`
	DatabaseURL    string `yaml:"database_url,omitempty"`     // literal (discouraged) or empty
	DatabaseURLEnv string `yaml:"database_url_env,omitempty"` // env var name holding DSN

	// kind=codex — only executed when escalation triggers (see EscalateOnly) or always if false.
	EscalateOnly bool   `yaml:"escalate_only,omitempty"`
	DockpipeBin  string `yaml:"dockpipe_bin,omitempty"` // default PATH dockpipe

	// kind=verifier — same transport as ollama (HTTP /api/generate); stdout JSON carries verifier score (independent judge).
	// (Shares model, prompt, ollama_host with ollama.)

	// Conditional phase-1 execution (evaluated against current phase-1 aggregate before the node runs).
	// RetrieveIfCalibratedBelow: if >0, skip this node when aggregate calibrated is already >= threshold (conditional retrieval / extra workers).
	RetrieveIfCalibratedBelow float64 `yaml:"retrieve_if_calibrated_below,omitempty"`

	// Branch competition: only run this node if policy.branch_judge emitted this branch id as winner (requires needs: [branch_judge]).
	BranchRequired string `yaml:"branch_required,omitempty"`
	// BranchTag labels output for logging / provenance (optional).
	BranchTag string `yaml:"branch_tag,omitempty"`

	// ParallelGroup: nodes in the same group in the same level get an agreement score from dispersion of node_self (optional).
	ParallelGroup string `yaml:"parallel_group,omitempty"`
}

// DefaultPolicy fills zero values.
func DefaultPolicy(p Policy) Policy {
	if p.EscalateConfidenceBelow <= 0 {
		p.EscalateConfidenceBelow = 0.75
	}
	return p
}

// ParseFile reads and parses a YAML DAG spec.
func ParseFile(path string) (*Doc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse parses YAML bytes.
func Parse(b []byte) (*Doc, error) {
	var d Doc
	if err := yaml.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if strings.TrimSpace(d.Name) == "" {
		return nil, fmt.Errorf("spec: name is required")
	}
	if len(d.Nodes) == 0 {
		return nil, fmt.Errorf("spec: nodes is empty")
	}
	ids := make(map[string]struct{}, len(d.Nodes))
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if strings.TrimSpace(n.ID) == "" {
			return nil, fmt.Errorf("spec: node[%d] id is empty", i)
		}
		if _, dup := ids[n.ID]; dup {
			return nil, fmt.Errorf("spec: duplicate node id %q", n.ID)
		}
		ids[n.ID] = struct{}{}
	}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		for _, dep := range n.Needs {
			if _, ok := ids[dep]; !ok {
				return nil, fmt.Errorf("spec: node %q needs unknown id %q", n.ID, dep)
			}
		}
	}
	d.Policy = DefaultPolicy(d.Policy)
	return &d, nil
}

// NodeByID returns a node or nil.
func (d *Doc) NodeByID(id string) *Node {
	for i := range d.Nodes {
		if d.Nodes[i].ID == id {
			return &d.Nodes[i]
		}
	}
	return nil
}
