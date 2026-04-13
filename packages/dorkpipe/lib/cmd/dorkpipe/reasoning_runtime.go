package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dorkpipe.orchestrator/reasoning"
	"dorkpipe.orchestrator/statepaths"
)

const reasoningArtifactVersion = "v2"

type runtimePolicy struct {
	BestOfN          int
	MaxBranches      int
	AbstainThreshold float64
	RepairMemory     bool
	HighAmbiguity    bool
	AmbiguityReason  string
	BranchingActive  bool
}

type chatAttemptResult struct {
	Attempt        reasoning.AttemptRecord
	Answer         string
	Validation     chatAnswerValidation
	FailureSummary string
}

func beginReasoningRun(root, requestID, route, parentRequestID string) (string, error) {
	artifactDir, err := statepaths.ReasoningArtifactsDir(root, requestID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", err
	}
	beginArtifactTrace(root, artifactDir, route, parentRequestID)
	return artifactDir, nil
}

func writeReasoningRunArtifact(artifactDir string, artifact *reasoning.RunArtifact) {
	if strings.TrimSpace(artifactDir) == "" || artifact == nil {
		return
	}
	writeJSON(filepath.Join(artifactDir, "reasoning.json"), artifact)
}

func readReasoningRunArtifact(artifactDir string) *reasoning.RunArtifact {
	if strings.TrimSpace(artifactDir) == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(artifactDir, "reasoning.json"))
	if err != nil {
		return nil
	}
	var artifact reasoning.RunArtifact
	if err := json.Unmarshal(body, &artifact); err != nil {
		return nil
	}
	return &artifact
}

func resolveRuntimePolicy(route, message, activeFile, selectionText string, candidateCount, openFileCount int) runtimePolicy {
	bestOfN := intEnvOrDefault("DORKPIPE_REASONING_BEST_OF_N", 3)
	maxBranches := intEnvOrDefault("DORKPIPE_REASONING_MAX_BRANCHES", 3)
	abstainThreshold := floatEnvOrDefault("DORKPIPE_REASONING_ABSTAIN_THRESHOLD", 0.62)
	repairMemory := boolEnvOrDefault("DORKPIPE_REASONING_REPAIR_MEMORY", true)
	if bestOfN < 1 {
		bestOfN = 1
	}
	if maxBranches < 1 {
		maxBranches = 1
	}
	policy := runtimePolicy{
		BestOfN:          bestOfN,
		MaxBranches:      maxBranches,
		AbstainThreshold: abstainThreshold,
		RepairMemory:     repairMemory,
	}
	messageLower := strings.ToLower(strings.TrimSpace(message))
	var reasons []string
	if route == "chat" {
		if isArchitectureChatQuery(message) || strings.Contains(messageLower, "tradeoff") || strings.Contains(messageLower, "compare") || strings.Contains(messageLower, "vs") {
			reasons = append(reasons, "architecture_or_tradeoff_question")
		}
		if activeFile == "" && selectionText == "" && candidateCount >= 3 {
			reasons = append(reasons, "wide_workspace_scope")
		}
	} else if route == "edit" {
		if activeFile == "" && selectionText == "" && candidateCount >= 4 {
			reasons = append(reasons, "edit_scope_is_broad")
		}
		if strings.Contains(messageLower, "refactor") || strings.Contains(messageLower, "rename") || strings.Contains(messageLower, "across") {
			reasons = append(reasons, "edit_requires_choice")
		}
	}
	if openFileCount >= 4 {
		reasons = append(reasons, "many_open_files")
	}
	policy.HighAmbiguity = len(reasons) > 0
	policy.AmbiguityReason = strings.Join(reasons, ", ")
	policy.BranchingActive = policy.HighAmbiguity && (policy.BestOfN > 1 || policy.MaxBranches > 1)
	return policy
}

func toPolicyRecord(policy runtimePolicy) reasoning.PolicyRecord {
	return reasoning.PolicyRecord{
		BestOfN:          policy.BestOfN,
		MaxBranches:      policy.MaxBranches,
		AbstainThreshold: policy.AbstainThreshold,
		RepairMemory:     policy.RepairMemory,
		HighAmbiguity:    policy.HighAmbiguity,
		AmbiguityReason:  policy.AmbiguityReason,
		BranchingActive:  policy.BranchingActive,
	}
}

