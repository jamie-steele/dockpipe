package orchestrationhelper

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSoftwareDevPromotionPatchGenerationAndApprovedApply(t *testing.T) {
	repoRoot, artifactRoot, taskPackPath, stepID := writePromotionFixture(t)
	extendPromotionFixtureAllowlist(t, artifactRoot)
	taskPackAbsolute := filepath.Join(repoRoot, filepath.FromSlash(taskPackPath))
	agentsAbsolute := filepath.Join(filepath.Dir(taskPackAbsolute), "agents.yml")
	taskPackBefore := mustReadTestFile(t, taskPackAbsolute)
	agentsBefore := mustReadTestFile(t, agentsAbsolute)

	if err := evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, stepID, artifactRoot); err != nil {
		t.Fatal(err)
	}
	if err := buildSoftwareDevPromotionPatchArtifacts(repoRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(artifactRoot, "proposal", "promotion-patch.json")
	patchPath := filepath.Join(artifactRoot, "proposal", "promotion.patch")
	firstManifest := mustReadTestFile(t, manifestPath)
	firstPatch := mustReadTestFile(t, patchPath)
	if err := buildSoftwareDevPromotionPatchArtifacts(repoRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstManifest, mustReadTestFile(t, manifestPath)) || !bytes.Equal(firstPatch, mustReadTestFile(t, patchPath)) {
		t.Fatal("promotion patch generation is not byte-for-byte deterministic")
	}
	if !bytes.Equal(taskPackBefore, mustReadTestFile(t, taskPackAbsolute)) || !bytes.Equal(agentsBefore, mustReadTestFile(t, agentsAbsolute)) {
		t.Fatal("promotion patch generation mutated the consumer repository")
	}
	patchText := string(firstPatch)
	for _, required := range []string{"--- a/" + taskPackPath, "--- a/workflows/software-dev/agents.yml", "Reusable promoted guidance.", "Reusable merge guidance", "reusable-floor.md"} {
		if !strings.Contains(patchText, required) {
			t.Fatalf("promotion patch does not contain %q:\n%s", required, patchText)
		}
	}
	addedPatchText := promotionAddedPatchLines(patchText)
	for _, forbidden := range []string{"session-model", "max_cloud_tokens", "target_root: docs/generated"} {
		if strings.Contains(addedPatchText, forbidden) {
			t.Fatalf("promotion patch contains excluded session/hard field %q", forbidden)
		}
	}

	manifest := readPromotionManifestForTest(t, manifestPath)
	if manifest.ContractVersion != promotionPatchContractVersion || !manifest.ApprovalRequired || manifest.WorkspaceMutation.Performed {
		t.Fatalf("unexpected promotion manifest: %#v", manifest)
	}
	if len(manifest.Targets) != 2 || manifest.Targets[0].StepID != stepID {
		t.Fatalf("unexpected exact targets: %#v", manifest.Targets)
	}
	approvalPath := writePromotionApprovalForTest(t, artifactRoot, manifest, nil)
	if err := applySoftwareDevPromotionPatch(repoRoot, artifactRoot, approvalPath); err != nil {
		t.Fatal(err)
	}
	taskPackAfter := mustReadTestFile(t, taskPackAbsolute)
	agentsAfter := mustReadTestFile(t, agentsAbsolute)
	if bytes.Equal(taskPackBefore, taskPackAfter) || bytes.Equal(agentsBefore, agentsAfter) {
		t.Fatal("approved promotion did not change both exact targets")
	}
	assertAppliedPromotionYAML(t, taskPackAbsolute, agentsAbsolute, stepID)
	result := readJSONMap(filepath.Join(artifactRoot, "proposal", "promotion-apply-result.json"))
	if stringValue(result["status"]) != "applied" || !boolAny(result["no_other_consumer_paths_changed"]) {
		t.Fatalf("unexpected promotion apply result: %s", mustJSON(result, nil))
	}
	if got := stringList(result["changed_target_paths"]); len(got) != 2 || got[0] != taskPackPath || got[1] != "workflows/software-dev/agents.yml" {
		t.Fatalf("changed targets = %#v", got)
	}

	if err := applySoftwareDevPromotionPatch(repoRoot, artifactRoot, approvalPath); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(taskPackAfter, mustReadTestFile(t, taskPackAbsolute)) || !bytes.Equal(agentsAfter, mustReadTestFile(t, agentsAbsolute)) {
		t.Fatal("idempotent promotion application rewrote an already-applied target")
	}
	result = readJSONMap(filepath.Join(artifactRoot, "proposal", "promotion-apply-result.json"))
	if stringValue(result["status"]) != "already_applied" {
		t.Fatalf("idempotent status = %q", stringValue(result["status"]))
	}
	assertNoPromotionTemporaryFiles(t, repoRoot, artifactRoot)
}

