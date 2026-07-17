package orchestrationhelper

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/infrastructure"
	"gopkg.in/yaml.v3"
)

const (
	promotionPatchContractVersion    = "software.dev.promotion-patch/v1"
	promotionApprovalContractVersion = "software.dev.promotion-approval/v1"
	promotionApplyContractVersion    = "software.dev.promotion-apply-result/v1"
)

type promotionPatchManifest struct {
	ContractVersion   string                     `json:"contract_version"`
	SourceCandidate   promotionSourceCandidate   `json:"source_candidate"`
	MutableSurface    promotionMutableSurface    `json:"mutable_surface"`
	ApprovalRequired  bool                       `json:"approval_required"`
	Patch             promotionPatchReference    `json:"patch"`
	Targets           []promotionPatchTarget     `json:"targets"`
	WorkspaceMutation promotionMutationStatement `json:"workspace_mutation"`
}

type promotionSourceCandidate struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type promotionMutableSurface struct {
	TaskPackPath string `json:"task_pack_path"`
	StepID       string `json:"step_id"`
}

type promotionPatchReference struct {
	Path   string `json:"path"`
	Format string `json:"format"`
	SHA256 string `json:"sha256"`
}

type promotionPatchTarget struct {
	Kind            string         `json:"kind"`
	Path            string         `json:"path"`
	StepID          string         `json:"step_id,omitempty"`
	BeforeSHA256    string         `json:"before_sha256"`
	AfterSHA256     string         `json:"after_sha256"`
	AssignedChanges map[string]any `json:"assigned_changes"`
}

type promotionMutationStatement struct {
	Performed    bool   `json:"performed"`
	Confirmation string `json:"confirmation"`
}

type promotionApproval struct {
	ContractVersion string                    `json:"contract_version"`
	Decision        string                    `json:"decision"`
	Approved        bool                      `json:"approved"`
	PatchSHA256     string                    `json:"patch_sha256"`
	Targets         []promotionApprovalTarget `json:"targets"`
}

type promotionApprovalTarget struct {
	Path         string `json:"path"`
	BeforeSHA256 string `json:"before_sha256"`
}

type promotionTargetImage struct {
	Kind     string
	Path     string
	StepID   string
	Absolute string
	Before   []byte
	After    []byte
}

type compiledPromotionPatch struct {
	Manifest      promotionPatchManifest
	ManifestBytes []byte
	PatchBytes    []byte
	Targets       []promotionTargetImage
}

var (
	promotionValidateWorkflow = infrastructure.ValidateResolvedWorkflowYAML
	promotionReplaceFile      = replacePromotionFileAtomic
)

func buildSoftwareDevPromotionPatchArtifacts(repoRoot, artifactRoot string) error {
	compiled, err := compileSoftwareDevPromotionPatch(repoRoot, artifactRoot)
	if err != nil {
		return err
	}
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	proposalDir := filepath.Join(rootPath, "proposal")
	if err := writePromotionArtifactSetAtomic(proposalDir, map[string][]byte{
		"promotion.patch":      compiled.PatchBytes,
		"promotion-patch.json": compiled.ManifestBytes,
	}); err != nil {
		return fmt.Errorf("write promotion patch artifacts: %w", err)
	}
	return nil
}

func compileSoftwareDevPromotionPatch(repoRoot, artifactRoot string) (*compiledPromotionPatch, error) {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return nil, err
	}
	proposalDir := filepath.Join(rootPath, "proposal")
	if err := requirePromotionDirectory(rootPath, proposalDir, "proposal artifact directory"); err != nil {
		return nil, err
	}
	candidate, candidateRaw, repoTaskPack, identity, err := revalidateSoftwareDevPromotionCandidate(repoRoot, rootPath)
	if err != nil {
		return nil, err
	}
	if stringValue(mapValue(candidate["eligibility"])["status"]) != "eligible" {
		return nil, errors.New("promotion candidate is not eligible")
	}
	delta := mapValue(candidate["promotable_soft_layer_delta"])
	if err := validatePromotionPatchDelta(delta); err != nil {
		return nil, err
	}

	repoRootPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("promotion consumer repo root is invalid: %w", err)
	}
	taskPackAbsolute := filepath.Join(repoRootPath, filepath.FromSlash(identity.TaskPackPath))
	taskPackBefore, err := os.ReadFile(taskPackAbsolute)
	if err != nil {
		return nil, fmt.Errorf("read promotion task pack target: %w", err)
	}
	roleDelta := mapValue(delta["roles"])
	inlineRoles := roleDelta
	if identity.SiblingRoles != "" {
		inlineRoles = nil
	}
	taskPackAfter, err := patchPromotionTaskPack(taskPackBefore, identity.StepID, mapValue(delta["task_pack_step"]), inlineRoles)
	if err != nil {
		return nil, fmt.Errorf("patch promotion task pack: %w", err)
	}
	if err := validatePromotionWorkflowBytes(proposalDir, taskPackAfter); err != nil {
		return nil, fmt.Errorf("patched promotion task pack is invalid: %w", err)
	}

	targets := []promotionTargetImage{{
		Kind:     "selected_task_pack_step",
		Path:     identity.TaskPackPath,
		StepID:   identity.StepID,
		Absolute: taskPackAbsolute,
		Before:   taskPackBefore,
		After:    taskPackAfter,
	}}
	manifestTargets := []promotionPatchTarget{{
		Kind:            "selected_task_pack_step",
		Path:            identity.TaskPackPath,
		StepID:          identity.StepID,
		BeforeSHA256:    promotionDigest(taskPackBefore),
		AfterSHA256:     promotionDigest(taskPackAfter),
		AssignedChanges: taskPackAssignedPromotionChanges(mapValue(delta["task_pack_step"]), inlineRoles),
	}}

	if identity.SiblingRoles != "" && len(roleDelta) > 0 {
		siblingAbsolute := filepath.Join(repoRootPath, filepath.FromSlash(identity.SiblingRoles))
		siblingBefore, readErr := os.ReadFile(siblingAbsolute)
		if readErr != nil {
			return nil, fmt.Errorf("read promotion sibling roles target: %w", readErr)
		}
		siblingAfter, patchErr := patchPromotionAgentsDocument(siblingBefore, roleDelta)
		if patchErr != nil {
			return nil, fmt.Errorf("patch promotion sibling roles: %w", patchErr)
		}
		if validateErr := validatePromotionAgentsDocument(siblingAfter); validateErr != nil {
			return nil, fmt.Errorf("patched promotion sibling roles are invalid: %w", validateErr)
		}
		targets = append(targets, promotionTargetImage{
			Kind: "owned_sibling_agents", Path: identity.SiblingRoles, Absolute: siblingAbsolute,
			Before: siblingBefore, After: siblingAfter,
		})
		manifestTargets = append(manifestTargets, promotionPatchTarget{
			Kind: "owned_sibling_agents", Path: identity.SiblingRoles,
			BeforeSHA256: promotionDigest(siblingBefore), AfterSHA256: promotionDigest(siblingAfter),
			AssignedChanges: map[string]any{"roles": copyMap(roleDelta)},
		})
	}

	changed := false
	for _, target := range targets {
		if !bytes.Equal(target.Before, target.After) {
			changed = true
		}
	}
	if !changed {
		return nil, errors.New("promotion patch contains no target changes")
	}
	patchBytes := renderPromotionUnifiedPatch(targets)
	manifest := promotionPatchManifest{
		ContractVersion:  promotionPatchContractVersion,
		SourceCandidate:  promotionSourceCandidate{Path: "proposal/promotion-candidate.json", SHA256: promotionDigest(candidateRaw)},
		MutableSurface:   promotionMutableSurface{TaskPackPath: identity.TaskPackPath, StepID: identity.StepID},
		ApprovalRequired: true,
		Patch:            promotionPatchReference{Path: "proposal/promotion.patch", Format: "unified-diff", SHA256: promotionDigest(patchBytes)},
		Targets:          manifestTargets,
		WorkspaceMutation: promotionMutationStatement{
			Performed:    false,
			Confirmation: "Patch generation read the exact consumer targets and wrote only proposal review artifacts under the run artifact root.",
		},
	}
	manifestBytes, err := marshalPromotionJSON(manifest)
	if err != nil {
		return nil, err
	}
	_ = repoTaskPack
	return &compiledPromotionPatch{Manifest: manifest, ManifestBytes: manifestBytes, PatchBytes: patchBytes, Targets: targets}, nil
}

