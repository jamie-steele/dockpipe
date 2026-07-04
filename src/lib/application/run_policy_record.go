package application

import (
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func writeRunPolicyRecord(workdir, wfName, wfConfig, stepID, imageRef, imageDecision string, rm *domain.CompiledRuntimeManifest) error {
	rec := buildRunPolicyRecord(workdir, wfName, wfConfig, stepID, imageRef, imageDecision, rm)
	if rec == nil {
		return nil
	}
	_, err := infrastructure.WriteRunPolicyRecord(workdir, rec)
	return err
}

func buildRunPolicyRecord(workdir, wfName, wfConfig, stepID, imageRef, imageDecision string, rm *domain.CompiledRuntimeManifest) *infrastructure.RunPolicyRecord {
	if strings.TrimSpace(workdir) == "" {
		return nil
	}
	rec := &infrastructure.RunPolicyRecord{
		WorkflowName:          strings.TrimSpace(wfName),
		WorkflowConfig:        strings.TrimSpace(wfConfig),
		StepID:                strings.TrimSpace(stepID),
		ImageRef:              strings.TrimSpace(imageRef),
		ImageArtifactDecision: strings.TrimSpace(imageDecision),
	}
	if rm != nil {
		rec.PolicyFingerprint = strings.TrimSpace(rm.PolicyFingerprint)
		rec.PolicySummary = summarizeCompiledRuntimeManifest(rm)
		rec.NetworkMode = strings.TrimSpace(rm.Security.Network.Mode)
		rec.NetworkEnforcement = strings.TrimSpace(rm.Security.Network.Enforcement)
		rec.AppliedRuleIDs = append([]string(nil), rm.RuleIDs...)
		rec.AdvisoryNotes = compactNonEmptyStrings(rm.EnforcementSummaries)
		rec.EnforcementNotes = compactNonEmptyStrings(compiledRuntimeEnforcementLogLines(rm))
	}
	return rec
}

func compactNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