func intEnvOrDefault(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func floatEnvOrDefault(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func boolEnvOrDefault(name string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func buildReasoningEvidence(graph chatEvidenceGraph) reasoning.EvidenceRecord {
	record := reasoning.EvidenceRecord{}
	for _, node := range graph.Nodes {
		record.Nodes = append(record.Nodes, reasoning.EvidenceNode{
			ID:      node.ID,
			Kind:    node.Kind,
			File:    node.File,
			Symbol:  node.Symbol,
			Summary: node.Summary,
		})
	}
	for _, edge := range graph.Edges {
		record.Edges = append(record.Edges, reasoning.EvidenceEdge{
			From: edge.From,
			To:   edge.To,
			Kind: edge.Kind,
		})
	}
	return record
}

func buildValidationFindings(validation chatAnswerValidation) []reasoning.ValidationFinding {
	if len(validation.Issues) == 0 {
		return nil
	}
	findings := make([]reasoning.ValidationFinding, 0, len(validation.Issues))
	for _, issue := range validation.Issues {
		findings = append(findings, reasoning.ValidationFinding{
			Severity: "error",
			Code:     "chat_validation",
			Message:  issue,
		})
	}
	return findings
}

func buildChatRunArtifact(reqID, route, summary string, req routeRequest, chatContext workspaceChatContext, validationStatus string, validation chatAnswerValidation, policy runtimePolicy, attempts []reasoning.AttemptRecord, decision reasoning.DecisionRecord, repairMemory []string) *reasoning.RunArtifact {
	candidates := make([]reasoning.RetrievalCandidate, 0, len(chatContext.Targets))
	for _, rel := range chatContext.Targets {
		candidates = append(candidates, reasoning.RetrievalCandidate{
			ID:       rel,
			Path:     rel,
			Source:   "workspace",
			Selected: true,
			Reason:   "retained_for_reasoning",
		})
	}
	output := reasoning.OutputRecord{
		Summary:          summary,
		Text:             summary,
		ValidationStatus: validationStatus,
		TargetFiles:      append([]string{}, chatContext.Targets...),
		Citations:        buildChatOutputCitations(chatContext.Evidence),
	}
	return &reasoning.RunArtifact{
		ArtifactVersion: reasoningArtifactVersion,
		Kind:            route,
		Request: reasoning.RequestRecord{
			RequestID:     reqID,
			Route:         route,
			Mode:          req.Mode,
			Message:       req.Message,
			ActiveFile:    req.ActiveFile,
			OpenFiles:     append([]string{}, req.OpenFiles...),
			SelectionText: req.SelectionText,
		},
		Policy: toPolicyRecord(policy),
		Retrieval: reasoning.RetrievalRecord{
			SearchTerms: extractChatSearchTerms(req.Message),
			Candidates:  candidates,
			Selected:    append([]string{}, chatContext.Targets...),
		},
		Evidence: buildReasoningEvidence(chatContext.Evidence),
		Validation: reasoning.ValidationRecord{
			Status:   validationStatus,
			Findings: buildValidationFindings(validation),
		},
		Attempts:     attempts,
		Decision:     decision,
		RepairMemory: append([]string{}, repairMemory...),
		Output: output,
	}
}

func buildChatOutputCitations(graph chatEvidenceGraph) []reasoning.OutputCitation {
	var out []reasoning.OutputCitation
	for _, node := range graph.Nodes {
		if node.Kind != "symbol" {
			continue
		}
		out = append(out, reasoning.OutputCitation{
			NodeID: node.ID,
			File:   node.File,
			Symbol: node.Symbol,
		})
	}
	return out
}

func buildEditRunArtifact(reqID, message, activeFile, selectionText string, plan *editPlan, candidates []string, artifact *editModelArtifact, validationStatus string, policy runtimePolicy, attempts []reasoning.AttemptRecord, decision reasoning.DecisionRecord, repairMemory []string) *reasoning.RunArtifact {
	retrievalCandidates := make([]reasoning.RetrievalCandidate, 0, len(candidates))
	for _, rel := range candidates {
		retrievalCandidates = append(retrievalCandidates, reasoning.RetrievalCandidate{
			ID:       rel,
			Path:     rel,
			Source:   "workspace",
			Selected: true,
			Reason:   "retained_for_edit",
		})
	}
	structuredOps := uniqueStructuredEditOps(nil)
	if artifact != nil {
		structuredOps = uniqueStructuredEditOps(artifact.StructuredEdits)
	}
	searchTerms := extractSearchTerms(message)
	if plan != nil {
		searchTerms = uniqueNonEmpty(append(searchTerms, plan.SearchTerms...))
	}
	targetFiles := []string{}
	summary := ""
	if artifact != nil {
		targetFiles = append(targetFiles, artifact.TargetFiles...)
		summary = artifact.Summary
	}
	return &reasoning.RunArtifact{
		ArtifactVersion: reasoningArtifactVersion,
		Kind:            "edit",
		Request: reasoning.RequestRecord{
			RequestID:     reqID,
			Route:         "edit",
			Message:       message,
			ActiveFile:    activeFile,
			SelectionText: selectionText,
		},
		Policy: toPolicyRecord(policy),
		Retrieval: reasoning.RetrievalRecord{
			SearchTerms: searchTerms,
			Candidates:  retrievalCandidates,
			Selected:    append([]string{}, candidates...),
		},
		Validation: reasoning.ValidationRecord{
			Status: validationStatus,
		},
		Attempts:     attempts,
		Decision:     decision,
		RepairMemory: append([]string{}, repairMemory...),
		Output: reasoning.OutputRecord{
			Summary:          summary,
			ValidationStatus: validationStatus,
			TargetFiles:      targetFiles,
			StructuredEditOp: structuredOps,
		},
	}
}

func buildChatBranchPrompts(policy runtimePolicy) []reasoning.AttemptRecord {
	count := policy.BestOfN
	if count > policy.MaxBranches {
		count = policy.MaxBranches
	}
	if count < 1 {
		count = 1
	}
	strategies := []struct {
		id    string
		label string
		hint  string
	}{
		{id: "branch-1", label: "balanced", hint: "Prefer the strongest code-anchored explanation with concise uncertainty."},
		{id: "branch-2", label: "control-flow", hint: "Prioritize control flow, routing, and validator-relevant behavior."},
		{id: "branch-3", label: "uncertainty-first", hint: "Prefer narrower claims and move anything weakly supported into Uncertain."},
	}
	attempts := make([]reasoning.AttemptRecord, 0, count)
	for i := 0; i < count; i++ {
		item := strategies[i%len(strategies)]
		attempts = append(attempts, reasoning.AttemptRecord{
			ID:    item.id,
			Label: item.label,
			Kind:  "branch",
			Metadata: map[string]any{
				"branch_hint": item.hint,
			},
		})
	}
	return attempts
}

func scoreChatAttempt(answer string, validation chatAnswerValidation, chatContext workspaceChatContext) float64 {
	score := 0.2
	if validation.Passed {
		score += 0.45
	}
	switch {
	case strings.Contains(answer, "## Confirmed"):
		score += 0.1
	}
	citations := countSupportedEvidenceCitations(answer, chatContext.Evidence)
	score += float64(minStructuredInt(citations, 4)) * 0.08
	if len(validation.Issues) > 0 {
		score -= float64(len(validation.Issues)) * 0.08
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func summarizeChatAttemptFailure(validation chatAnswerValidation) string {
	if validation.Passed {
		return ""
	}
	if len(validation.Issues) == 0 {
		return "validator_rejected_without_specific_issue"
	}
	return strings.Join(validation.Issues, "; ")
}

func buildEditBranchAttempts(candidates []string, plan *editPlan, policy runtimePolicy) ([]reasoning.AttemptRecord, reasoning.DecisionRecord) {
	if len(candidates) == 0 {
		return nil, reasoning.DecisionRecord{}
	}
	limit := policy.MaxBranches
	if limit > len(candidates) {
		limit = len(candidates)
	}
	if limit < 1 {
		limit = 1
	}
	targetHint := 1
	if plan != nil && len(plan.TargetFiles) > 0 {
		targetHint = len(plan.TargetFiles)
		if targetHint > 3 {
			targetHint = 3
		}
	}
	attempts := make([]reasoning.AttemptRecord, 0, limit)
	decision := reasoning.DecisionRecord{
		BranchesConsidered: limit,
		BranchesPruned:     maxInt(limit-1, 0),
	}
	for i := 0; i < limit; i++ {
		start := i
		end := i + targetHint
		if end > len(candidates) {
			end = len(candidates)
		}
		slice := append([]string{}, candidates[start:end]...)
		if len(slice) == 0 {
			slice = []string{candidates[i]}
		}
		score := 1.0 - float64(i)*0.15
		attempt := reasoning.AttemptRecord{
			ID:      fmt.Sprintf("edit-branch-%d", i+1),
			Label:   fmt.Sprintf("candidate set %d", i+1),
			Kind:    "branch",
			Status:  "candidate",
			Score:   score,
			Summary: strings.Join(slice, ", "),
			Metadata: map[string]any{
				"target_files": slice,
			},
		}
		if i == 0 {
			attempt.Selected = true
			attempt.Status = "selected"
			decision.SelectedAttemptID = attempt.ID
		} else {
			attempt.Pruned = true
			attempt.PrunedReason = "lower_ranked_candidate_branch"
			attempt.Status = "pruned"
		}
		attempts = append(attempts, attempt)
	}
	return attempts, decision
}
