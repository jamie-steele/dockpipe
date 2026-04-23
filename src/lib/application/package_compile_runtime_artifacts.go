package application

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func writeCompiledWorkflowRuntimeArtifacts(workdir, staging, pkgName string, wf *domain.Workflow) error {
	rm, im, stepArtifacts, err := compileWorkflowRuntimeArtifacts(workdir, pkgName, wf)
	if err != nil {
		return err
	}
	manifestDir := filepath.Join(staging, domain.RuntimeManifestDirName)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(manifestDir, domain.RuntimeManifestFileName), rm); err != nil {
		return err
	}
	if im != nil {
		if err := writeJSONFile(filepath.Join(manifestDir, domain.ImageArtifactFileName), im); err != nil {
			return err
		}
	}
	for _, a := range stepArtifacts {
		if a.Manifest == nil {
			continue
		}
		p := filepath.Join(manifestDir, domain.RuntimeManifestPathForStep(a.StepID))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := writeJSONFile(p, a.Manifest); err != nil {
			return err
		}
		if a.Image != nil {
			ip := filepath.Join(manifestDir, domain.ImageArtifactPathForStep(a.StepID))
			if err := os.MkdirAll(filepath.Dir(ip), 0o755); err != nil {
				return err
			}
			if err := writeJSONFile(ip, a.Image); err != nil {
				return err
			}
		}
	}
	return nil
}

type compiledStepRuntimeArtifacts struct {
	StepID   string
	Manifest *domain.CompiledRuntimeManifest
	Image    *domain.ImageArtifactManifest
}

func compileWorkflowRuntimeArtifacts(workdir, pkgName string, wf *domain.Workflow) (*domain.CompiledRuntimeManifest, *domain.ImageArtifactManifest, []compiledStepRuntimeArtifacts, error) {
	policyProfile := normalizeWorkflowPolicyProfile(wf)
	security, policySources := compileSecurityPolicyForWorkflow(wf, policyProfile)
	rm := &domain.CompiledRuntimeManifest{
		Schema:          2,
		Kind:            domain.RuntimeManifestKind,
		WorkflowName:    strings.TrimSpace(wf.Name),
		PackageName:     strings.TrimSpace(pkgName),
		RuntimeProfile:  strings.TrimSpace(wf.Runtime),
		ResolverProfile: strings.TrimSpace(wf.Resolver),
		PolicyProfile:   policyProfile,
		PolicySources:   policySources,
		Security:        security,
	}

	policyFingerprint, err := domain.FingerprintJSON(rm.Security)
	if err != nil {
		return nil, nil, nil, err
	}
	rm.PolicyFingerprint = policyFingerprint

	imageSel, artifact, err := selectCompiledImageArtifact(workdir, pkgName, wf, policyFingerprint)
	if err != nil {
		return nil, nil, nil, err
	}
	rm.Image = imageSel
	if fp, err := domain.FingerprintJSON(rm.Image); err == nil {
		rm.ImageFingerprint = fp
	}
	rm.EnforcementSummaries = compiledEnforcementSummaries(rm)
	rm.RuleIDs = compiledRuleIDs(rm)
	if err := domain.ValidateCompiledRuntimeManifest(rm); err != nil {
		return nil, nil, nil, err
	}
	if artifact != nil {
		if err := domain.ValidateImageArtifactManifest(artifact); err != nil {
			return nil, nil, nil, err
		}
	}
	stepArtifacts, err := compileStepRuntimeArtifacts(workdir, pkgName, wf)
	if err != nil {
		return nil, nil, nil, err
	}
	return rm, artifact, stepArtifacts, nil
}

func compileStepRuntimeArtifacts(workdir, pkgName string, wf *domain.Workflow) ([]compiledStepRuntimeArtifacts, error) {
	if wf == nil || len(wf.Steps) == 0 {
		return nil, nil
	}
	out := make([]compiledStepRuntimeArtifacts, 0, len(wf.Steps))
	for i, step := range wf.Steps {
		if step.IsHostStep() || step.UsesPackagedWorkflow() {
			continue
		}
		stepID := compiledStepID(step, i)
		rm, im, err := compileStepRuntimeManifest(workdir, pkgName, wf, step, stepID)
		if err != nil {
			return nil, err
		}
		out = append(out, compiledStepRuntimeArtifacts{
			StepID:   stepID,
			Manifest: rm,
			Image:    im,
		})
	}
	return out, nil
}