func TestSoftwareDevPromotionPatchUsesInlineRolesOnlyWithoutOwnedSibling(t *testing.T) {
	repoRoot, artifactRoot, taskPackPath, stepID := writePromotionFixture(t)
	taskPackAbsolute := filepath.Join(repoRoot, filepath.FromSlash(taskPackPath))
	agentsAbsolute := filepath.Join(filepath.Dir(taskPackAbsolute), "agents.yml")
	if err := os.Remove(agentsAbsolute); err != nil {
		t.Fatal(err)
	}
	if err := evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, stepID, artifactRoot); err != nil {
		t.Fatal(err)
	}
	compiled, err := compileSoftwareDevPromotionPatch(repoRoot, artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Targets) != 1 || len(compiled.Manifest.Targets) != 1 {
		t.Fatalf("inline-role patch targets = %#v", compiled.Manifest.Targets)
	}
	if compiled.Manifest.Targets[0].AssignedChanges["inline_roles"] == nil {
		t.Fatal("task-pack target does not record inline role changes")
	}
	var workflow map[string]any
	if err := yamlUnmarshalForTest(compiled.Targets[0].After, &workflow); err != nil {
		t.Fatal(err)
	}
	step := findWorkflowStepForTest(t, workflow, stepID)
	roles := mapValue(mapValue(mapValue(step["agent"])["orchestration"])["agents"])
	if !strings.Contains(stringValue(mapValue(roles["writer"])["role"]), "reusable evidence writer") {
		t.Fatalf("inline role delta was not assigned to selected step: %#v", roles)
	}
	if _, err := os.Stat(agentsAbsolute); !os.IsNotExist(err) {
		t.Fatalf("inline-role generation created a sibling agents.yml: %v", err)
	}
}

func TestPromotionPatchDeltaRejectsSessionAndHardAuthorityFields(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"task_pack_step": map[string]any{"orchestration": map[string]any{}},
			"roles":          map[string]any{},
		}
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{name: "tasks", field: "tasks", mutate: func(delta map[string]any) {
			mapValue(mapValue(delta["task_pack_step"])["orchestration"])["tasks"] = []any{}
		}},
		{name: "access", field: "access", mutate: func(delta map[string]any) {
			mapValue(mapValue(delta["task_pack_step"])["orchestration"])["access"] = map[string]any{}
		}},
		{name: "approval", field: "require_approval", mutate: func(delta map[string]any) {
			mapValue(mapValue(delta["task_pack_step"])["orchestration"])["apply"] = map[string]any{"require_approval": false}
		}},
		{name: "provider", field: "worker", mutate: func(delta map[string]any) {
			delta["roles"] = map[string]any{"writer": map[string]any{"worker": "codex"}}
		}},
		{name: "auth", field: "auth", mutate: func(delta map[string]any) {
			mapValue(mapValue(delta["task_pack_step"])["orchestration"])["merge"] = map[string]any{"auth": "x"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta := base()
			tt.mutate(delta)
			err := validatePromotionPatchDelta(delta)
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("validatePromotionPatchDelta() error = %v, want field %q", err, tt.field)
			}
		})
	}
}

