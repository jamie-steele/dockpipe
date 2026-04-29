package reasoning

type RequestRecord struct {
	RequestID       string   `json:"request_id"`
	ParentRequestID string   `json:"parent_request_id,omitempty"`
	Route           string   `json:"route"`
	Mode            string   `json:"mode,omitempty"`
	Message         string   `json:"message"`
	ActiveFile      string   `json:"active_file,omitempty"`
	OpenFiles       []string `json:"open_files,omitempty"`
	SelectionText   string   `json:"selection_text,omitempty"`
}

type RetrievalCandidate struct {
	ID       string         `json:"id,omitempty"`
	Path     string         `json:"path"`
	Source   string         `json:"source,omitempty"`
	Selected bool           `json:"selected,omitempty"`
	Score    int            `json:"score,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type RetrievalRecord struct {
	SearchTerms []string             `json:"search_terms,omitempty"`
	Candidates  []RetrievalCandidate `json:"candidates,omitempty"`
	Selected    []string             `json:"selected,omitempty"`
}

type PolicyRecord struct {
	BestOfN          int     `json:"best_of_n,omitempty"`
	MaxBranches      int     `json:"max_branches,omitempty"`
	AbstainThreshold float64 `json:"abstain_threshold,omitempty"`
	RepairMemory     bool    `json:"repair_memory,omitempty"`
	HighAmbiguity    bool    `json:"high_ambiguity,omitempty"`
	AmbiguityReason  string  `json:"ambiguity_reason,omitempty"`
	BranchingActive  bool    `json:"branching_active,omitempty"`
}

type EvidenceNode struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	File      string   `json:"file,omitempty"`
	Symbol    string   `json:"symbol,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	StartLine int      `json:"start_line,omitempty"`
	EndLine   int      `json:"end_line,omitempty"`
	Language  string   `json:"language,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type EvidenceEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type EvidenceRecord struct {
	Nodes []EvidenceNode `json:"nodes,omitempty"`
	Edges []EvidenceEdge `json:"edges,omitempty"`
}

type ValidationFinding struct {
	Severity string   `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Message  string   `json:"message"`
	NodeIDs  []string `json:"node_ids,omitempty"`
}

type ValidationRecord struct {
	Status   string              `json:"status,omitempty"`
	Findings []ValidationFinding `json:"findings,omitempty"`
}

type AttemptRecord struct {
	ID               string         `json:"id"`
	Label            string         `json:"label,omitempty"`
	Kind             string         `json:"kind,omitempty"`
	Status           string         `json:"status,omitempty"`
	Score            float64        `json:"score,omitempty"`
	Summary          string         `json:"summary,omitempty"`
	ValidationStatus string         `json:"validation_status,omitempty"`
	FailureSummary   string         `json:"failure_summary,omitempty"`
	Selected         bool           `json:"selected,omitempty"`
	Pruned           bool           `json:"pruned,omitempty"`
	PrunedReason     string         `json:"pruned_reason,omitempty"`
	Escalate         bool           `json:"escalate,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type DecisionRecord struct {
	SelectedAttemptID  string `json:"selected_attempt_id,omitempty"`
	Abstained          bool   `json:"abstained,omitempty"`
	Escalated          bool   `json:"escalated,omitempty"`
	EscalationReason   string `json:"escalation_reason,omitempty"`
	BranchesConsidered int    `json:"branches_considered,omitempty"`
	BranchesPruned     int    `json:"branches_pruned,omitempty"`
}

type OutputCitation struct {
	NodeID string `json:"node_id,omitempty"`
	File   string `json:"file,omitempty"`
	Symbol string `json:"symbol,omitempty"`
}

type OutputRecord struct {
	Summary          string           `json:"summary,omitempty"`
	Text             string           `json:"text,omitempty"`
	Citations        []OutputCitation `json:"citations,omitempty"`
	ValidationStatus string           `json:"validation_status,omitempty"`
	TargetFiles      []string         `json:"target_files,omitempty"`
	StructuredEditOp []string         `json:"structured_edit_ops,omitempty"`
}

type ModelCallRecord struct {
	Provider             string         `json:"provider,omitempty"`
	BaseURL              string         `json:"base_url,omitempty"`
	Endpoint             string         `json:"endpoint,omitempty"`
	Model                string         `json:"model,omitempty"`
	Stream               bool           `json:"stream,omitempty"`
	PromptChars          int            `json:"prompt_chars,omitempty"`
	MessageCount         int            `json:"message_count,omitempty"`
	Options              map[string]any `json:"options,omitempty"`
	KeepAlive            string         `json:"keep_alive,omitempty"`
	TotalDurationNS      int64          `json:"total_duration_ns,omitempty"`
	LoadDurationNS       int64          `json:"load_duration_ns,omitempty"`
	PromptEvalCount      int            `json:"prompt_eval_count,omitempty"`
	PromptEvalDurationNS int64          `json:"prompt_eval_duration_ns,omitempty"`
	EvalCount            int            `json:"eval_count,omitempty"`
	EvalDurationNS       int64          `json:"eval_duration_ns,omitempty"`
	DoneReason           string         `json:"done_reason,omitempty"`
}

type RunArtifact struct {
	ArtifactVersion string            `json:"artifact_version"`
	Kind            string            `json:"kind"`
	Request         RequestRecord     `json:"request"`
	Policy          PolicyRecord      `json:"policy,omitempty"`
	Retrieval       RetrievalRecord   `json:"retrieval,omitempty"`
	Evidence        EvidenceRecord    `json:"evidence,omitempty"`
	Validation      ValidationRecord  `json:"validation,omitempty"`
	Attempts        []AttemptRecord   `json:"attempts,omitempty"`
	Decision        DecisionRecord    `json:"decision,omitempty"`
	RepairMemory    []string          `json:"repair_memory,omitempty"`
	ModelCalls      []ModelCallRecord `json:"model_calls,omitempty"`
	Output          OutputRecord      `json:"output,omitempty"`
}
