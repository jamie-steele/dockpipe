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
	cmd, bashExe, err := dockpipeScriptCommand(target.ScriptAbs)
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
	if bashExe != "" {
		cmd.Env = upsertEnvLocal(cmd.Env, "DOCKPIPE_HOST_BASH_BIN", bashExe)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func upsertEnvLocal(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func dockpipeScriptCommand(scriptAbs string) (*exec.Cmd, string, error) {
	lower := strings.ToLower(scriptAbs)
	switch {
	case strings.HasSuffix(lower, ".ps1"):
		return exec.Command("pwsh", "-File", scriptAbs), "", nil
	case strings.HasSuffix(lower, ".cmd"), strings.HasSuffix(lower, ".bat"):
		if runtime.GOOS != "windows" {
			return nil, "", fmt.Errorf("script %q requires cmd.exe on Windows", scriptAbs)
		}
		return exec.Command("cmd", "/c", scriptAbs), "", nil
	default:
		bashExe, bashArg, err := dockpipeBashCommandParts(scriptAbs)
		if err != nil {
			return nil, "", err
		}
		return exec.Command(bashExe, bashArg), bashExe, nil
	}
}

func dockpipeBashCommandParts(scriptAbs string) (string, string, error) {
	if runtime.GOOS == "windows" {
		if bashExe := gitBashWindowsPath(); bashExe != "" {
			return bashExe, pathForGitBash(scriptAbs), nil
		}
	}
	bashExe, err := exec.LookPath("bash")
	if err != nil {
		return "", "", fmt.Errorf("bash not found for script %q", scriptAbs)
	}
	return bashExe, scriptAbs, nil
}

func gitBashWindowsPath() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Programs", "Git", "bin", "bash.exe"),
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	}
	seen := map[string]bool{}
	for _, p := range candidates {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func pathForGitBash(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	vol := filepath.VolumeName(abs)
	if len(vol) >= 2 && vol[1] == ':' {
		drive := strings.ToLower(string(vol[0]))
		rest := abs[len(vol):]
		for len(rest) > 0 && (rest[0] == '\\' || rest[0] == '/') {
			rest = rest[1:]
		}
		rest = filepath.ToSlash(rest)
		return "/" + drive + "/" + rest
	}
	return filepath.ToSlash(abs)
}