func compileStepRuntimeManifest(workdir, pkgName string, wf *domain.Workflow, step domain.Step, stepID string) (*domain.CompiledRuntimeManifest, *domain.ImageArtifactManifest, error) {
	policyProfile := normalizeWorkflowPolicyProfile(wf)
	if p := strings.TrimSpace(step.Security.Profile); p != "" {
		policyProfile = p
	}
	security, policySources := compileSecurityPolicyForWorkflow(wf, policyProfile)
	stepOverride := applyStepSecurityOverrides(&security, step)
	if strings.TrimSpace(step.Security.Profile) != "" {
		stepOverride = true
	}
	security.Preset = policyProfile
	security.Network.Enforcement = compiledNetworkEnforcement(security.Network.Mode, policyProfile)
	security.Network.InternalDNS = true

	rm := &domain.CompiledRuntimeManifest{
		Schema:          2,
		Kind:            domain.RuntimeManifestKind,
		WorkflowName:    strings.TrimSpace(wf.Name),
		PackageName:     strings.TrimSpace(pkgName),
		StepID:          stepID,
		RuntimeProfile:  firstNonEmptyString(strings.TrimSpace(step.Runtime), strings.TrimSpace(wf.Runtime)),
		ResolverProfile: firstNonEmptyString(strings.TrimSpace(step.Resolver), strings.TrimSpace(wf.Resolver)),
		PolicyProfile:   policyProfile,
		PolicySources: domain.PolicySources{
			EngineDefault:    policySources.EngineDefault,
			RuntimeBaseline:  firstNonEmptyString(stepBaselineName(step, wf), policySources.RuntimeBaseline),
			PolicyProfile:    policyProfile,
			WorkflowOverride: policySources.WorkflowOverride,
			StepOverride:     stepOverride,
		},
		Security: security,
	}
	policyFingerprint, err := domain.FingerprintJSON(rm.Security)
	if err != nil {
		return nil, nil, err
	}
	rm.PolicyFingerprint = policyFingerprint
	imageSel, artifact, err := selectCompiledImageArtifactForStep(workdir, pkgName, wf, step, stepID, policyFingerprint)
	if err != nil {
		return nil, nil, err
	}
	rm.Image = imageSel
	if fp, err := domain.FingerprintJSON(rm.Image); err == nil {
		rm.ImageFingerprint = fp
	}
	rm.EnforcementSummaries = compiledEnforcementSummaries(rm)
	rm.RuleIDs = compiledRuleIDs(rm)
	if err := domain.ValidateCompiledRuntimeManifest(rm); err != nil {
		return nil, nil, err
	}
	if artifact != nil {
		if err := domain.ValidateImageArtifactManifest(artifact); err != nil {
			return nil, nil, err
		}
	}
	return rm, artifact, nil
}

func compiledStepID(step domain.Step, idx int) string {
	if s := strings.TrimSpace(step.ID); s != "" {
		return s
	}
	return "step-" + strings.TrimSpace(strconv.Itoa(idx+1))
}

func stepBaselineName(step domain.Step, wf *domain.Workflow) string {
	return firstNonEmptyString(
		strings.TrimSpace(step.Runtime),
		strings.TrimSpace(step.Isolate),
		strings.TrimSpace(step.Resolver),
		strings.TrimSpace(wf.Runtime),
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.Resolver),
		"container-default",
	)
}

func normalizeWorkflowPolicyProfile(wf *domain.Workflow) string {
	if wf == nil {
		return "secure-default"
	}
	if p := strings.TrimSpace(wf.Security.Profile); p != "" {
		return p
	}
	return "secure-default"
}

func compileSecurityPolicyForWorkflow(wf *domain.Workflow, profile string) (domain.CompiledSecurityPolicy, domain.PolicySources) {
	security := engineDefaultSecurityPolicy()
	baselineName, baseline := runtimeBaselineSecurityPolicy(wf)
	mergeCompiledSecurityPolicy(&security, baseline)
	mergeCompiledSecurityPolicy(&security, securityPolicyProfile(profile))
	workflowOverride := applyWorkflowSecurityOverrides(&security, wf)
	security.Preset = profile
	security.Network.Enforcement = compiledNetworkEnforcement(security.Network.Mode, profile)
	security.Network.InternalDNS = true
	return security, domain.PolicySources{
		EngineDefault:    true,
		RuntimeBaseline:  baselineName,
		PolicyProfile:    profile,
		WorkflowOverride: workflowOverride,
	}
}

func engineDefaultSecurityPolicy() domain.CompiledSecurityPolicy {
	return domain.CompiledSecurityPolicy{
		Preset: "secure-default",
		Network: domain.CompiledNetworkPolicy{
			Mode: "offline",
		},
	}
}

