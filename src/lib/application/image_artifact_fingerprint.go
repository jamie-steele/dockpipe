package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

func buildImageArtifactManifest(repoRoot, workflowName, packageName, imageKey, imageRef, buildDir, contextDir, policyFingerprint string, provenance domain.ImageArtifactProvenance) (*domain.ImageArtifactManifest, error) {
	provenance = trimImageArtifactProvenance(provenance)
	buildFingerprint, err := fingerprintDirTree(buildDir)
	if err != nil {
		return nil, err
	}
	buildSpec := &domain.CompiledImageBuildSpec{
		Context:    relOrAbs(repoRoot, contextDir),
		Dockerfile: relOrAbs(repoRoot, filepath.Join(buildDir, "Dockerfile")),
	}
	sourceFingerprint, err := domain.FingerprintJSON(struct {
		ImageRef         string                         `json:"image_ref"`
		Build            *domain.CompiledImageBuildSpec `json:"build"`
		BuildFingerprint string                         `json:"build_fingerprint"`
	}{
		ImageRef:         imageRef,
		Build:            buildSpec,
		BuildFingerprint: buildFingerprint,
	})
	if err != nil {
		return nil, err
	}
	fingerprint, err := domain.FingerprintJSON(struct {
		SourceFingerprint string                         `json:"source_fingerprint"`
		Provenance        domain.ImageArtifactProvenance `json:"provenance,omitempty"`
	}{
		SourceFingerprint: sourceFingerprint,
		Provenance:        provenance,
	})
	if err != nil {
		return nil, err
	}
	return &domain.ImageArtifactManifest{
		Schema:                      3,
		Kind:                        domain.ImageArtifactManifestKind,
		WorkflowName:                strings.TrimSpace(workflowName),
		PackageName:                 strings.TrimSpace(packageName),
		ImageKey:                    strings.TrimSpace(imageKey),
		Source:                      "build",
		ArtifactState:               "planned",
		Fingerprint:                 fingerprint,
		SourceFingerprint:           sourceFingerprint,
		SecurityManifestFingerprint: strings.TrimSpace(policyFingerprint),
		ImageRef:                    strings.TrimSpace(imageRef),
		Build:                       buildSpec,
		Provenance:                  provenance,
	}, nil
}

func trimImageArtifactProvenance(p domain.ImageArtifactProvenance) domain.ImageArtifactProvenance {
	return domain.ImageArtifactProvenance{
		Runtime:         strings.TrimSpace(p.Runtime),
		Resolver:        strings.TrimSpace(p.Resolver),
		Isolate:         strings.TrimSpace(p.Isolate),
		PackageVersion:  strings.TrimSpace(p.PackageVersion),
		DockpipeVersion: strings.TrimSpace(p.DockpipeVersion),
	}
}

func fingerprintDirTree(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			_, _ = io.WriteString(h, "dir:"+rel+"\n")
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		_, _ = io.WriteString(h, "file:"+rel+"\n")
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		_, _ = io.WriteString(h, "\n")
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func marshalArtifactJSON(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
