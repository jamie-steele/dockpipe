package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

const tarWorkflowSep = "##"

func loadWorkflowFromTarball(tarPath, entry string) (*domain.Workflow, error) {
	entry = filepath.ToSlash(entry)
	data, err := packagebuild.ReadFileFromTarGz(tarPath, entry)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.ToSlash(filepath.Dir(entry))
	readFile := func(p string) ([]byte, error) {
		return packagebuild.ReadFileFromTarGz(tarPath, filepath.ToSlash(filepath.Clean(p)))
	}
	return domain.ParseWorkflowFromDisk(data, baseDir, readFile)
}

// tryResolveWorkflowTarballURI returns a tar workflow URI when no on-disk config exists but a
// dockpipe-workflow-<name>-*.tar.gz exists under the packages store workflows dir (first), then
// packages.tarball_dir or release/artifacts. The archive must contain workflows/<name>/config.yml.
// When dockpipe.config.json sets packages.namespace, the workflow namespace in that file must match.
func tryResolveWorkflowTarballURI(repoRoot, workdir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	cfg, err := domain.LoadDockpipeProjectConfig(repoRoot)
	if err != nil {
		return "", err
	}
	var searchDirs []string
	if strings.TrimSpace(workdir) != "" {
		if pw, err := PackagesWorkflowsDir(workdir); err == nil {
			searchDirs = append(searchDirs, pw)
		}
	}
	if gw, err := GlobalPackagesWorkflowsDir(); err == nil {
		searchDirs = append(searchDirs, gw)
	}
	if d := workflowTarballSearchDir(repoRoot, cfg); d != "" {
		searchDirs = append(searchDirs, d)
	}
	entry := filepath.ToSlash(filepath.Join("workflows", name, "config.yml"))
	for _, dir := range searchDirs {
		pattern := filepath.Join(dir, fmt.Sprintf("dockpipe-workflow-%s-*.tar.gz", name))
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		// Prefer highest version suffix (lexicographic works for semver-like names).
		for i := len(matches) - 1; i >= 0; i-- {
			tarPath := matches[i]
			b, err := packagebuild.ReadFileFromTarGz(tarPath, entry)
			if err != nil {
				continue
			}
			var top struct {
				Namespace string `yaml:"namespace"`
			}
			if err := yaml.Unmarshal(b, &top); err != nil {
				continue
			}
			if cfg != nil && cfg.Packages.Namespace != nil {
				want := strings.TrimSpace(*cfg.Packages.Namespace)
				if want != "" {
					if strings.TrimSpace(top.Namespace) != want {
						continue
					}
				}
			}
			absTar, err := filepath.Abs(tarPath)
			if err != nil {
				return "", err
			}
			return formatTarWorkflowURI(absTar, entry), nil
		}
	}
	return "", nil
}

func workflowTarballSearchDir(repoRoot string, cfg *domain.DockpipeProjectConfig) string {
	if cfg != nil && cfg.Packages.TarballDir != nil {
		rel := strings.TrimSpace(*cfg.Packages.TarballDir)
		if rel != "" {
			p := filepath.Join(repoRoot, filepath.Clean(rel))
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p
			}
		}
	}
	def := filepath.Join(repoRoot, "release", "artifacts")
	if st, err := os.Stat(def); err == nil && st.IsDir() {
		return def
	}
	return ""
}

func formatTarWorkflowURI(absTarPath, entryInTar string) string {
	entryInTar = filepath.ToSlash(strings.TrimPrefix(entryInTar, "/"))
	return "tar://" + absTarPath + tarWorkflowSep + entryInTar
}

// SplitTarWorkflowURI splits a LoadWorkflow path produced by tryResolveWorkflowTarballURI.
func SplitTarWorkflowURI(path string) (tarPath, entry string, ok bool) {
	if !strings.HasPrefix(path, "tar://") {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, "tar://")
	i := strings.Index(rest, tarWorkflowSep)
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+len(tarWorkflowSep):], true
}
