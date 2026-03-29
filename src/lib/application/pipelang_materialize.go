package application

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/pipelang"
)

func dedupeAbsExistingDirs(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(filepath.Clean(p))
		if err != nil {
			continue
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}
	return out
}

func materializePipeLangRoots(roots []string, force bool) (int, error) {
	total := 0
	for _, root := range dedupeAbsExistingDirs(roots) {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".pipelang" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".pipe") {
				return nil
			}
			wrote, err := materializePipeLangFile(path, force)
			if err != nil {
				return err
			}
			if wrote {
				total++
			}
			return nil
		})
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func materializePipeLangFile(path string, force bool) (bool, error) {
	entryDir := filepath.Dir(path)
	moduleRoot := detectPipeLangModuleRoot(entryDir)
	files, latestMod, err := readPipeFilesUnder(moduleRoot)
	if err != nil {
		return false, err
	}
	entryName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	entryClass := ""
	if b, ok := files[path]; ok {
		if p, err := pipelang.Parse(b); err == nil && len(p.Classes) > 0 {
			entryClass = p.Classes[0].Name
		}
	}
	if strings.TrimSpace(entryClass) == "" {
		// Interface-only/helper files are valid split units but do not emit artifacts directly.
		return false, nil
	}
	res, err := pipelang.CompileFiles(files, entryClass)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	outDir := filepath.Join(entryDir, ".pipelang")
	prefix := filepath.Join(outDir, entryName+"."+res.EntryClass)
	wfPath := prefix + ".workflow.yml"
	jsonPath := prefix + ".bindings.json"
	envPath := prefix + ".bindings.env"
	if !force {
		if upToDate(latestMod, wfPath, jsonPath, envPath) {
			return false, nil
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(wfPath, res.WorkflowYAML, 0o644); err != nil {
		return false, err
	}
	if err := os.WriteFile(jsonPath, res.BindingsJSON, 0o644); err != nil {
		return false, err
	}
	if err := os.WriteFile(envPath, res.BindingsEnv, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func detectPipeLangModuleRoot(startDir string) string {
	dir := startDir
	for {
		if hasFile(filepath.Join(dir, "config.pipe")) || hasFile(filepath.Join(dir, "config.yml")) {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			return startDir
		}
		dir = next
	}
}

func hasFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func readPipeFilesUnder(root string) (map[string][]byte, time.Time, error) {
	out := map[string][]byte{}
	var latest time.Time
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".pipelang" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".pipe") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = b
		if st, err := os.Stat(path); err == nil && st.ModTime().After(latest) {
			latest = st.ModTime()
		}
		return nil
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(out) == 0 {
		return nil, time.Time{}, fmt.Errorf("no .pipe files found under %s", root)
	}
	return out, latest, nil
}

func upToDate(srcMod time.Time, outs ...string) bool {
	for _, p := range outs {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return false
		}
		if st.ModTime().Before(srcMod) {
			return false
		}
	}
	return true
}

func compileWorkflowOneFromPipe(workdir, wfDir string, force bool) error {
	pipePath := filepath.Join(wfDir, "config.pipe")
	files, _, err := readPipeFilesUnder(wfDir)
	if err != nil {
		return err
	}
	entryClass := ""
	if b, ok := files[pipePath]; ok {
		if p, err := pipelang.Parse(b); err == nil && len(p.Classes) > 0 {
			entryClass = p.Classes[0].Name
		}
	}
	compiled, err := pipelang.CompileFiles(files, entryClass)
	if err != nil {
		return fmt.Errorf("%s: %w", pipePath, err)
	}
	staging, err := os.MkdirTemp("", "dockpipe-wf-pipe-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := copyDir(wfDir, staging); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, "config.yml"), compiled.WorkflowYAML, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, ".pipelang.bindings.json"), compiled.BindingsJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, ".pipelang.bindings.env"), compiled.BindingsEnv, 0o644); err != nil {
		return err
	}
	return compileWorkflowOne(workdir, staging, filepath.Base(wfDir), force)
}
