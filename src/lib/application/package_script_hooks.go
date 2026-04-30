package application

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
)

type packageScriptTarget struct {
	Name       string
	Manifest   string
	PackageDir string
	ScriptRel  string
	ScriptAbs  string
}

func discoverPackageScriptTargets(workdir, only string, selectScript func(*domain.PackageManifest) string) ([]packageScriptTarget, error) {
	only = strings.TrimSpace(only)
	if only != "" && strings.ContainsRune(only, os.PathListSeparator) {
		return nil, fmt.Errorf("package selector accepts one package name")
	}
	projectRoot, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return nil, err
	}
	cfg, err := domain.LoadDockpipeProjectConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	roots := domain.EffectiveWorkflowCompileRoots(cfg, projectRoot)
	var targets []packageScriptTarget
	seenManifests := map[string]struct{}{}
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Base(path) != "package.yml" {
				return nil
			}
			manifestPath := path
			if abs, err := filepath.Abs(manifestPath); err == nil {
				manifestPath = abs
			}
			if _, ok := seenManifests[manifestPath]; ok {
				return nil
			}
			seenManifests[manifestPath] = struct{}{}
			manifest, err := domain.ParsePackageManifest(manifestPath)
			if err != nil {
				return err
			}
			dirName := filepath.Base(filepath.Dir(manifestPath))
			if only != "" && manifest.Name != only && dirName != only {
				return nil
			}
			rawScript := strings.TrimSpace(selectScript(manifest))
			if rawScript == "" {
				return nil
			}
			scriptRel := filepath.Clean(rawScript)
			packageDir := filepath.Dir(manifestPath)
			targets = append(targets, packageScriptTarget{
				Name:       manifest.Name,
				Manifest:   manifestPath,
				PackageDir: packageDir,
				ScriptRel:  scriptRel,
				ScriptAbs:  filepath.Join(packageDir, filepath.FromSlash(scriptRel)),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets, nil
}

func runPackageScriptTarget(workdir string, target packageScriptTarget, env []string, missingLabel string) error {
	if _, err := os.Stat(target.ScriptAbs); err != nil {
		return fmt.Errorf("%s %q not found (%s)", missingLabel, target.ScriptRel, target.ScriptAbs)
	}
	cmd, err := dockpipeScriptCommand(target.ScriptAbs)
	if err != nil {
		return err
	}
	baseEnv := append(os.Environ(),
		"DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_PACKAGE_ROOT="+target.PackageDir,
		"DOCKPIPE_PACKAGE_MANIFEST="+target.Manifest,
	)
	cmd.Dir = target.PackageDir
	cmd.Env = append(baseEnv, env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dockpipeScriptCommand(scriptAbs string) (*exec.Cmd, error) {
	lower := strings.ToLower(scriptAbs)
	switch {
	case strings.HasSuffix(lower, ".ps1"):
		return exec.Command("pwsh", "-File", scriptAbs), nil
	case strings.HasSuffix(lower, ".cmd"), strings.HasSuffix(lower, ".bat"):
		if runtime.GOOS != "windows" {
			return nil, fmt.Errorf("script %q requires cmd.exe on Windows", scriptAbs)
		}
		return exec.Command("cmd", "/c", scriptAbs), nil
	default:
		return exec.Command("bash", scriptAbs), nil
	}
}