func runtimeBaselineSecurityPolicy(wf *domain.Workflow) (string, domain.CompiledSecurityPolicy) {
	if wf == nil {
		return "container-default", domain.CompiledSecurityPolicy{}
	}
	if !workflowUsesContainerSecurityPolicy(wf) {
		return "host-only", domain.CompiledSecurityPolicy{}
	}
	name := firstNonEmptyString(strings.TrimSpace(wf.Runtime), strings.TrimSpace(wf.Isolate), strings.TrimSpace(wf.Resolver), "container-default")
	return name, domain.CompiledSecurityPolicy{
		FS: domain.CompiledFilesystemPolicy{
			Root:      "readonly",
			Writes:    "workspace-only",
			TempPaths: []string{"/tmp"},
		},
		Process: domain.CompiledProcessPolicy{
			User:            "non-root",
			NoNewPrivileges: true,
			DropCaps:        []string{"ALL"},
			PIDLimit:        256,
		},
	}
}

func workflowUsesContainerSecurityPolicy(wf *domain.Workflow) bool {
	if wf == nil {
		return false
	}
	if len(wf.Steps) == 0 {
		return true
	}
	return wf.AnyContainerStep()
}

func securityPolicyProfile(name string) domain.CompiledSecurityPolicy {
	switch strings.TrimSpace(name) {
	case "internet-client":
		return domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{Mode: "internet"},
		}
	case "build-online":
		return domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{Mode: "internet"},
			FS: domain.CompiledFilesystemPolicy{
				Root:          "writable",
				Writes:        "declared",
				WritablePaths: []string{"/tmp", "/var/tmp"},
				TempPaths:     []string{"/tmp", "/var/tmp"},
			},
		}
	case "sidecar-client":
		return domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{Mode: "restricted"},
		}
	default:
		return domain.CompiledSecurityPolicy{}
	}
}

func applyWorkflowSecurityOverrides(dst *domain.CompiledSecurityPolicy, wf *domain.Workflow) bool {
	if dst == nil || wf == nil {
		return false
	}
	changed := false
	if v := strings.TrimSpace(wf.Security.Network.Mode); v != "" {
		dst.Network.Mode = v
		changed = true
	}
	if len(wf.Security.Network.Allow) > 0 {
		dst.Network.Allow = append([]string(nil), wf.Security.Network.Allow...)
		changed = true
	}
	if len(wf.Security.Network.Block) > 0 {
		dst.Network.Block = append([]string(nil), wf.Security.Network.Block...)
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Filesystem.Root); v != "" {
		dst.FS.Root = v
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Filesystem.Writes); v != "" {
		dst.FS.Writes = v
		changed = true
	}
	if len(wf.Security.Filesystem.WritablePaths) > 0 {
		dst.FS.WritablePaths = append([]string(nil), wf.Security.Filesystem.WritablePaths...)
		changed = true
	}
	if len(wf.Security.Filesystem.TempPaths) > 0 {
		dst.FS.TempPaths = append([]string(nil), wf.Security.Filesystem.TempPaths...)
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Process.User); v != "" {
		dst.Process.User = v
		changed = true
	}
	if wf.Security.Process.PIDLimit > 0 {
		dst.Process.PIDLimit = wf.Security.Process.PIDLimit
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Process.Resources.CPU); v != "" {
		dst.Process.Resources.CPU = v
		changed = true
	}
	if v := strings.TrimSpace(wf.Security.Process.Resources.Memory); v != "" {
		dst.Process.Resources.Memory = v
		changed = true
	}
	return changed
}

func applyStepSecurityOverrides(dst *domain.CompiledSecurityPolicy, step domain.Step) bool {
	if dst == nil {
		return false
	}
	changed := false
	if v := strings.TrimSpace(step.Security.Network.Mode); v != "" {
		dst.Network.Mode = v
		changed = true
	}
	if len(step.Security.Network.Allow) > 0 {
		dst.Network.Allow = append([]string(nil), step.Security.Network.Allow...)
		changed = true
	}
	if len(step.Security.Network.Block) > 0 {
		dst.Network.Block = append([]string(nil), step.Security.Network.Block...)
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Filesystem.Root); v != "" {
		dst.FS.Root = v
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Filesystem.Writes); v != "" {
		dst.FS.Writes = v
		changed = true
	}
	if len(step.Security.Filesystem.WritablePaths) > 0 {
		dst.FS.WritablePaths = append([]string(nil), step.Security.Filesystem.WritablePaths...)
		changed = true
	}
	if len(step.Security.Filesystem.TempPaths) > 0 {
		dst.FS.TempPaths = append([]string(nil), step.Security.Filesystem.TempPaths...)
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Process.User); v != "" {
		dst.Process.User = v
		changed = true
	}
	if step.Security.Process.PIDLimit > 0 {
		dst.Process.PIDLimit = step.Security.Process.PIDLimit
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Process.Resources.CPU); v != "" {
		dst.Process.Resources.CPU = v
		changed = true
	}
	if v := strings.TrimSpace(step.Security.Process.Resources.Memory); v != "" {
		dst.Process.Resources.Memory = v
		changed = true
	}
	return changed
}