func revalidateSoftwareDevPromotionCandidate(repoRoot, artifactRoot string) (map[string]any, []byte, map[string]any, softwareDevPromotionIdentity, error) {
	candidateRaw, err := readPromotionRegularFile(artifactRoot, filepath.Join(artifactRoot, "proposal", "promotion-candidate.json"), "proposal/promotion-candidate.json")
	if err != nil {
		return nil, nil, nil, softwareDevPromotionIdentity{}, err
	}
	candidate := map[string]any{}
	if err := json.Unmarshal(candidateRaw, &candidate); err != nil {
		return nil, nil, nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion candidate is malformed JSON: %w", err)
	}
	if stringValue(candidate["contract_version"]) != "software.dev.promotion-candidate/v1" {
		return nil, nil, nil, softwareDevPromotionIdentity{}, errors.New("promotion candidate contract version is unsupported")
	}
	mutable := mapValue(candidate["mutable_surface"])
	taskPackPath := stringValue(mutable["task_pack_path"])
	stepID := stringValue(mutable["step_id"])
	repoTaskPack, identity, err := loadSoftwareDevPromotionIdentity(repoRoot, taskPackPath, stepID)
	if err != nil {
		return nil, nil, nil, softwareDevPromotionIdentity{}, err
	}
	expected, err := evaluatePromotionCandidateFromEvidence(repoRoot, repoTaskPack, identity, artifactRoot)
	if err != nil {
		return nil, nil, nil, softwareDevPromotionIdentity{}, err
	}
	expectedRaw, err := marshalPromotionJSON(expected)
	if err != nil {
		return nil, nil, nil, softwareDevPromotionIdentity{}, err
	}
	if !bytes.Equal(candidateRaw, expectedRaw) {
		return nil, nil, nil, softwareDevPromotionIdentity{}, errors.New("promotion candidate does not exactly match the current target and source evidence")
	}
	return candidate, candidateRaw, repoTaskPack, identity, nil
}

func evaluatePromotionCandidateFromEvidence(repoRoot string, repoTaskPack map[string]any, identity softwareDevPromotionIdentity, artifactRoot string) (map[string]any, error) {
	rawProposal, rawRelativePath, err := readPromotionRawProposal(artifactRoot)
	if err != nil {
		return nil, err
	}
	parsed, err := parsePlannerProposal(rawProposal)
	if err != nil {
		return nil, fmt.Errorf("promotion source proposal is invalid: %w", err)
	}
	normalized, err := readPromotionJSONMap(artifactRoot, "proposal/normalized.json")
	if err != nil {
		return nil, err
	}
	metadata, err := readPromotionJSONMap(artifactRoot, "proposal/metadata.json")
	if err != nil {
		return nil, err
	}
	verification, err := readPromotionJSONMap(artifactRoot, "verify/result.json")
	if err != nil {
		return nil, err
	}
	if mustJSON(parsed.Declaration, nil) != mustJSON(normalized, nil) {
		return nil, errors.New("promotion proposal raw and normalized artifacts are inconsistent")
	}
	if err := validatePromotionMetadata(parsed, metadata, identity); err != nil {
		return nil, err
	}
	if err := validatePromotionVerificationArtifact(verification); err != nil {
		return nil, err
	}
	candidate := evaluateSoftwareDevPromotion(repoTaskPack, normalized, metadata, verification, identity, rawRelativePath)
	if err := addPromotionCandidateTargetDigests(candidate, repoRoot, identity); err != nil {
		return nil, err
	}
	return candidate, nil
}

func addPromotionCandidateTargetDigests(candidate map[string]any, repoRoot string, identity softwareDevPromotionIdentity) error {
	rootPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("promotion consumer repo root is invalid: %w", err)
	}
	paths := []string{identity.TaskPackPath}
	if identity.SiblingRoles != "" {
		paths = append(paths, identity.SiblingRoles)
	}
	digests := make([]any, 0, len(paths))
	for _, path := range paths {
		absolute := filepath.Join(rootPath, filepath.FromSlash(path))
		raw, readErr := os.ReadFile(absolute)
		if readErr != nil {
			return fmt.Errorf("read promotion target source digest for %s: %w", path, readErr)
		}
		digests = append(digests, map[string]any{"path": path, "sha256": promotionDigest(raw)})
	}
	candidate["target_source_digests"] = digests
	return nil
}

