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

type RunArtifact struct {
	ArtifactVersion string           `json:"artifact_version"`
	Kind            string           `json:"kind"`
	Request         RequestRecord    `json:"request"`
	Retrieval       RetrievalRecord  `json:"retrieval,omitempty"`
	Evidence        EvidenceRecord   `json:"evidence,omitempty"`
	Validation      ValidationRecord `json:"validation,omitempty"`
	Output          OutputRecord     `json:"output,omitempty"`
}
