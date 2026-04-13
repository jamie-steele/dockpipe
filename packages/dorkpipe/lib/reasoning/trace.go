package reasoning

type TraceEvent struct {
	ContractVersion string         `json:"contract_version"`
	ArtifactVersion string         `json:"artifact_version"`
	RequestID       string         `json:"request_id"`
	ParentRequestID string         `json:"parent_request_id,omitempty"`
	Route           string         `json:"route,omitempty"`
	Phase           string         `json:"phase,omitempty"`
	EventType       string         `json:"event_type,omitempty"`
	Label           string         `json:"label,omitempty"`
	Status          string         `json:"status,omitempty"`
	Progress        float64        `json:"progress,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	ArtifactDir     string         `json:"artifact_dir,omitempty"`
	Timestamp       string         `json:"timestamp"`
}