func mergeCompiledSecurityPolicy(dst *domain.CompiledSecurityPolicy, src domain.CompiledSecurityPolicy) {
	if dst == nil {
		return
	}
	if strings.TrimSpace(src.Preset) != "" {
		dst.Preset = strings.TrimSpace(src.Preset)
	}
	if strings.TrimSpace(src.Network.Mode) != "" {
		dst.Network.Mode = strings.TrimSpace(src.Network.Mode)
	}
	if len(src.Network.Allow) > 0 {
		dst.Network.Allow = append([]string(nil), src.Network.Allow...)
	}
	if len(src.Network.Block) > 0 {
		dst.Network.Block = append([]string(nil), src.Network.Block...)
	}
	if strings.TrimSpace(src.FS.Root) != "" {
		dst.FS.Root = strings.TrimSpace(src.FS.Root)
	}
	if strings.TrimSpace(src.FS.Writes) != "" {
		dst.FS.Writes = strings.TrimSpace(src.FS.Writes)
	}
	if len(src.FS.WritablePaths) > 0 {
		dst.FS.WritablePaths = append([]string(nil), src.FS.WritablePaths...)
	}
	if len(src.FS.TempPaths) > 0 {
		dst.FS.TempPaths = append([]string(nil), src.FS.TempPaths...)
	}
	if strings.TrimSpace(src.Process.User) != "" {
		dst.Process.User = strings.TrimSpace(src.Process.User)
	}
	if src.Process.NoNewPrivileges {
		dst.Process.NoNewPrivileges = true
	}
	if len(src.Process.DropCaps) > 0 {
		dst.Process.DropCaps = append([]string(nil), src.Process.DropCaps...)
	}
	if len(src.Process.AddCaps) > 0 {
		dst.Process.AddCaps = append([]string(nil), src.Process.AddCaps...)
	}
	if src.Process.PIDLimit > 0 {
		dst.Process.PIDLimit = src.Process.PIDLimit
	}
	if strings.TrimSpace(src.Process.Resources.CPU) != "" {
		dst.Process.Resources.CPU = strings.TrimSpace(src.Process.Resources.CPU)
	}
	if strings.TrimSpace(src.Process.Resources.Memory) != "" {
		dst.Process.Resources.Memory = strings.TrimSpace(src.Process.Resources.Memory)
	}
}

func compiledNetworkEnforcement(mode, profile string) string {
	switch strings.TrimSpace(mode) {
	case "offline", "internet":
		return "native"
	case "allowlist", "restricted":
		if strings.TrimSpace(profile) == "sidecar-client" {
			return "proxy"
		}
		return "advisory"
	default:
		return "advisory"
	}
}

func compiledEnforcementSummaries(rm *domain.CompiledRuntimeManifest) []string {
	if rm == nil {
		return nil
	}
	ownership := "policy ownership: engine defaults + runtime baseline + selected profile + workflow overrides"
	if rm.PolicySources.StepOverride {
		ownership += " + step overrides"
	}
	lines := []string{ownership}
	if strings.TrimSpace(rm.PolicySources.RuntimeBaseline) == "host-only" {
		lines = append(lines, "container security policy applies only to container steps; host-only steps remain outside Docker enforcement")
	}
	switch strings.TrimSpace(rm.Security.Network.Enforcement) {
	case "proxy":
		lines = append([]string{"network policy requires a proxy-backed egress layer when this workflow runs"}, lines...)
	case "advisory":
		lines = append([]string{"network policy currently compiles as advisory until full Docker egress enforcement lands"}, lines...)
	}
	return lines
}