func TestSoftwareDevPromotionPatchRejectsEditedIneligibleAndDriftedCandidates(t *testing.T) {
	t.Run("edited candidate", func(t *testing.T) {
		repoRoot, artifactRoot, taskPackPath, stepID := writePromotionFixture(t)
		if err := evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, stepID, artifactRoot); err != nil {
			t.Fatal(err)
		}
		candidatePath := filepath.Join(artifactRoot, "proposal", "promotion-candidate.json")
		candidate := readJSONMap(candidatePath)
		mapValue(candidate["promotable_soft_layer_delta"])["tasks"] = []any{"injected"}
		if err := writeJSONFile(candidatePath, candidate); err != nil {
			t.Fatal(err)
		}
		err := buildSoftwareDevPromotionPatchArtifacts(repoRoot, artifactRoot)
		if err == nil || !strings.Contains(err.Error(), "does not exactly match") {
			t.Fatalf("edited candidate error = %v", err)
		}
		assertNoFinalPromotionPatch(t, artifactRoot)
	})

	t.Run("ineligible candidate", func(t *testing.T) {
		repoRoot, artifactRoot, taskPackPath, stepID := writePromotionFixture(t)
		verificationPath := filepath.Join(artifactRoot, "verify", "result.json")
		verification := readJSONMap(verificationPath)
		verification["status"] = "review"
		verification["failure_class"] = "verification_review"
		if err := writeJSONFile(verificationPath, verification); err != nil {
			t.Fatal(err)
		}
		if err := evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, stepID, artifactRoot); err != nil {
			t.Fatal(err)
		}
		err := buildSoftwareDevPromotionPatchArtifacts(repoRoot, artifactRoot)
		if err == nil || !strings.Contains(err.Error(), "not eligible") {
			t.Fatalf("ineligible candidate error = %v", err)
		}
		assertNoFinalPromotionPatch(t, artifactRoot)
	})

	t.Run("drifted target", func(t *testing.T) {
		repoRoot, artifactRoot, taskPackPath, stepID := writePromotionFixture(t)
		if err := evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, stepID, artifactRoot); err != nil {
			t.Fatal(err)
		}
		taskPackAbsolute := filepath.Join(repoRoot, filepath.FromSlash(taskPackPath))
		drifted := append(mustReadTestFile(t, taskPackAbsolute), []byte("# drift\n")...)
		if err := os.WriteFile(taskPackAbsolute, drifted, 0o644); err != nil {
			t.Fatal(err)
		}
		err := buildSoftwareDevPromotionPatchArtifacts(repoRoot, artifactRoot)
		if err == nil || !strings.Contains(err.Error(), "does not exactly match") {
			t.Fatalf("drifted target error = %v", err)
		}
		assertNoFinalPromotionPatch(t, artifactRoot)
	})
}

