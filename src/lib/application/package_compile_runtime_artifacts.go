package application

import (
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func writeCompiledWorkflowRuntimeArtifacts(workdir, staging, pkgName string, wf *domain.Workflow) error {
	rm, im, err := compileWorkflowRuntimeArtifacts(workdir, pkgName, wf)
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
	return nil
}

func compileWorkflowRuntimeArtifacts(workdir, pkgName string, wf *domain.Workflow) (*domain.CompiledRuntimeManifest, *domain.ImageArtifactManifest, error) {
	rm := &domain.CompiledRuntimeManifest{
		Schema:          1,
		Kind:            domain.RuntimeManifestKind,
		WorkflowName:    strings.TrimSpace(wf.Name),
		PackageName:     strings.TrimSpace(pkgName),
		RuntimeProfile:  strings.TrimSpace(wf.Runtime),
		ResolverProfile: firstNonEmptyString(strings.TrimSpace(wf.DefaultResolver), strings.TrimSpace(wf.Resolver)),
		Security: domain.CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "advisory",
				InternalDNS: true,
			},
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
		},
	}

	policyFingerprint, err := domain.FingerprintJSON(rm.Security)
	if err != nil {
		return nil, nil, err
	}
	rm.PolicyFingerprint = policyFingerprint

	imageSel, artifact, err := selectCompiledImageArtifact(workdir, pkgName, wf, policyFingerprint)
	if err != nil {
		return nil, nil, err
	}
	rm.Image = imageSel
	if fp, err := domain.FingerprintJSON(rm.Image); err == nil {
		rm.ImageFingerprint = fp
	}
	rm.EnforcementSummaries = []string{
		"network policy currently compiles as advisory until full Docker egress enforcement lands",
		"filesystem and process defaults are emitted as the effective policy baseline",
	}
	rm.RuleIDs = []string{
		"security.preset.secure-default",
		"network.mode.restricted",
		"filesystem.root.readonly",
		"process.no-new-privileges",
	}
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

func selectCompiledImageArtifact(workdir, pkgName string, wf *domain.Workflow, policyFingerprint string) (domain.CompiledImageSelection, *domain.ImageArtifactManifest, error) {
	identity := firstNonEmptyString(
		strings.TrimSpace(wf.Isolate),
		strings.TrimSpace(wf.DefaultResolver),
		strings.TrimSpace(wf.Resolver),
		strings.TrimSpace(wf.Runtime),
		strings.TrimSpace(wf.DefaultRuntime),
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
