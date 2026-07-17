package application

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func writeCompiledWorkflowRuntimeArtifacts(workdir, staging, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest) error {
	rm, im, stepArtifacts, err := compileWorkflowRuntimeArtifacts(workdir, staging, pkgName, wf, pm)
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

func compileWorkflowRuntimeArtifacts(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest) (*domain.CompiledRuntimeManifest, *domain.ImageArtifactManifest, []compiledStepRuntimeArtifacts, error) {
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

	imageSel, artifact, err := selectCompiledImageArtifact(workdir, sourceRoot, pkgName, wf, pm, policyFingerprint)
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
	stepArtifacts, err := compileStepRuntimeArtifacts(workdir, sourceRoot, pkgName, wf, pm)
	if err != nil {
		return nil, nil, nil, err
	}
	return rm, artifact, stepArtifacts, nil
}

func compileStepRuntimeArtifacts(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest) ([]compiledStepRuntimeArtifacts, error) {
	if wf == nil || len(wf.Steps) == 0 {
		return nil, nil
	}
	out := make([]compiledStepRuntimeArtifacts, 0, len(wf.Steps))
	for i, step := range wf.Steps {
		if step.IsHostStep() || step.UsesPackagedWorkflow() {
			continue
		}
		stepID := compiledStepID(step, i)
		rm, im, err := compileStepRuntimeManifest(workdir, sourceRoot, pkgName, wf, pm, step, stepID)
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

func compileStepRuntimeManifest(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest, step domain.Step, stepID string) (*domain.CompiledRuntimeManifest, *domain.ImageArtifactManifest, error) {
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
	imageSel, artifact, err := selectCompiledImageArtifactForStep(workdir, sourceRoot, pkgName, wf, pm, step, stepID, policyFingerprint)
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

func selectCompiledImageArtifact(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest, policyFingerprint string) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	provenance := workflowImageArtifactProvenance(workdir, pm, wf)
	packages := normalizeAptPackages(wf.Image.Packages.Apt)
	if pm != nil && strings.TrimSpace(pm.Image.Ref) != "" {
		imageKey := packageImageKey(pm, wf)
		if len(packages) == 0 {
			sel, artifact, _, err := selectPackageImageArtifact(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), "", imageKey, pm, policyFingerprint, provenance)
			return sel, artifact, err
		}
		baseRef := strings.TrimSpace(pm.Image.Ref)
		derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, "workflow", baseRef, packages)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		ref := derivedImageRef(baseRef, packages)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(sourceRoot, sourceRoot),
			Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), imageKey, ref, derived, sourceRoot, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}
	identity := firstNonEmptyString(
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.Resolver),
		strings.TrimSpace(wf.Runtime),
	)
	if identity == "" {
		return domain.CompiledImageSelection{}, nil, nil
	}

	if image, dockerfileDir, ok := infrastructure.TemplateBuild(workdir, identity); ok {
		manifestRoot := workdir
		contextDir := workdir
		if localDir := workflowLocalImageDir(sourceRoot, identity); localDir != "" {
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = localDir
		}
		ref := infrastructure.MaybeVersionTag(workdir, image)
		if len(packages) > 0 {
			derived, err := writeDerivedAptImageBuild(sourceRoot, "workflow", ref, dockerfileDir, packages)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = derived
			ref = derivedImageRef(ref, packages)
		}
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(manifestRoot, contextDir),
			Dockerfile: relOrAbs(manifestRoot, filepath.Join(dockerfileDir, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(manifestRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), identity, ref, dockerfileDir, contextDir, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}

	if len(packages) > 0 {
		derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, "workflow", identity, packages)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		ref := derivedImageRef(identity, packages)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(sourceRoot, sourceRoot),
			Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), identity, ref, derived, sourceRoot, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}
	return registryImageSelection(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), "", identity, identity, "never", policyFingerprint, provenance)
}