func TestSoftwareDevPromotionApprovalFailuresDoNotMutateTargets(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, string, promotionPatchManifest, string) string
		stale   bool
	}{
		{name: "missing", prepare: func(_ *testing.T, _ string, _ promotionPatchManifest, _ string) string { return "" }},
		{name: "denied", prepare: func(t *testing.T, root string, manifest promotionPatchManifest, _ string) string {
			return writePromotionApprovalForTest(t, root, manifest, func(approval *promotionApproval) { approval.Decision = "deny"; approval.Approved = false })
		}},
		{name: "malformed", prepare: func(t *testing.T, root string, _ promotionPatchManifest, _ string) string {
			path := filepath.Join(root, "proposal", "promotion-approval.json")
			if err := os.WriteFile(path, []byte("{\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "wrong digest", prepare: func(t *testing.T, root string, manifest promotionPatchManifest, _ string) string {
			return writePromotionApprovalForTest(t, root, manifest, func(approval *promotionApproval) { approval.PatchSHA256 = "sha256:" + strings.Repeat("0", 64) })
		}},
		{name: "stale target", stale: true, prepare: func(t *testing.T, root string, manifest promotionPatchManifest, _ string) string {
			return writePromotionApprovalForTest(t, root, manifest, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot, artifactRoot, taskPackPath, _ := writeBuiltPromotionFixture(t)
			taskPackAbsolute := filepath.Join(repoRoot, filepath.FromSlash(taskPackPath))
			agentsAbsolute := filepath.Join(filepath.Dir(taskPackAbsolute), "agents.yml")
			manifest := readPromotionManifestForTest(t, filepath.Join(artifactRoot, "proposal", "promotion-patch.json"))
			approvalPath := tt.prepare(t, artifactRoot, manifest, taskPackPath)
			if tt.stale {
				if err := os.WriteFile(taskPackAbsolute, append(mustReadTestFile(t, taskPackAbsolute), []byte("# stale\n")...), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			taskPackBefore := mustReadTestFile(t, taskPackAbsolute)
			agentsBefore := mustReadTestFile(t, agentsAbsolute)
			err := applySoftwareDevPromotionPatch(repoRoot, artifactRoot, approvalPath)
			if err == nil {
				t.Fatal("unapproved or stale promotion unexpectedly applied")
			}
			if !bytes.Equal(taskPackBefore, mustReadTestFile(t, taskPackAbsolute)) || !bytes.Equal(agentsBefore, mustReadTestFile(t, agentsAbsolute)) {
				t.Fatal("failed promotion approval mutated a target")
			}
		})
	}
}

func TestSoftwareDevPromotionApplyRollsBackFirstTarget(t *testing.T) {
	repoRoot, artifactRoot, taskPackPath, _ := writeBuiltPromotionFixture(t)
	taskPackAbsolute := filepath.Join(repoRoot, filepath.FromSlash(taskPackPath))
	agentsAbsolute := filepath.Join(filepath.Dir(taskPackAbsolute), "agents.yml")
	taskPackBefore := mustReadTestFile(t, taskPackAbsolute)
	agentsBefore := mustReadTestFile(t, agentsAbsolute)
	manifest := readPromotionManifestForTest(t, filepath.Join(artifactRoot, "proposal", "promotion-patch.json"))
	approvalPath := writePromotionApprovalForTest(t, artifactRoot, manifest, nil)
	originalReplace := promotionReplaceFile
	calls := 0
	promotionReplaceFile = func(stagedPath, targetPath string) error {
		calls++
		if calls == 2 {
			return errors.New("injected second-target replacement failure")
		}
		return os.Rename(stagedPath, targetPath)
	}
	t.Cleanup(func() { promotionReplaceFile = originalReplace })
	err := applySoftwareDevPromotionPatch(repoRoot, artifactRoot, approvalPath)
	if err == nil || !strings.Contains(err.Error(), "injected second-target") {
		t.Fatalf("transactional apply error = %v", err)
	}
	if !bytes.Equal(taskPackBefore, mustReadTestFile(t, taskPackAbsolute)) || !bytes.Equal(agentsBefore, mustReadTestFile(t, agentsAbsolute)) {
		t.Fatal("two-target failure did not restore original bytes")
	}
	result := readJSONMap(filepath.Join(artifactRoot, "proposal", "promotion-apply-result.json"))
	if stringValue(mapValue(result["rollback"])["status"]) != "succeeded" {
		t.Fatalf("rollback result = %s", mustJSON(result, nil))
	}
	assertNoPromotionTemporaryFiles(t, repoRoot, artifactRoot)
}

func TestSoftwareDevPromotionInvalidStagedWorkflowDoesNotMutate(t *testing.T) {
	repoRoot, artifactRoot, taskPackPath, _ := writeBuiltPromotionFixture(t)
	taskPackAbsolute := filepath.Join(repoRoot, filepath.FromSlash(taskPackPath))
	agentsAbsolute := filepath.Join(filepath.Dir(taskPackAbsolute), "agents.yml")
	taskPackBefore := mustReadTestFile(t, taskPackAbsolute)
	agentsBefore := mustReadTestFile(t, agentsAbsolute)
	manifest := readPromotionManifestForTest(t, filepath.Join(artifactRoot, "proposal", "promotion-patch.json"))
	approvalPath := writePromotionApprovalForTest(t, artifactRoot, manifest, nil)
	originalValidate := promotionValidateWorkflow
	promotionValidateWorkflow = func(string) error { return errors.New("injected invalid patched workflow") }
	t.Cleanup(func() { promotionValidateWorkflow = originalValidate })
	err := applySoftwareDevPromotionPatch(repoRoot, artifactRoot, approvalPath)
	if err == nil || !strings.Contains(err.Error(), "invalid patched workflow") {
		t.Fatalf("invalid staged workflow error = %v", err)
	}
	if !bytes.Equal(taskPackBefore, mustReadTestFile(t, taskPackAbsolute)) || !bytes.Equal(agentsBefore, mustReadTestFile(t, agentsAbsolute)) {
		t.Fatal("invalid staged workflow mutated consumer targets")
	}
}

func extendPromotionFixtureAllowlist(t *testing.T, artifactRoot string) {
	t.Helper()
	normalizedPath := filepath.Join(artifactRoot, "proposal", "normalized.json")
	proposal := readJSONMap(normalizedPath)
	orchestration := contractOrchestration(proposal)
	orchestration["constraints"] = []any{"stable orchestration guidance"}
	orchestration["plan"] = map[string]any{
		"goal": "Use the reusable review plan.", "steps": []any{"inspect stable sources"}, "constraints": []any{"keep the plan reviewable"},
	}
	orchestration["merge"] = map[string]any{"title": "Reusable merge guidance"}
	orchestration["verify"] = map[string]any{"next_action_default": "review reusable evidence"}
	contractApply(proposal)["required_artifacts"] = []any{"existing.md", "promoted.md", "reusable-floor.md"}
	metadataPath := filepath.Join(artifactRoot, "proposal", "metadata.json")
	metadata := readJSONMap(metadataPath)
	metadata["constraints"] = []any{"preserve reusable evidence", "stable orchestration guidance", "keep the plan reviewable"}
	if err := writeJSONFile(metadataPath, metadata); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(normalizedPath, proposal); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "proposal", "raw.json"), []byte(mustJSON(proposal, nil)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBuiltPromotionFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	repoRoot, artifactRoot, taskPackPath, stepID := writePromotionFixture(t)
	if err := evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, stepID, artifactRoot); err != nil {
		t.Fatal(err)
	}
	if err := buildSoftwareDevPromotionPatchArtifacts(repoRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	return repoRoot, artifactRoot, taskPackPath, stepID
}

func readPromotionManifestForTest(t *testing.T, path string) promotionPatchManifest {
	t.Helper()
	manifest := promotionPatchManifest{}
	if err := decodePromotionJSONStrict(mustReadTestFile(t, path), &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func writePromotionApprovalForTest(t *testing.T, artifactRoot string, manifest promotionPatchManifest, mutate func(*promotionApproval)) string {
	t.Helper()
	approval := promotionApproval{
		ContractVersion: promotionApprovalContractVersion,
		Decision:        "approve",
		Approved:        true,
		PatchSHA256:     manifest.Patch.SHA256,
		Targets:         make([]promotionApprovalTarget, 0, len(manifest.Targets)),
	}
	for _, target := range manifest.Targets {
		approval.Targets = append(approval.Targets, promotionApprovalTarget{Path: target.Path, BeforeSHA256: target.BeforeSHA256})
	}
	if mutate != nil {
		mutate(&approval)
	}
	path := filepath.Join(artifactRoot, "proposal", "promotion-approval.json")
	raw, err := marshalPromotionJSON(approval)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertAppliedPromotionYAML(t *testing.T, taskPackPath, agentsPath, stepID string) {
	t.Helper()
	workflow := map[string]any{}
	if err := yamlUnmarshalForTest(mustReadTestFile(t, taskPackPath), &workflow); err != nil {
		t.Fatal(err)
	}
	step := findWorkflowStepForTest(t, workflow, stepID)
	agent := mapValue(step["agent"])
	startup := stringValue(agent["startup_prompt"])
	if !strings.Contains(startup, "Existing repo guidance.") || !strings.Contains(startup, "Reusable promoted guidance.") || !strings.Contains(startup, "- preserve reusable evidence") || !strings.Contains(startup, "- stable orchestration guidance") {
		t.Fatalf("startup/root/orchestration guidance was not promoted: %#v", agent)
	}
	orchestration := mapValue(agent["orchestration"])
	plan := mapValue(orchestration["plan"])
	if stringValue(plan["goal"]) != "Use the reusable review plan." {
		t.Fatalf("plan goal = %q", stringValue(plan["goal"]))
	}
	assertStringOrder(t, "promoted plan steps", stringList(plan["steps"]), []string{"inspect stable sources", "keep the plan reviewable"})
	assertStringOrder(t, "promoted required floor", stringList(mapValue(orchestration["apply"])["required_artifacts"]), []string{"existing.md", "promoted.md", "reusable-floor.md"})
	if stringValue(mapValue(orchestration["merge"])["title"]) != "Reusable merge guidance" || stringValue(mapValue(orchestration["verify"])["next_action_default"]) != "review reusable evidence" {
		t.Fatalf("merge/verify guidance was not promoted: %#v", orchestration)
	}
	agents := map[string]any{}
	if err := yamlUnmarshalForTest(mustReadTestFile(t, agentsPath), &agents); err != nil {
		t.Fatal(err)
	}
	writer := mapValue(mapValue(agents["agents"])["writer"])
	if !strings.Contains(stringValue(writer["role"]), "repo writer") || !strings.Contains(stringValue(writer["role"]), "reusable evidence writer") {
		t.Fatalf("sibling role wording = %q", stringValue(writer["role"]))
	}
	assertStringOrder(t, "sibling role constraints", stringList(writer["constraints"]), []string{"cite durable sources"})
}

func findWorkflowStepForTest(t *testing.T, workflow map[string]any, stepID string) map[string]any {
	t.Helper()
	for _, raw := range listValue(workflow["steps"]) {
		step := mapValue(raw)
		if stringValue(step["id"]) == stepID {
			return step
		}
	}
	t.Fatalf("workflow step %q not found", stepID)
	return nil
}

func yamlUnmarshalForTest(raw []byte, target any) error {
	return yaml.Unmarshal(raw, target)
}

func mustReadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertNoFinalPromotionPatch(t *testing.T, artifactRoot string) {
	t.Helper()
	for _, name := range []string{"promotion-patch.json", "promotion.patch"} {
		if _, err := os.Stat(filepath.Join(artifactRoot, "proposal", name)); !os.IsNotExist(err) {
			t.Fatalf("failed generation left %s: %v", name, err)
		}
	}
}

func assertNoPromotionTemporaryFiles(t *testing.T, repoRoot, artifactRoot string) {
	t.Helper()
	patterns := []string{
		filepath.Join(repoRoot, "workflows", "software-dev", ".promotion-apply-*"),
		filepath.Join(repoRoot, "workflows", "software-dev", ".promotion-rollback-*"),
		filepath.Join(artifactRoot, "proposal", ".promotion-patch-stage-*"),
		filepath.Join(artifactRoot, "proposal", ".promotion-workflow-*"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) > 0 {
			t.Fatalf("promotion temporary files for %s: %#v, %v", pattern, matches, err)
		}
	}
}

func promotionAddedPatchLines(patch string) string {
	added := []string{}
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added = append(added, line)
		}
	}
	return strings.Join(added, "\n")
}
