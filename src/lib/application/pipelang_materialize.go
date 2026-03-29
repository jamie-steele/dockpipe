package application

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/pipelang"

	"gopkg.in/yaml.v3"
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

func materializePipeLangRoots(roots []string, force bool, outBase string) (int, error) {
	total := 0
	for _, root := range dedupeAbsExistingDirs(roots) {
		root := root
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
			wrote, err := materializePipeLangFile(path, root, force, outBase)
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

func materializePipeLangFile(path, root string, force bool, outBase string) (bool, error) {
	entryDir := filepath.Dir(path)
	moduleRoot := detectPipeLangModuleRoot(entryDir)
	files, latestMod, err := readPipeFilesUnder(moduleRoot)
	if err != nil {
		return false, err
	}
	entryName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	entryClass := ""
	mappings, err := loadPipeTypeMappings(moduleRoot)
	if err != nil {
		return false, err
	}
	pathAbs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, err
	}
	if len(mappings) > 0 {
		mapped, ok := mappings[pathAbs]
		if !ok {
			return false, nil
		}
		ec, err := inferEntryClassFromTypeRef(files, mapped.TypeRef)
		if err != nil {
			return false, fmt.Errorf("%s: %w", path, err)
		}
		entryClass = ec
	} else {
		if b, ok := files[path]; ok {
			if p, err := pipelang.Parse(b); err == nil && len(p.Classes) > 0 {
				entryClass = p.Classes[0].Name
			}
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
	if strings.TrimSpace(outBase) != "" {
		relDir, err := filepath.Rel(root, entryDir)
		if err != nil {
			relDir = filepath.Base(entryDir)
		}
		outDir = filepath.Join(outBase, rootHashToken(root), relDir)
	}
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

func rootHashToken(root string) string {
	sum := sha1.Sum([]byte(filepath.Clean(root)))
	return hex.EncodeToString(sum[:8])
}

type workflowTypeMapDoc struct {
	Types []string `yaml:"types"`
}

type pipeTypeMapping struct {
	TypeRef string
}

func loadPipeTypeMappings(moduleRoot string) (map[string]pipeTypeMapping, error) {
	cfgPath := filepath.Join(moduleRoot, "config.yml")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]pipeTypeMapping{}, nil
		}
		return nil, err
	}
	var doc workflowTypeMapDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	out := map[string]pipeTypeMapping{}
	for _, raw := range doc.Types {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}
		left := spec
		typeRef := ""
		i := strings.Index(spec, "<")
		j := strings.LastIndex(spec, ">")
		if i >= 0 || j >= 0 {
			if i <= 0 || j <= i+1 {
				return nil, fmt.Errorf("invalid types entry %q in %s (expected path/Type<EntryClass>)", spec, cfgPath)
			}
			left = strings.TrimSpace(spec[:i])
			typeRef = strings.TrimSpace(spec[i+1 : j])
		}
		left = strings.TrimSpace(left)
		if left == "" {
			return nil, fmt.Errorf("invalid types entry %q in %s (empty side)", spec, cfgPath)
		}
		leftPath := left
		if filepath.Ext(leftPath) == "" {
			leftPath += ".pipe"
		}
		leftAbs, err := filepath.Abs(filepath.Join(moduleRoot, filepath.FromSlash(leftPath)))
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(typeRef) == "" {
			typeRef = strings.TrimSuffix(filepath.Base(leftPath), filepath.Ext(leftPath))
		}
		out[leftAbs] = pipeTypeMapping{TypeRef: typeRef}
	}
	return out, nil
}

func inferEntryClassFromTypeRef(files map[string][]byte, typeRef string) (string, error) {
	ref := strings.TrimSpace(typeRef)
	if ref == "" {
		return "", fmt.Errorf("empty type reference")
	}
	merged := &pipelang.Program{}
	for name, b := range files {
		p, err := pipelang.Parse(b)
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		merged.Interfaces = append(merged.Interfaces, p.Interfaces...)
		merged.Classes = append(merged.Classes, p.Classes...)
	}
	for _, c := range merged.Classes {
		if c.Name == ref {
			return c.Name, nil
		}
	}
	var impl []string
	for _, c := range merged.Classes {
		if strings.TrimSpace(c.Implements) == ref {
			impl = append(impl, c.Name)
		}
	}
	switch len(impl) {
	case 1:
		return impl[0], nil
	case 0:
		return "", fmt.Errorf("types entry %q did not match a class or implemented interface", ref)
	default:
		return "", fmt.Errorf("types entry %q is ambiguous; multiple implementing classes: %s", ref, strings.Join(impl, ", "))
	}
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