func validatePromotionPatchDelta(delta map[string]any) error {
	if err := requirePromotionKeys(delta, "promotable_soft_layer_delta", "task_pack_step", "roles"); err != nil {
		return err
	}
	step := mapValue(delta["task_pack_step"])
	if err := requirePromotionKeys(step, "promotable_soft_layer_delta.task_pack_step", "startup_prompt", "constraints", "orchestration"); err != nil {
		return err
	}
	if err := validatePromotionOptionalString(step, "startup_prompt", "task_pack_step.startup_prompt"); err != nil {
		return err
	}
	if err := validatePromotionStringList(step["constraints"], "task_pack_step.constraints"); err != nil {
		return err
	}
	orchestration := mapValue(step["orchestration"])
	if err := requirePromotionKeys(orchestration, "task_pack_step.orchestration", "constraints", "plan", "merge", "verify", "apply"); err != nil {
		return err
	}
	if err := validatePromotionStringList(orchestration["constraints"], "task_pack_step.orchestration.constraints"); err != nil {
		return err
	}
	plan := mapValue(orchestration["plan"])
	if err := requirePromotionKeys(plan, "task_pack_step.orchestration.plan", "goal", "steps", "constraints"); err != nil {
		return err
	}
	if err := validatePromotionOptionalString(plan, "goal", "task_pack_step.orchestration.plan.goal"); err != nil {
		return err
	}
	for _, field := range []string{"steps", "constraints"} {
		if err := validatePromotionStringList(plan[field], "task_pack_step.orchestration.plan."+field); err != nil {
			return err
		}
	}
	merge := mapValue(orchestration["merge"])
	if err := requirePromotionKeys(merge, "task_pack_step.orchestration.merge", "title"); err != nil {
		return err
	}
	if err := validatePromotionOptionalScalar(merge, "title", "task_pack_step.orchestration.merge.title"); err != nil {
		return err
	}
	verify := mapValue(orchestration["verify"])
	if err := requirePromotionKeys(verify, "task_pack_step.orchestration.verify", "next_action_default"); err != nil {
		return err
	}
	if err := validatePromotionOptionalScalar(verify, "next_action_default", "task_pack_step.orchestration.verify.next_action_default"); err != nil {
		return err
	}
	apply := mapValue(orchestration["apply"])
	if err := requirePromotionKeys(apply, "task_pack_step.orchestration.apply", "required_artifacts"); err != nil {
		return err
	}
	if err := validatePromotionStringList(apply["required_artifacts"], "task_pack_step.orchestration.apply.required_artifacts"); err != nil {
		return err
	}
	roles := mapValue(delta["roles"])
	roleIDs := make([]string, 0, len(roles))
	for id := range roles {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	for _, id := range roleIDs {
		if strings.TrimSpace(id) == "" {
			return errors.New("promotable role id must not be empty")
		}
		role := mapValue(roles[id])
		if err := requirePromotionKeys(role, fmt.Sprintf("roles[%q]", id), "role", "constraints"); err != nil {
			return err
		}
		if err := validatePromotionOptionalString(role, "role", fmt.Sprintf("roles[%q].role", id)); err != nil {
			return err
		}
		if err := validatePromotionStringList(role["constraints"], fmt.Sprintf("roles[%q].constraints", id)); err != nil {
			return err
		}
	}
	return nil
}

func requirePromotionKeys(data map[string]any, field string, allowed ...string) error {
	allow := map[string]bool{}
	for _, key := range allowed {
		allow[key] = true
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !allow[key] {
			return fmt.Errorf("%s contains forbidden promotion field %q", field, key)
		}
	}
	return nil
}

func validatePromotionOptionalString(data map[string]any, key, field string) error {
	value, ok := data[key]
	if !ok {
		return nil
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fmt.Errorf("%s must be a non-empty string", field)
	}
	return nil
}

func validatePromotionOptionalScalar(data map[string]any, key, field string) error {
	value, ok := data[key]
	if !ok {
		return nil
	}
	if !isContractScalar(value) {
		return fmt.Errorf("%s must be a scalar", field)
	}
	return nil
}

func validatePromotionStringList(value any, field string) error {
	if value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%s must be an array of non-empty strings", field)
	}
	seen := map[string]bool{}
	for _, raw := range items {
		text, ok := raw.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must be an array of non-empty strings", field)
		}
		if seen[text] {
			return fmt.Errorf("%s contains duplicate value %q", field, text)
		}
		seen[text] = true
	}
	return nil
}

func taskPackAssignedPromotionChanges(step, inlineRoles map[string]any) map[string]any {
	assigned := map[string]any{"task_pack_step": copyMap(step)}
	if len(inlineRoles) > 0 {
		assigned["inline_roles"] = copyMap(inlineRoles)
	}
	return assigned
}

func promotionDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", digest[:])
}

