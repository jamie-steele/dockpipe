package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func cmdPackageImages(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageImagesUsageText)
		return nil
	}
	workdir, err := parsePackageImagesWorkdir(args)
	if err != nil {
		return err
	}
	records, err := collectPackageImageArtifacts(workdir)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		root, _ := infrastructure.ImageArtifactIndexDir(workdir)
		fmt.Fprintf(os.Stderr, "[dockpipe] no image artifacts found (%s)\n", root)
		return nil
	}
	for _, r := range records {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			trimDigestPrefix(r.Fingerprint),
			firstNonEmptyString(strings.TrimSpace(r.ArtifactState), "planned"),
			strings.TrimSpace(r.Source),
			strings.TrimSpace(r.ImageRef),
			strings.TrimSpace(r.WorkflowName),
			strings.TrimSpace(r.PackageName),
			strings.TrimSpace(r.StepID),
			strings.TrimSpace(r.ImageKey),
		)
	}
	return nil
}

func parsePackageImagesWorkdir(args []string) (string, error) {
	var workdir string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return "", fmt.Errorf("unknown option %s", args[i])
		default:
			return "", fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if strings.TrimSpace(workdir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return filepath.Abs(workdir)
}

func collectPackageImageArtifacts(workdir string) ([]domain.ImageArtifactManifest, error) {
	byFingerprint := map[string]domain.ImageArtifactManifest{}
	planned, err := collectPlannedImageArtifactsFromPackages(workdir)
	if err != nil {
		return nil, err
	}
	for _, r := range planned {
		putImageArtifactRecord(byFingerprint, r)
	}
	indexed, err := collectIndexedImageArtifacts(workdir)
	if err != nil {
		return nil, err
	}
	for _, r := range indexed {
		putImageArtifactRecord(byFingerprint, r)
	}
	out := make([]domain.ImageArtifactManifest, 0, len(byFingerprint))
	for _, r := range byFingerprint {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]
		for _, cmp := range []struct{ x, y string }{
			{a.WorkflowName, b.WorkflowName},
			{a.PackageName, b.PackageName},
			{a.StepID, b.StepID},
			{a.ImageKey, b.ImageKey},
			{a.ImageRef, b.ImageRef},
			{a.Fingerprint, b.Fingerprint},
		} {
			if cmp.x != cmp.y {
				return cmp.x < cmp.y
			}
		}
		return false
	})
	return out, nil
}

func putImageArtifactRecord(records map[string]domain.ImageArtifactManifest, r domain.ImageArtifactManifest) {
	key := strings.TrimSpace(r.Fingerprint)
	if key == "" {
		key = strings.TrimSpace(r.ImageRef) + "\x00" + strings.TrimSpace(r.ImageKey)
	}
	if strings.TrimSpace(r.ArtifactState) == "" {
		r.ArtifactState = "planned"
	}
	if existing, ok := records[key]; ok && imageArtifactStateRank(existing.ArtifactState) >= imageArtifactStateRank(r.ArtifactState) {
		return
	}
	records[key] = r
}

func imageArtifactStateRank(state string) int {
	switch strings.TrimSpace(state) {
	case "materialized", "cached":
		return 3
	case "referenced":
		return 2
	case "planned":
		return 1
	default:
		return 0
	}
}

func collectPlannedImageArtifactsFromPackages(workdir string) ([]domain.ImageArtifactManifest, error) {
	workflowDir, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(workflowDir, "dockpipe-workflow-*.tar.gz"))
	if err != nil {
		return nil, err
	}
	var out []domain.ImageArtifactManifest
	for _, tgz := range matches {
		paths, err := packagebuild.ListTarGzMemberPaths(tgz)
		if err != nil {
			return nil, err
		}
		for _, entry := range paths {
			if !isImageArtifactTarEntry(entry) {
				continue
			}
			b, err := packagebuild.ReadFileFromTarGz(tgz, entry)
			if err != nil {
				return nil, err
			}
			var r domain.ImageArtifactManifest
			if err := json.Unmarshal(b, &r); err != nil {
				return nil, fmt.Errorf("%s:%s: %w", tgz, entry, err)
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func collectIndexedImageArtifacts(workdir string) ([]domain.ImageArtifactManifest, error) {
	root, err := infrastructure.ImageArtifactIndexDir(workdir)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "by-fingerprint")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]domain.ImageArtifactManifest, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r domain.ImageArtifactManifest
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, e.Name()), err)
		}
		out = append(out, r)
	}
	return out, nil
}

func trimDigestPrefix(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "sha256:")
}

const packageImagesUsageText = `dockpipe package images [--workdir <path>]

Lists Docker image artifacts known to the project. The command merges planned image
artifacts from compiled workflow tarballs with materialized/cached receipts under
bin/.dockpipe/internal/images/by-fingerprint.

Output columns (tab-separated):
  fingerprint, state, source, image_ref, workflow, package, step_id, image_key

`