func selectCompiledImageArtifactForStep(workdir, sourceRoot, pkgName string, wf *domain.Workflow, pm *domain.PackageManifest, step domain.Step, stepID, policyFingerprint string) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	provenance := stepImageArtifactProvenance(workdir, pm, wf, step)
	packages := normalizeAptPackages(append(append([]string{}, wf.Image.Packages.Apt...), step.Image.Packages.Apt...))
	if !stepHasImageSelectionOverride(step) {
		if pm != nil && strings.TrimSpace(pm.Image.Ref) != "" {
			if len(packages) == 0 {
				sel, artifact, _, err := selectPackageImageArtifact(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, stepID, pm, policyFingerprint, provenance)
				return sel, artifact, err
			}
			baseRef := strings.TrimSpace(pm.Image.Ref)
			derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, stepID, baseRef, packages)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			ref := derivedImageRef(baseRef, packages)
			buildSpec := &domain.CompiledImageBuildSpec{
				Context:    relOrAbs(sourceRoot, sourceRoot),
				Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
			}
			sel := domain.CompiledImageSelection{
				Source:    "build",
				Ref:       ref,
				AutoBuild: "if-stale",
				Build:     buildSpec,
			}
			artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, ref, derived, sourceRoot, policyFingerprint, provenance)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			return sel, artifact, nil
		}
	}
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
		manifestRoot := workdir
		contextDir := workdir
		if localDir := workflowLocalImageDir(sourceRoot, identity); localDir != "" {
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = localDir
		}
		ref := infrastructure.MaybeVersionTag(workdir, image)
		if len(packages) > 0 {
			derived, err := writeDerivedAptImageBuild(sourceRoot, stepID, ref, dockerfileDir, packages)
			if err != nil {
				return domain.CompiledImageSelection{}, nil, err
			}
			manifestRoot = sourceRoot
			contextDir = sourceRoot
			dockerfileDir = derived
			ref = derivedImageRef(ref, packages)
		}
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(manifestRoot, contextDir),
			Dockerfile: relOrAbs(manifestRoot, filepath.Join(dockerfileDir, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(manifestRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, ref, dockerfileDir, contextDir, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}

	if len(packages) > 0 {
		derived, err := writeDerivedRegistryAptImageBuild(sourceRoot, stepID, identity, packages)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		ref := derivedImageRef(identity, packages)
		buildSpec := &domain.CompiledImageBuildSpec{
			Context:    relOrAbs(sourceRoot, sourceRoot),
			Dockerfile: relOrAbs(sourceRoot, filepath.Join(derived, "Dockerfile")),
		}
		sel := domain.CompiledImageSelection{
			Source:    "build",
			Ref:       ref,
			AutoBuild: "if-stale",
			Build:     buildSpec,
		}
		artifact, err := buildImageArtifactManifest(sourceRoot, strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, ref, derived, sourceRoot, policyFingerprint, provenance)
		if err != nil {
			return domain.CompiledImageSelection{}, nil, err
		}
		return sel, artifact, nil
	}
	return registryImageSelection(strings.TrimSpace(wf.Name), strings.TrimSpace(pkgName), stepID, stepID, identity, "never", policyFingerprint, provenance)
}

func stepHasImageSelectionOverride(step domain.Step) bool {
	return strings.TrimSpace(step.Isolate) != "" ||
		strings.TrimSpace(step.Runtime) != "" ||
		strings.TrimSpace(step.Resolver) != ""
}

func workflowLocalImageDir(sourceRoot, identity string) string {
	sourceRoot = strings.TrimSpace(sourceRoot)
	identity = strings.TrimSpace(identity)
	if sourceRoot == "" || identity == "" {
		return ""
	}
	dir := filepath.Join(sourceRoot, "assets", "images", identity)
	if st, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil && !st.IsDir() {
		return dir
	}
	return ""
}

var aptPackageNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+._:-]*$`)

func normalizeAptPackages(pkgs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}

func validateAptPackages(pkgs []string) error {
	for _, pkg := range pkgs {
		if !aptPackageNamePattern.MatchString(pkg) {
			return fmt.Errorf("image.packages.apt contains invalid package name %q", pkg)
		}
	}
	return nil
}

func writeDerivedAptImageBuild(sourceRoot, key, baseRef, baseDockerfileDir string, pkgs []string) (string, error) {
	if err := validateAptPackages(pkgs); err != nil {
		return "", err
	}
	baseDockerfile := filepath.Join(baseDockerfileDir, "Dockerfile")
	b, err := os.ReadFile(baseDockerfile)
	if err != nil {
		return "", fmt.Errorf("read base Dockerfile for image.packages: %w", err)
	}
	body := insertAptInstallAfterBaseImage(string(b), pkgs)
	return writeDerivedImageDockerfile(sourceRoot, key, body)
}

func writeDerivedRegistryAptImageBuild(sourceRoot, key, baseRef string, pkgs []string) (string, error) {
	if err := validateAptPackages(pkgs); err != nil {
		return "", err
	}
	body := strings.Join([]string{
		"# syntax=docker/dockerfile:1.7",
		"FROM " + strings.TrimSpace(baseRef),
		"",
		aptInstallDockerfileRun(pkgs),
		"",
	}, "\n")
	return writeDerivedImageDockerfile(sourceRoot, key, body)
}

func writeDerivedImageDockerfile(sourceRoot, key, body string) (string, error) {
	key = packagebuild.SafeTarballToken(firstNonEmptyString(strings.TrimSpace(key), "workflow"))
	dir := filepath.Join(sourceRoot, domain.RuntimeManifestDirName, "images", key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(body), 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

func insertAptInstallAfterBaseImage(dockerfile string, pkgs []string) string {
	lines := strings.Split(strings.ReplaceAll(dockerfile, "\r\n", "\n"), "\n")
	lines = ensureDockerfileSyntax(lines)
	insertAt := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
			insertAt = i + 1
			break
		}
	}
	block := []string{
		"",
		"# DockPipe workflow-authored image packages.",
		"USER root",
		aptInstallDockerfileRun(pkgs),
		"",
	}
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:insertAt]...)
	out = append(out, block...)
	out = append(out, lines[insertAt:]...)
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func ensureDockerfileSyntax(lines []string) []string {
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# syntax=") {
		return lines
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, "# syntax=docker/dockerfile:1.7")
	out = append(out, lines...)
	return out
}

func aptInstallDockerfileRun(pkgs []string) string {
	return "RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt,sharing=locked apt-get update && apt-get install -y --no-install-recommends " + strings.Join(pkgs, " ") + " && rm -rf /var/lib/apt/lists/*"
}

func derivedImageRef(baseRef string, pkgs []string) string {
	fp, err := domain.FingerprintJSON(struct {
		Base string   `json:"base"`
		Apt  []string `json:"apt"`
	}{Base: strings.TrimSpace(baseRef), Apt: pkgs})
	token := "tools"
	if err == nil && strings.HasPrefix(fp, "sha256:") && len(fp) >= len("sha256:")+12 {
		token = fp[len("sha256:") : len("sha256:")+12]
	}
	return fmt.Sprintf("dockpipe-%s-tools:%s", imageRefSlug(baseRef), token)
}

func imageRefSlug(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(ref))
	var b strings.Builder
	lastDash := false
	for _, r := range ref {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "image"
	}
	if len(out) > 64 {
		return out[:64]
	}
	return out
}

func workflowImageArtifactProvenance(workdir string, pm *domain.PackageManifest, wf *domain.Workflow) domain.ImageArtifactProvenance {
	p := baseImageArtifactProvenance(workdir, pm)
	if wf == nil {
		return p
	}
	switch {
	case strings.TrimSpace(wf.Isolate) != "":
		p.Isolate = strings.TrimSpace(wf.Isolate)
	case strings.TrimSpace(wf.Resolver) != "":
		p.Resolver = strings.TrimSpace(wf.Resolver)
	case strings.TrimSpace(wf.Runtime) != "":
		p.Runtime = strings.TrimSpace(wf.Runtime)
	}
	return p
}

func stepImageArtifactProvenance(workdir string, pm *domain.PackageManifest, wf *domain.Workflow, step domain.Step) domain.ImageArtifactProvenance {
	p := baseImageArtifactProvenance(workdir, pm)
	switch {
	case strings.TrimSpace(step.Isolate) != "":
		p.Isolate = strings.TrimSpace(step.Isolate)
	case strings.TrimSpace(step.Resolver) != "":
		p.Resolver = strings.TrimSpace(step.Resolver)
	case strings.TrimSpace(step.Runtime) != "":
		p.Runtime = strings.TrimSpace(step.Runtime)
	case wf != nil && strings.TrimSpace(wf.Isolate) != "":
		p.Isolate = strings.TrimSpace(wf.Isolate)
	case wf != nil && strings.TrimSpace(wf.Resolver) != "":
		p.Resolver = strings.TrimSpace(wf.Resolver)
	case wf != nil && strings.TrimSpace(wf.Runtime) != "":
		p.Runtime = strings.TrimSpace(wf.Runtime)
	}
	return p
}

func baseImageArtifactProvenance(workdir string, pm *domain.PackageManifest) domain.ImageArtifactProvenance {
	p := domain.ImageArtifactProvenance{
		DockpipeVersion: authoredPackageVersion(workdir),
	}
	if pm != nil {
		p.PackageVersion = strings.TrimSpace(pm.Version)
	}
	return p
}

func selectPackageImageArtifact(workflowName, packageName, stepID, imageKey string, pm *domain.PackageManifest, policyFingerprint string, provenance domain.ImageArtifactProvenance) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, bool, error) {
	if pm == nil {
		return domain.CompiledImageSelection{}, nil, false, nil
	}
	ref := strings.TrimSpace(pm.Image.Ref)
	if ref == "" {
		return domain.CompiledImageSelection{}, nil, false, nil
	}
	pullPolicy := firstNonEmptyString(strings.TrimSpace(pm.Image.PullPolicy), "never")
	sel, artifact, err := registryImageSelection(workflowName, packageName, stepID, imageKey, ref, pullPolicy, policyFingerprint, provenance)
	return sel, artifact, true, err
}

func packageImageKey(pm *domain.PackageManifest, wf *domain.Workflow) string {
	if pm != nil && strings.TrimSpace(pm.Name) != "" {
		return strings.TrimSpace(pm.Name)
	}
	if wf != nil && strings.TrimSpace(wf.Name) != "" {
		return strings.TrimSpace(wf.Name)
	}
	return "workflow-image"
}

func registryImageSelection(workflowName, packageName, stepID, imageKey, ref, pullPolicy, policyFingerprint string, provenance domain.ImageArtifactProvenance) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	provenance = trimImageArtifactProvenance(provenance)
	expectedDigest := registryExpectedDigest(ref)
	sel := domain.CompiledImageSelection{
		Source:         "registry",
		Ref:            ref,
		PullPolicy:     pullPolicy,
		ExpectedDigest: expectedDigest,
	}
	sourceFingerprint, err := domain.FingerprintJSON(struct {
		StepID         string `json:"step_id,omitempty"`
		ImageKey       string `json:"image_key"`
		Ref            string `json:"ref"`
		PullPolicy     string `json:"pull_policy"`
		ExpectedDigest string `json:"expected_digest"`
	}{
		StepID:         stepID,
		ImageKey:       imageKey,
		Ref:            ref,
		PullPolicy:     pullPolicy,
		ExpectedDigest: expectedDigest,
	})
	if err != nil {
		return domain.CompiledImageSelection{}, nil, err
	}
	fingerprint, err := domain.FingerprintJSON(struct {
		SourceFingerprint string                         `json:"source_fingerprint"`
		Provenance        domain.ImageArtifactProvenance `json:"provenance,omitempty"`
	}{
		SourceFingerprint: sourceFingerprint,
		Provenance:        provenance,
	})
	if err != nil {
		return domain.CompiledImageSelection{}, nil, err
	}
	artifact := &domain.ImageArtifactManifest{
		Schema:                      3,
		Kind:                        domain.ImageArtifactManifestKind,
		WorkflowName:                workflowName,
		PackageName:                 packageName,
		StepID:                      stepID,
		ImageKey:                    imageKey,
		Source:                      "registry",
		ArtifactState:               "referenced",
		Fingerprint:                 fingerprint,
		SourceFingerprint:           sourceFingerprint,
		SecurityManifestFingerprint: policyFingerprint,
		ImageRef:                    ref,
		ExpectedDigest:              expectedDigest,
		Provenance:                  provenance,
	}
	return sel, artifact, nil
}

func registryExpectedDigest(ref string) string {
	ref = strings.TrimSpace(ref)
	if i := strings.LastIndex(ref, "@sha256:"); i >= 0 {
		return ref[i+1:]
	}
	return ""
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