func marshalPromotionJSON(payload any) ([]byte, error) {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func patchPromotionTaskPack(raw []byte, stepID string, stepDelta, inlineRoles map[string]any) ([]byte, error) {
	document, err := parsePromotionYAML(raw)
	if err != nil {
		return nil, err
	}
	root, err := promotionDocumentMapping(document)
	if err != nil {
		return nil, err
	}
	steps, err := promotionMappingValue(root, "steps", yaml.SequenceNode, true)
	if err != nil {
		return nil, err
	}
	var selected *yaml.Node
	matches := 0
	selectedIndex := -1
	for index, rawStep := range steps.Content {
		if rawStep.Kind != yaml.MappingNode {
			continue
		}
		id, _ := promotionMappingValue(rawStep, "id", yaml.ScalarNode, false)
		if id != nil && id.Value == stepID {
			selected = rawStep
			selectedIndex = index
			matches++
		}
	}
	if matches == 0 {
		return nil, fmt.Errorf("promotion task-pack step id %q was not found", stepID)
	}
	if matches > 1 {
		return nil, fmt.Errorf("promotion task-pack step id %q is ambiguous (%d matches)", stepID, matches)
	}
	agent, err := promotionEnsureMapping(selected, "agent")
	if err != nil {
		return nil, err
	}
	if value, ok := stepDelta["startup_prompt"]; ok {
		if err := promotionAppendScalarGuidance(agent, "startup_prompt", "Promoted startup guidance", stringValue(value)); err != nil {
			return nil, err
		}
	}
	if err := promotionAppendGuidanceString(agent, "startup_prompt", "Promoted root constraints", stringList(stepDelta["constraints"])); err != nil {
		return nil, err
	}
	orchestration, err := promotionEnsureMapping(agent, "orchestration")
	if err != nil {
		return nil, err
	}
	orchestrationDelta := mapValue(stepDelta["orchestration"])
	if err := promotionAppendGuidanceString(agent, "startup_prompt", "Promoted orchestration constraints", stringList(orchestrationDelta["constraints"])); err != nil {
		return nil, err
	}
	if planDelta := mapValue(orchestrationDelta["plan"]); len(planDelta) > 0 {
		plan, ensureErr := promotionEnsureMapping(orchestration, "plan")
		if ensureErr != nil {
			return nil, ensureErr
		}
		if value, ok := planDelta["goal"]; ok {
			existingGoal, scalarErr := promotionStringScalar(plan, "goal")
			if scalarErr != nil {
				return nil, scalarErr
			}
			proposedGoal := stringValue(value)
			if existingGoal == "" {
				promotionSetScalar(plan, "goal", proposedGoal)
			} else if existingGoal != proposedGoal {
				if appendErr := promotionAppendStrings(plan, "steps", []string{"Promoted plan goal: " + proposedGoal}); appendErr != nil {
					return nil, appendErr
				}
			}
		}
		for _, key := range []string{"steps", "constraints"} {
			targetKey := key
			if key == "constraints" {
				targetKey = "steps"
			}
			if appendErr := promotionAppendStrings(plan, targetKey, stringList(planDelta[key])); appendErr != nil {
				return nil, appendErr
			}
		}
	}
	for _, key := range []string{"merge", "verify"} {
		if scalarDelta := mapValue(orchestrationDelta[key]); len(scalarDelta) > 0 {
			target, ensureErr := promotionEnsureMapping(orchestration, key)
			if ensureErr != nil {
				return nil, ensureErr
			}
			keys := make([]string, 0, len(scalarDelta))
			for field := range scalarDelta {
				keys = append(keys, field)
			}
			sort.Strings(keys)
			for _, field := range keys {
				value := stringValue(scalarDelta[field])
				existing, scalarErr := promotionStringScalar(target, field)
				if scalarErr != nil {
					return nil, scalarErr
				}
				if existing == "" {
					promotionSetScalar(target, field, value)
					continue
				}
				if existing == value {
					continue
				}
				if key == "merge" && field == "title" {
					if appendErr := promotionAppendStrings(target, "summary_points", []string{"Promoted merge guidance: " + value}); appendErr != nil {
						return nil, appendErr
					}
				} else if appendErr := promotionAppendScalarGuidance(target, field, "Promoted verification guidance", value); appendErr != nil {
					return nil, appendErr
				}
			}
		}
	}
	if applyDelta := mapValue(orchestrationDelta["apply"]); len(applyDelta) > 0 {
		apply, ensureErr := promotionEnsureMapping(orchestration, "apply")
		if ensureErr != nil {
			return nil, ensureErr
		}
		if appendErr := promotionAppendStrings(apply, "required_artifacts", stringList(applyDelta["required_artifacts"])); appendErr != nil {
			return nil, appendErr
		}
	}
	if len(inlineRoles) > 0 {
		agents, ensureErr := promotionEnsureMapping(orchestration, "agents")
		if ensureErr != nil {
			return nil, ensureErr
		}
		if mergeErr := promotionMergeRoles(agents, inlineRoles); mergeErr != nil {
			return nil, mergeErr
		}
	}
	return renderSelectedPromotionStep(raw, root, steps, selected, selectedIndex)
}

func patchPromotionAgentsDocument(raw []byte, roles map[string]any) ([]byte, error) {
	document, err := parsePromotionYAML(raw)
	if err != nil {
		return nil, err
	}
	root, err := promotionDocumentMapping(document)
	if err != nil {
		return nil, err
	}
	agents, err := promotionMappingValue(root, "agents", yaml.MappingNode, true)
	if err != nil {
		return nil, err
	}
	if err := promotionMergeRoles(agents, roles); err != nil {
		return nil, err
	}
	return encodePromotionYAML(document)
}

func promotionMergeRoles(agents *yaml.Node, roles map[string]any) error {
	roleIDs := make([]string, 0, len(roles))
	for id := range roles {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	for _, id := range roleIDs {
		role, err := promotionEnsureMapping(agents, id)
		if err != nil {
			return err
		}
		delta := mapValue(roles[id])
		if value, ok := delta["role"]; ok {
			if err := promotionAppendScalarGuidance(role, "role", "Promoted role guidance", stringValue(value)); err != nil {
				return err
			}
		}
		if err := promotionAppendStrings(role, "constraints", stringList(delta["constraints"])); err != nil {
			return err
		}
	}
	return nil
}

func parsePromotionYAML(raw []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	document := &yaml.Node{}
	if err := decoder.Decode(document); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	extra := &yaml.Node{}
	if err := decoder.Decode(extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("YAML document must contain exactly one document")
		}
		return nil, fmt.Errorf("read YAML document: %w", err)
	}
	return document, nil
}

func promotionDocumentMapping(document *yaml.Node) (*yaml.Node, error) {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("YAML document root must be a mapping")
	}
	return document.Content[0], nil
}

func promotionMappingValue(mapping *yaml.Node, key string, kind yaml.Kind, required bool) (*yaml.Node, error) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("YAML parent for %q is not a mapping", key)
	}
	var found *yaml.Node
	matches := 0
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			found = mapping.Content[index+1]
			matches++
		}
	}
	if matches > 1 {
		return nil, fmt.Errorf("YAML mapping key %q is ambiguous (%d matches)", key, matches)
	}
	if matches == 0 {
		if required {
			return nil, fmt.Errorf("YAML mapping key %q is required", key)
		}
		return nil, nil
	}
	if kind != 0 && found.Kind != kind {
		return nil, fmt.Errorf("YAML mapping key %q has the wrong structure", key)
	}
	return found, nil
}

func promotionEnsureMapping(parent *yaml.Node, key string) (*yaml.Node, error) {
	found, err := promotionMappingValue(parent, key, 0, false)
	if err != nil {
		return nil, err
	}
	if found != nil {
		if found.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("YAML mapping key %q has the wrong structure", key)
		}
		return found, nil
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, keyNode, valueNode)
	return valueNode, nil
}