func compiledRuleIDs(rm *domain.CompiledRuntimeManifest) []string {
	if rm == nil {
		return nil
	}
	rules := []string{
		"security.profile." + firstNonEmptyString(strings.TrimSpace(rm.PolicyProfile), "secure-default"),
		"network.mode." + firstNonEmptyString(strings.TrimSpace(rm.Security.Network.Mode), "offline"),
	}
	if strings.TrimSpace(rm.Security.FS.Root) != "" {
		rules = append(rules, "filesystem.root."+strings.TrimSpace(rm.Security.FS.Root))
	}
	if rm.Security.Process.NoNewPrivileges {
		rules = append(rules, "process.no-new-privileges")
	}
	if len(rm.Security.Process.DropCaps) > 0 {
		rules = append(rules, "process.drop-caps")
	}
	if rm.PolicySources.WorkflowOverride {
		rules = append(rules, "security.workflow-override")
	}
	if rm.PolicySources.StepOverride {
		rules = append(rules, "security.step-override")
	}
	return rules
}

func selectCompiledImageArtifact(workdir, pkgName string, wf *domain.Workflow, policyFingerprint string) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	identity := firstNonEmptyString(
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.Resolver),
		strings.TrimSpace(wf.Runtime),
	)
	if identity == "" {
		return domain.CompiledImageSelection{}, nil, nil
	}

	if image, dockerfileDir, ok := infrastructure.TemplateBuild(workdir, identity); ok {
		ref := infrastructure.MaybeVersionTag(workdir, image)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(workdir, workdir),
			Dockerfile: relOrAbs(workdir, filepath.Join(dockerfileDir, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(workdir, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), identity, ref, dockerfileDir, workdir, policyFingerprint)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}

	sel := domain.CompiledImageSelection{
		Source: "registry",
		Ref:    identity,
	}
	fingerprint, err := domain.FingerprintJSON(struct {
		Identity          string `json:"identity"`
		Ref               string `json:"ref"`
		PolicyFingerprint string `json:"policy_fingerprint"`
	}{
		Identity:          identity,
		Ref:               identity,
		PolicyFingerprint: policyFingerprint,
	})
	if err != nil {
		return domain.CompiledImageSelection{}, nil, err
	}
	artifact := &domain.ImageArtifactManifest{
		Schema:                      1,
		Kind:                        domain.ImageArtifactManifestKind,
		WorkflowName:                strings.TrimSpace(wf.Name),
		PackageName:                 strings.TrimSpace(pkgName),
		ImageKey:                    identity,
		Source:                      "registry",
		Fingerprint:                 fingerprint,
		SourceFingerprint:           fingerprint,
		SecurityManifestFingerprint: policyFingerprint,
		ImageRef:                    identity,
	}
	return sel, artifact, nil
}

func selectCompiledImageArtifactForStep(workdir, pkgName string, wf *domain.Workflow, step domain.Step, stepID, policyFingerprint string) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	identity := firstNonEmptyString(
		strings.TrimSpace(step.Isolate),
		strings.TrimSpace(step.Runtime),
		strings.TrimSpace(step.Resolver),
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.Runtime),
		strings.TrimSpace(wf.Resolver),
	)
	if identity == "" {
		return domain.CompiledImageSelection{}, nil, nil
	}

	if image, dockerfileDir, ok := infrastructure.TemplateBuild(workdir, identity); ok {
		ref := infrastructure.MaybeVersionTag(workdir, image)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(workdir, workdir),
			Dockerfile: relOrAbs(workdir, filepath.Join(dockerfileDir, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(workdir, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, ref, dockerfileDir, workdir, policyFingerprint)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}

	sel := domain.CompiledImageSelection{
		Source: "registry",
		Ref:    identity,
	}
	fingerprint, err := domain.FingerprintJSON(struct {
		StepID            string `json:"step_id"`
		Identity          string `json:"identity"`
		Ref               string `json:"ref"`
		PolicyFingerprint string `json:"policy_fingerprint"`
	}{
		StepID:            stepID,
		Identity:          identity,
		Ref:               identity,
		PolicyFingerprint: policyFingerprint,
	})
	if err != nil {
		return domain.CompiledImageSelection{}, nil, err
	}
	artifact := &domain.ImageArtifactManifest{
		Schema:                      1,
		Kind:                        domain.ImageArtifactManifestKind,
		WorkflowName:                strings.TrimSpace(wf.Name),
		PackageName:                 strings.TrimSpace(pkgName),
		ImageKey:                    stepID,
		Source:                      "registry",
		Fingerprint:                 fingerprint,
		SourceFingerprint:           fingerprint,
		SecurityManifestFingerprint: policyFingerprint,
		ImageRef:                    identity,
	}
	return sel, artifact, nil
}

func writeJSONFile(path string, v any) error {
	b, err := marshalArtifactJSON(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func relOrAbs(base, path string) string {
	if base == "" {
		return path
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	if strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
