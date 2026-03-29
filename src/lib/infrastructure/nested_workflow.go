package infrastructure

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var errStopNestedWalk = errors.New("stop nested workflow walk")

// nestedResolverLeafDirs returns directories under each workflow/resolver root (any depth) named leaf
// that contain a resolver profile directory.
func nestedResolverLeafDirs(leaf string, roots []string) []string {
	leaf = strings.TrimSpace(leaf)
	if leaf == "" {
		return nil
	}
	var out []string
	for _, st := range roots {
		_ = filepath.WalkDir(st, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !d.IsDir() || filepath.Base(path) != leaf {
				return nil
			}
			if _, err := os.Stat(filepath.Join(path, "profile")); err == nil {
				out = append(out, path)
			}
			return nil
		})
	}
	return out
}

// nestedResolverProfileCandidates returns profile paths to try for resolver name (deduped).
func nestedResolverProfileCandidates(repoRoot, name string, roots []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, base := range nestedResolverLeafDirs(name, roots) {
		add(filepath.Join(base, "profile"))
		add(base)
	}
	return out
}

// nestedWorkflowConfigCandidates returns config.yml paths for workflow name (deduped).
// Tries nested src/core/workflows/**/<name>, then each extra root /**/<name>.
func nestedWorkflowConfigCandidates(repoRoot, name string, roots []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if bw := BundledWorkflowsAuthoringDir(repoRoot); bw != "" {
		_ = filepath.WalkDir(bw, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || d.Name() != "config.yml" {
				return nil
			}
			if filepath.Base(filepath.Dir(path)) != name {
				return nil
			}
			add(path)
			return nil
		})
	}
	for _, st := range roots {
		_ = filepath.WalkDir(st, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || d.Name() != "config.yml" {
				return nil
			}
			if filepath.Base(filepath.Dir(path)) != name {
				return nil
			}
			add(path)
			return nil
		})
	}
	return out
}

// FindNestedWorkflowDirByLeafName returns the first directory under extra workflow roots named leaf
// that contains config.yml (for dockpipe init --from <name> with namespace nesting).
func FindNestedWorkflowDirByLeafName(repoRoot, leaf string) string {
	return findNestedWorkflowDirByLeafName(repoRoot, leaf, WorkflowCompileRootsCached(repoRoot))
}

func findNestedWorkflowDirByLeafName(repoRoot, leaf string, roots []string) string {
	leaf = strings.TrimSpace(leaf)
	if leaf == "" {
		return ""
	}
	var hit string
	for _, st := range roots {
		err := filepath.WalkDir(st, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !d.IsDir() || filepath.Base(path) != leaf {
				return nil
			}
			if _, err := os.Stat(filepath.Join(path, "config.yml")); err == nil {
				hit = path
				return errStopNestedWalk
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopNestedWalk) {
			continue
		}
		if hit != "" {
			return hit
		}
	}
	return ""
}

// FindBundledWorkflowAuthoringDirByLeafName returns the first src/core/workflows/**/<leaf> directory
// that contains config.yml (nested namespace under bundled examples).
func FindBundledWorkflowAuthoringDirByLeafName(repoRoot, leaf string) string {
	leaf = strings.TrimSpace(leaf)
	if leaf == "" {
		return ""
	}
	bw := BundledWorkflowsAuthoringDir(repoRoot)
	if bw == "" {
		return ""
	}
	var hit string
	err := filepath.WalkDir(bw, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() || filepath.Base(path) != leaf {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "config.yml")); err == nil {
			hit = path
			return errStopNestedWalk
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopNestedWalk) {
		return ""
	}
	return hit
}