func promotionSetScalar(parent *yaml.Node, key string, value any) {
	found, _ := promotionMappingValue(parent, key, 0, false)
	if found == nil {
		found = &yaml.Node{}
		parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, found)
	}
	style := found.Style
	_ = found.Encode(value)
	if found.Kind == yaml.ScalarNode {
		found.Style = style
	}
}

func promotionAppendStrings(parent *yaml.Node, key string, additions []string) error {
	if len(additions) == 0 {
		return nil
	}
	sequence, err := promotionMappingValue(parent, key, 0, false)
	if err != nil {
		return err
	}
	if sequence == nil {
		sequence = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, sequence)
	}
	if sequence.Kind != yaml.SequenceNode {
		return fmt.Errorf("YAML mapping key %q must be a sequence", key)
	}
	existing := map[string]bool{}
	for _, node := range sequence.Content {
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return fmt.Errorf("YAML mapping key %q must contain only strings", key)
		}
		existing[node.Value] = true
	}
	for _, addition := range additions {
		if existing[addition] {
			continue
		}
		sequence.Content = append(sequence.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: addition})
		existing[addition] = true
	}
	return nil
}

func promotionAppendGuidanceString(parent *yaml.Node, key, heading string, additions []string) error {
	if len(additions) == 0 {
		return nil
	}
	found, err := promotionMappingValue(parent, key, 0, false)
	if err != nil {
		return err
	}
	existing := ""
	if found != nil {
		if found.Kind != yaml.ScalarNode || found.Tag != "!!str" {
			return fmt.Errorf("YAML mapping key %q must be a string", key)
		}
		existing = strings.TrimSpace(found.Value)
	}
	newItems := []string{}
	for _, addition := range additions {
		bullet := "- " + addition
		if !strings.Contains("\n"+existing+"\n", "\n"+bullet+"\n") {
			newItems = append(newItems, bullet)
		}
	}
	if len(newItems) == 0 {
		return nil
	}
	block := heading + ":\n" + strings.Join(newItems, "\n")
	if existing != "" {
		block = existing + "\n\n" + block
	}
	promotionSetScalar(parent, key, block)
	return nil
}

func promotionAppendScalarGuidance(parent *yaml.Node, key, heading, addition string) error {
	addition = strings.TrimSpace(addition)
	if addition == "" {
		return nil
	}
	existing, err := promotionStringScalar(parent, key)
	if err != nil {
		return err
	}
	if existing == addition || strings.Contains(existing, heading+":\n"+addition) {
		return nil
	}
	combined := addition
	if existing != "" {
		combined = existing + "\n\n" + heading + ":\n" + addition
	}
	promotionSetScalar(parent, key, combined)
	return nil
}

func promotionStringScalar(parent *yaml.Node, key string) (string, error) {
	found, err := promotionMappingValue(parent, key, 0, false)
	if err != nil {
		return "", err
	}
	if found == nil {
		return "", nil
	}
	if found.Kind != yaml.ScalarNode || found.Tag != "!!str" {
		return "", fmt.Errorf("YAML mapping key %q must be a string", key)
	}
	return strings.TrimSpace(found.Value), nil
}

func renderSelectedPromotionStep(raw []byte, root, steps, selected *yaml.Node, selectedIndex int) ([]byte, error) {
	if selected.Line <= 0 {
		return nil, errors.New("selected task-pack step has no source line identity")
	}
	newline := "\n"
	if bytes.Contains(raw, []byte("\r\n")) {
		newline = "\r\n"
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start := selected.Line - 1
	if start < 0 || start >= len(lines) {
		return nil, errors.New("selected task-pack step source line is invalid")
	}
	end := len(lines)
	if selectedIndex+1 < len(steps.Content) {
		end = steps.Content[selectedIndex+1].Line - 1
	} else {
		for index := 0; index+1 < len(root.Content); index += 2 {
			if root.Content[index+1] == steps && index+2 < len(root.Content) {
				end = root.Content[index+2].Line - 1
				break
			}
		}
	}
	if end <= start || end > len(lines) {
		return nil, errors.New("selected task-pack step source range is invalid")
	}
	for end > start+1 {
		trimmed := strings.TrimSpace(lines[end-1])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		end--
	}
	originalLine := strings.TrimSuffix(lines[start], "\r")
	dash := strings.Index(originalLine, "-")
	if dash < 0 || strings.TrimSpace(originalLine[:dash]) != "" {
		return nil, errors.New("selected task-pack step is not a canonical YAML sequence item")
	}
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{selected}}
	var rendered bytes.Buffer
	encoder := yaml.NewEncoder(&rendered)
	encoder.SetIndent(2)
	if err := encoder.Encode(sequence); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	encodedLines := strings.Split(strings.TrimSuffix(rendered.String(), "\n"), "\n")
	prefix := strings.Repeat(" ", dash)
	for index := range encodedLines {
		encodedLines[index] = prefix + encodedLines[index]
	}
	resultLines := make([]string, 0, len(lines)-(end-start)+len(encodedLines))
	resultLines = append(resultLines, lines[:start]...)
	resultLines = append(resultLines, encodedLines...)
	resultLines = append(resultLines, lines[end:]...)
	return []byte(strings.ReplaceAll(strings.Join(resultLines, "\n"), "\n", newline)), nil
}

