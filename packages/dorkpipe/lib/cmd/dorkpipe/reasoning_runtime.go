package main

import (
	"os"
	"path/filepath"
	"strings"

	"dorkpipe.orchestrator/reasoning"
	"dorkpipe.orchestrator/statepaths"
)

const reasoningArtifactVersion = "v1"

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

func buildChatRunArtifact(reqID, route, summary string, req routeRequest, chatContext workspaceChatContext, validationStatus string, validation chatAnswerValidation) *reasoning.RunArtifact {
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

func buildEditRunArtifact(reqID, message, activeFile, selectionText string, plan *editPlan, candidates []string, artifact *editModelArtifact, validationStatus string) *reasoning.RunArtifact {
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
		Retrieval: reasoning.RetrievalRecord{
			SearchTerms: searchTerms,
			Candidates:  retrievalCandidates,
			Selected:    append([]string{}, candidates...),
		},
		Validation: reasoning.ValidationRecord{
			Status: validationStatus,
		},
		Output: reasoning.OutputRecord{
			Summary:          summary,
			ValidationStatus: validationStatus,
			TargetFiles:      targetFiles,
			StructuredEditOp: structuredOps,
		},
	}
}