func encodePromotionYAML(document *yaml.Node) ([]byte, error) {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func validatePromotionWorkflowBytes(directory string, raw []byte) error {
	temporary, err := os.CreateTemp(directory, ".promotion-workflow-*.yml")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer os.Remove(path)
	if _, err = temporary.Write(raw); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return promotionValidateWorkflow(path)
}

func validatePromotionAgentsDocument(raw []byte) error {
	document, err := parsePromotionYAML(raw)
	if err != nil {
		return err
	}
	root, err := promotionDocumentMapping(document)
	if err != nil {
		return err
	}
	agents, err := promotionMappingValue(root, "agents", yaml.MappingNode, true)
	if err != nil {
		return err
	}
	if len(agents.Content)%2 != 0 {
		return errors.New("agents mapping is malformed")
	}
	return nil
}

type promotionDiffLine struct {
	kind   byte
	text   string
	oldPos int
	newPos int
}

func renderPromotionUnifiedPatch(targets []promotionTargetImage) []byte {
	var output strings.Builder
	for _, target := range targets {
		if bytes.Equal(target.Before, target.After) {
			continue
		}
		output.WriteString("--- a/")
		output.WriteString(target.Path)
		output.WriteByte('\n')
		output.WriteString("+++ b/")
		output.WriteString(target.Path)
		output.WriteByte('\n')
		output.WriteString(renderPromotionFileDiff(target.Before, target.After))
	}
	return []byte(output.String())
}

func renderPromotionFileDiff(before, after []byte) string {
	oldLines := splitPromotionDiffLines(before)
	newLines := splitPromotionDiffLines(after)
	lcs := make([][]int, len(oldLines)+1)
	for index := range lcs {
		lcs[index] = make([]int, len(newLines)+1)
	}
	for oldIndex := len(oldLines) - 1; oldIndex >= 0; oldIndex-- {
		for newIndex := len(newLines) - 1; newIndex >= 0; newIndex-- {
			if oldLines[oldIndex] == newLines[newIndex] {
				lcs[oldIndex][newIndex] = lcs[oldIndex+1][newIndex+1] + 1
			} else if lcs[oldIndex+1][newIndex] >= lcs[oldIndex][newIndex+1] {
				lcs[oldIndex][newIndex] = lcs[oldIndex+1][newIndex]
			} else {
				lcs[oldIndex][newIndex] = lcs[oldIndex][newIndex+1]
			}
		}
	}
	edits := []promotionDiffLine{}
	oldIndex, newIndex := 0, 0
	for oldIndex < len(oldLines) || newIndex < len(newLines) {
		switch {
		case oldIndex < len(oldLines) && newIndex < len(newLines) && oldLines[oldIndex] == newLines[newIndex]:
			edits = append(edits, promotionDiffLine{kind: ' ', text: oldLines[oldIndex], oldPos: oldIndex + 1, newPos: newIndex + 1})
			oldIndex++
			newIndex++
		case oldIndex < len(oldLines) && (newIndex == len(newLines) || lcs[oldIndex+1][newIndex] >= lcs[oldIndex][newIndex+1]):
			edits = append(edits, promotionDiffLine{kind: '-', text: oldLines[oldIndex], oldPos: oldIndex + 1, newPos: newIndex + 1})
			oldIndex++
		default:
			edits = append(edits, promotionDiffLine{kind: '+', text: newLines[newIndex], oldPos: oldIndex + 1, newPos: newIndex + 1})
			newIndex++
		}
	}
	changes := []int{}
	for index, edit := range edits {
		if edit.kind != ' ' {
			changes = append(changes, index)
		}
	}
	if len(changes) == 0 {
		return ""
	}
	type diffRange struct{ start, end int }
	ranges := []diffRange{}
	for _, change := range changes {
		start := change - 3
		if start < 0 {
			start = 0
		}
		end := change + 4
		if end > len(edits) {
			end = len(edits)
		}
		if len(ranges) > 0 && start <= ranges[len(ranges)-1].end {
			if end > ranges[len(ranges)-1].end {
				ranges[len(ranges)-1].end = end
			}
		} else {
			ranges = append(ranges, diffRange{start: start, end: end})
		}
	}
	var output strings.Builder
	for _, hunk := range ranges {
		oldCount, newCount := 0, 0
		for _, edit := range edits[hunk.start:hunk.end] {
			if edit.kind != '+' {
				oldCount++
			}
			if edit.kind != '-' {
				newCount++
			}
		}
		first := edits[hunk.start]
		oldStart, newStart := first.oldPos, first.newPos
		if oldCount == 0 && oldStart > 0 {
			oldStart--
		}
		if newCount == 0 && newStart > 0 {
			newStart--
		}
		fmt.Fprintf(&output, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, edit := range edits[hunk.start:hunk.end] {
			output.WriteByte(edit.kind)
			output.WriteString(edit.text)
			if !strings.HasSuffix(edit.text, "\n") {
				output.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return output.String()
}

func splitPromotionDiffLines(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	parts := strings.SplitAfter(text, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func writePromotionArtifactSetAtomic(directory string, files map[string][]byte) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	stageDir, err := os.MkdirTemp(directory, ".promotion-patch-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)
	names := make([]string, 0, len(files))
	for name, content := range files {
		names = append(names, name)
		if err := os.WriteFile(filepath.Join(stageDir, name), content, 0o644); err != nil {
			return err
		}
	}
	sort.Strings(names)
	type previousFile struct {
		existed bool
		content []byte
	}
	previous := map[string]previousFile{}
	committed := []string{}
	for _, name := range names {
		target := filepath.Join(directory, name)
		if info, statErr := os.Lstat(target); statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("promotion artifact target is not a regular file: %s", name)
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil {
				return readErr
			}
			previous[name] = previousFile{existed: true, content: content}
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		if err := os.Rename(filepath.Join(stageDir, name), target); err != nil {
			for index := len(committed) - 1; index >= 0; index-- {
				committedName := committed[index]
				prior := previous[committedName]
				committedTarget := filepath.Join(directory, committedName)
				if prior.existed {
					_ = writePromotionBytesAtomic(committedTarget, prior.content)
				} else {
					_ = os.Remove(committedTarget)
				}
			}
			return err
		}
		committed = append(committed, name)
	}
	return nil
}

func writePromotionBytesAtomic(path string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".promotion-write-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(content); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func applySoftwareDevPromotionPatch(repoRoot, artifactRoot, approvalPath string) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	resultPath := filepath.Join(rootPath, "proposal", "promotion-apply-result.json")
	resultBase := map[string]any{
		"contract_version":                promotionApplyContractVersion,
		"status":                          "failed",
		"approved_patch_digest":           "",
		"changed_target_paths":            []any{},
		"targets":                         []any{},
		"rollback":                        map[string]any{"status": "not_needed"},
		"no_other_consumer_paths_changed": true,
	}
	fail := func(status, message string, rollback map[string]any) error {
		result := copyMap(resultBase)
		result["status"] = status
		result["reason"] = message
		if rollback != nil {
			result["rollback"] = rollback
		}
		_ = writeJSONFileAtomic(resultPath, result)
		return errors.New(message)
	}

	manifestRaw, err := readPromotionRegularFile(rootPath, filepath.Join(rootPath, "proposal", "promotion-patch.json"), "proposal/promotion-patch.json")
	if err != nil {
		return fail("failed", err.Error(), nil)
	}
	manifest := promotionPatchManifest{}
	if err := decodePromotionJSONStrict(manifestRaw, &manifest); err != nil {
		return fail("failed", "promotion patch manifest is malformed: "+err.Error(), nil)
	}
	if err := validatePromotionPatchManifestShape(manifest); err != nil {
		return fail("failed", err.Error(), nil)
	}
	patchRaw, err := readPromotionRegularFile(rootPath, filepath.Join(rootPath, "proposal", "promotion.patch"), "proposal/promotion.patch")
	if err != nil {
		return fail("failed", err.Error(), nil)
	}
	if promotionDigest(patchRaw) != manifest.Patch.SHA256 {
		return fail("failed", "promotion patch digest does not match the manifest", nil)
	}
	candidateRaw, err := readPromotionRegularFile(rootPath, filepath.Join(rootPath, "proposal", "promotion-candidate.json"), "proposal/promotion-candidate.json")
	if err != nil {
		return fail("failed", err.Error(), nil)
	}
	if promotionDigest(candidateRaw) != manifest.SourceCandidate.SHA256 {
		return fail("failed", "promotion candidate digest does not match the manifest", nil)
	}
	approval, err := readPromotionApproval(rootPath, approvalPath)
	if err != nil {
		return fail("denied", err.Error(), nil)
	}
	if err := validatePromotionApproval(approval, manifest); err != nil {
		return fail("denied", err.Error(), nil)
	}
	resultBase["approved_patch_digest"] = manifest.Patch.SHA256

	currentTargets, err := inspectPromotionManifestTargets(repoRoot, manifest)
	if err != nil {
		return fail("failed", err.Error(), nil)
	}
	allAfter := true
	allBefore := true
	for index, target := range currentTargets {
		digest := promotionDigest(target.Before)
		if digest != manifest.Targets[index].AfterSHA256 {
			allAfter = false
		}
		if digest != manifest.Targets[index].BeforeSHA256 {
			allBefore = false
		}
	}
	if allAfter {
		result := copyMap(resultBase)
		result["status"] = "already_applied"
		result["reason"] = "all exact approved targets already match their after digests"
		result["targets"] = promotionResultTargets(manifest.Targets)
		if err := writeJSONFileAtomic(resultPath, result); err != nil {
			return err
		}
		return nil
	}
	if !allBefore {
		return fail("failed", "promotion target before-digests are stale or partially applied", nil)
	}

	compiled, err := compileSoftwareDevPromotionPatch(repoRoot, artifactRoot)
	if err != nil {
		return fail("failed", err.Error(), nil)
	}
	if !bytes.Equal(compiled.ManifestBytes, manifestRaw) || !bytes.Equal(compiled.PatchBytes, patchRaw) {
		return fail("failed", "promotion patch artifacts do not exactly match regenerated candidate and targets", nil)
	}
	if err := validatePromotionApproval(approval, compiled.Manifest); err != nil {
		return fail("denied", err.Error(), nil)
	}

	changed, rollback, applyErr := applyPromotionTargetsTransactionally(compiled.Targets)
	if applyErr != nil {
		result := copyMap(resultBase)
		result["status"] = "failed"
		result["reason"] = applyErr.Error()
		result["changed_target_paths"] = anySlice(changed)
		result["targets"] = promotionResultTargets(manifest.Targets)
		result["rollback"] = rollback
		result["no_other_consumer_paths_changed"] = rollback["status"] == "succeeded"
		_ = writeJSONFileAtomic(resultPath, result)
		return applyErr
	}
	result := copyMap(resultBase)
	result["status"] = "applied"
	result["changed_target_paths"] = anySlice(changed)
	result["targets"] = promotionResultTargets(manifest.Targets)
	if err := writeJSONFileAtomic(resultPath, result); err != nil {
		return err
	}
	return nil
}

func decodePromotionJSONStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("JSON document must contain exactly one value")
		}
		return err
	}
	return nil
}

func validatePromotionPatchManifestShape(manifest promotionPatchManifest) error {
	if manifest.ContractVersion != promotionPatchContractVersion {
		return errors.New("promotion patch manifest contract version is unsupported")
	}
	if !manifest.ApprovalRequired {
		return errors.New("promotion patch manifest must require approval")
	}
	if manifest.SourceCandidate.Path != "proposal/promotion-candidate.json" || !reResponseHash.MatchString(manifest.SourceCandidate.SHA256) {
		return errors.New("promotion patch manifest has an invalid source candidate binding")
	}
	if manifest.Patch.Path != "proposal/promotion.patch" || manifest.Patch.Format != "unified-diff" || !reResponseHash.MatchString(manifest.Patch.SHA256) {
		return errors.New("promotion patch manifest has an invalid textual patch binding")
	}
	if manifest.MutableSurface.TaskPackPath == "" || manifest.MutableSurface.StepID == "" {
		return errors.New("promotion patch manifest has no exact mutable surface")
	}
	if manifest.WorkspaceMutation.Performed {
		return errors.New("promotion patch manifest does not confirm mutation-free generation")
	}
	if len(manifest.Targets) < 1 || len(manifest.Targets) > 2 {
		return errors.New("promotion patch manifest must contain one task pack and at most one sibling roles target")
	}
	seen := map[string]bool{}
	for index, target := range manifest.Targets {
		if target.Path == "" || seen[target.Path] || !reResponseHash.MatchString(target.BeforeSHA256) || !reResponseHash.MatchString(target.AfterSHA256) {
			return errors.New("promotion patch manifest has an invalid or duplicate target binding")
		}
		seen[target.Path] = true
		if index == 0 {
			if target.Kind != "selected_task_pack_step" || target.Path != manifest.MutableSurface.TaskPackPath || target.StepID != manifest.MutableSurface.StepID {
				return errors.New("promotion patch manifest task-pack target does not match the mutable surface")
			}
		} else if target.Kind != "owned_sibling_agents" || target.StepID != "" {
			return errors.New("promotion patch manifest sibling target is invalid")
		}
	}
	return nil
}

func readPromotionApproval(artifactRoot, approvalPath string) (promotionApproval, error) {
	if strings.TrimSpace(approvalPath) == "" {
		return promotionApproval{}, errors.New("explicit promotion approval artifact is required")
	}
	absolute, err := filepath.Abs(approvalPath)
	if err != nil || !withinRoot(artifactRoot, absolute) {
		return promotionApproval{}, errors.New("promotion approval artifact must be inside the run artifact root")
	}
	relative, _ := filepath.Rel(artifactRoot, absolute)
	raw, err := readPromotionRegularFile(artifactRoot, absolute, filepath.ToSlash(relative))
	if err != nil {
		return promotionApproval{}, fmt.Errorf("explicit promotion approval artifact is required: %w", err)
	}
	approval := promotionApproval{}
	if err := decodePromotionJSONStrict(raw, &approval); err != nil {
		return promotionApproval{}, fmt.Errorf("promotion approval artifact is malformed: %w", err)
	}
	return approval, nil
}

func validatePromotionApproval(approval promotionApproval, manifest promotionPatchManifest) error {
	if approval.ContractVersion != promotionApprovalContractVersion {
		return errors.New("promotion approval contract version is unsupported")
	}
	if approval.Decision != "approve" || !approval.Approved {
		return errors.New("promotion approval decision does not approve this patch")
	}
	if approval.PatchSHA256 != manifest.Patch.SHA256 {
		return errors.New("promotion approval patch digest does not match")
	}
	if len(approval.Targets) != len(manifest.Targets) {
		return errors.New("promotion approval target bindings do not match")
	}
	for index, target := range manifest.Targets {
		approved := approval.Targets[index]
		if approved.Path != target.Path || approved.BeforeSHA256 != target.BeforeSHA256 {
			return errors.New("promotion approval target before-digests do not match")
		}
	}
	return nil
}

func inspectPromotionManifestTargets(repoRoot string, manifest promotionPatchManifest) ([]promotionTargetImage, error) {
	_, identity, err := loadSoftwareDevPromotionIdentity(repoRoot, manifest.MutableSurface.TaskPackPath, manifest.MutableSurface.StepID)
	if err != nil {
		return nil, err
	}
	expectedPaths := []string{identity.TaskPackPath}
	if len(manifest.Targets) == 2 {
		if identity.SiblingRoles == "" {
			return nil, errors.New("promotion patch manifest names a sibling roles target that is no longer owned")
		}
		expectedPaths = append(expectedPaths, identity.SiblingRoles)
	} else if identity.SiblingRoles != "" && manifest.Targets[0].AssignedChanges["inline_roles"] != nil {
		return nil, errors.New("promotion patch manifest uses inline roles while an exact owned sibling exists")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	targets := make([]promotionTargetImage, 0, len(expectedPaths))
	for index, path := range expectedPaths {
		if manifest.Targets[index].Path != path {
			return nil, errors.New("promotion patch manifest target identity does not match current repo ownership")
		}
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := rejectSymlinkPath(root, absolute, "promotion apply target"); err != nil {
			return nil, err
		}
		info, err := os.Stat(absolute)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("promotion apply target is not a regular repo-owned file: %s", path)
		}
		content, err := os.ReadFile(absolute)
		if err != nil {
			return nil, err
		}
		targets = append(targets, promotionTargetImage{Kind: manifest.Targets[index].Kind, Path: path, StepID: manifest.Targets[index].StepID, Absolute: absolute, Before: content})
	}
	return targets, nil
}

func applyPromotionTargetsTransactionally(targets []promotionTargetImage) ([]string, map[string]any, error) {
	type stagedTarget struct {
		target    promotionTargetImage
		stagePath string
	}
	staged := make([]stagedTarget, 0, len(targets))
	cleanup := func() {
		for _, item := range staged {
			_ = os.Remove(item.stagePath)
		}
	}
	defer cleanup()
	for _, target := range targets {
		if promotionDigest(target.Before) == promotionDigest(target.After) {
			continue
		}
		temporary, err := os.CreateTemp(filepath.Dir(target.Absolute), ".promotion-apply-*")
		if err != nil {
			return nil, map[string]any{"status": "not_started"}, err
		}
		stagePath := temporary.Name()
		staged = append(staged, stagedTarget{target: target, stagePath: stagePath})
		if _, err = temporary.Write(target.After); err == nil {
			err = temporary.Sync()
		}
		if closeErr := temporary.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, map[string]any{"status": "not_started"}, err
		}
		if target.Kind == "selected_task_pack_step" {
			if err := promotionValidateWorkflow(stagePath); err != nil {
				return nil, map[string]any{"status": "not_started"}, fmt.Errorf("staged promotion task pack is invalid: %w", err)
			}
		} else if err := validatePromotionAgentsDocument(target.After); err != nil {
			return nil, map[string]any{"status": "not_started"}, fmt.Errorf("staged promotion agents document is invalid: %w", err)
		}
	}

	changed := []string{}
	replaced := []promotionTargetImage{}
	for _, item := range staged {
		if err := promotionReplaceFile(item.stagePath, item.target.Absolute); err != nil {
			rollbackErrors := []string{}
			for index := len(replaced) - 1; index >= 0; index-- {
				replacedTarget := replaced[index]
				if restoreErr := writePromotionBytesWithReplace(replacedTarget.Absolute, replacedTarget.Before); restoreErr != nil {
					rollbackErrors = append(rollbackErrors, restoreErr.Error())
				}
			}
			rollback := map[string]any{"status": "succeeded", "restored_target_paths": anySlice(changed)}
			if len(rollbackErrors) > 0 {
				rollback["status"] = "failed"
				rollback["errors"] = anySlice(rollbackErrors)
			}
			return changed, rollback, fmt.Errorf("replace promotion target %s: %w", item.target.Path, err)
		}
		replaced = append(replaced, item.target)
		changed = append(changed, item.target.Path)
	}
	return changed, map[string]any{"status": "not_needed"}, nil
}

func replacePromotionFileAtomic(stagedPath, targetPath string) error {
	return os.Rename(stagedPath, targetPath)
}

func writePromotionBytesWithReplace(targetPath string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(targetPath), ".promotion-rollback-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(content); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return promotionReplaceFile(temporaryPath, targetPath)
}

func promotionResultTargets(targets []promotionPatchTarget) []any {
	result := make([]any, 0, len(targets))
	for _, target := range targets {
		result = append(result, map[string]any{
			"path": target.Path, "before_sha256": target.BeforeSHA256, "after_sha256": target.AfterSHA256,
		})
	}
	return result
}
