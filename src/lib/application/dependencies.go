package application

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type hostDependencyCandidate struct {
	Dep       domain.HostDependency
	Platforms []string
}

type missingHostDependency struct {
	Dep       domain.HostDependency
	Platforms []string
}

type packageDependencyMetadata struct {
	Platforms    []string
	Dependencies domain.DependencySpec
}

var (
	dependencyLookPathFn = exec.LookPath
	dependencyPowerShellLookupFn = dependencyLookupViaPowerShell
	dependencyRunShellFn = runDependencyInstallShellCommand
)

func checkWorkflowHostDependencies(wf *domain.Workflow, wfRoot, wfConfig string, opts *CliOpts) error {
	var deps []hostDependencyCandidate
	if wf != nil {
		if err := checkHostPlatformSupported("workflow", wf.Platforms); err != nil {
			return err
		}
		deps = appendDependencyCandidates(deps, wf.Dependencies, wf.Platforms)
	}
	if pkgMeta, err := packageDependenciesForWorkflow(wfRoot, wfConfig); err != nil {
		return err
	} else {
		if err := checkHostPlatformSupported("package", pkgMeta.Platforms); err != nil {
			return err
		}
		deps = appendDependencyCandidates(deps, pkgMeta.Dependencies, pkgMeta.Platforms)
	}
	missing := missingRequiredHostDependencies(deduplicateHostDependencies(deps))
	if len(missing) == 0 {
		return nil
	}
	if err := maybeInstallMissingHostDependencies(missing, opts); err != nil {
		return err
	}
	missing = missingRequiredHostDependencies(deduplicateHostDependencies(deps))
	if len(missing) == 0 {
		return nil
	}
	return missingHostDependenciesError(missing)
}

func appendDependencyCandidates(out []hostDependencyCandidate, deps domain.DependencySpec, platforms []string) []hostDependencyCandidate {
	for _, dep := range deps.Host {
		out = append(out, hostDependencyCandidate{
			Dep:       dep,
			Platforms: append([]string(nil), platforms...),
		})
	}
	return out
}

func checkHostPlatformSupported(owner string, platforms []string) error {
	if dependencyPlatformsSupportCurrent(platforms) {
		return nil
	}
	return fmt.Errorf("%s does not support host platform %q (supported: %s)", owner, currentDependencyPlatform(), strings.Join(platforms, ", "))
}

func missingHostDependenciesError(missing []missingHostDependency) error {
	var b strings.Builder
	b.WriteString("missing required host dependencies:")
	for _, missingDep := range missing {
		dep := missingDep.Dep
		id := hostDependencyID(dep)
		command := hostDependencyCommand(dep)
		b.WriteString("\n  - ")
		b.WriteString(id)
		if command != "" && command != id {
			b.WriteString(" (command: ")
			b.WriteString(command)
			b.WriteString(")")
		}
		if desc := strings.TrimSpace(dep.Description); desc != "" {
			b.WriteString(": ")
			b.WriteString(desc)
		}
		if hint := installCommandForCurrentPlatform(dep); hint != "" && dependencyPlatformsSupportCurrent(missingDep.Platforms) {
			b.WriteString("\n    install: ")
			b.WriteString(hint)
		} else if !dependencyPlatformsSupportCurrent(missingDep.Platforms) {
			b.WriteString("\n    install: no installer declared for this host platform")
		}
	}
	b.WriteString("\nInstall the missing dependency or choose a workflow/package that does not require it, then rerun.")
	return fmt.Errorf("%s", b.String())
}

func maybeInstallMissingHostDependencies(missing []missingHostDependency, opts *CliOpts) error {
	for _, missingDep := range missing {
		dep := missingDep.Dep
		installCmd := installCommandForCurrentPlatform(dep)
		if strings.TrimSpace(installCmd) == "" {
			continue
		}
		if !dependencyPlatformsSupportCurrent(missingDep.Platforms) {
			continue
		}
		approved, err := approveDependencyInstall(dep, installCmd, opts)
		if err != nil {
			return err
		}
		if !approved {
			continue
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] dependency install: %s\n", hostDependencyID(dep))
		if err := dependencyRunShellFn(installCmd); err != nil {
			return fmt.Errorf("install dependency %s: %w", hostDependencyID(dep), err)
		}
	}
	return nil
}

func approveDependencyInstall(dep domain.HostDependency, installCmd string, opts *CliOpts) (bool, error) {
	if opts != nil && opts.ApproveSystemChanges {
		return true, nil
	}
	if envBool(os.Getenv("DOCKPIPE_APPROVE_PROMPTS")) {
		return true, nil
	}
	fd, ok := stdinFDInt()
	if !ok || !term.IsTerminal(fd) {
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Missing dependency %s (%s).\n", hostDependencyID(dep), hostDependencyCommand(dep))
	fmt.Fprintf(os.Stderr, "[dockpipe] Installer from workflow/package metadata:\n  %s\n", installCmd)
	fmt.Fprintf(os.Stderr, "Run this installer now? [y/N]: ")
	br := bufio.NewReader(os.Stdin)
	line, _ := br.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

func packageDependenciesForWorkflow(wfRoot, wfConfig string) (packageDependencyMetadata, error) {
	if strings.HasPrefix(wfConfig, "tar://") {
		tarPath, entry, ok := infrastructure.SplitTarWorkflowURI(wfConfig)
		if !ok {
			return packageDependencyMetadata{}, nil
		}
		pmEntry := filepath.ToSlash(filepath.Join(filepath.Dir(entry), infrastructure.PackageManifestFilename))
		b, err := packagebuild.ReadFileFromTarGz(tarPath, pmEntry)
		if err != nil {
			return packageDependencyMetadata{}, nil
		}
		var pm domain.PackageManifest
		if err := yamlUnmarshalPackageManifest(b, &pm); err != nil {
			return packageDependencyMetadata{}, fmt.Errorf("package manifest: %w", err)
		}
		return packageDependencyMetadata{Platforms: pm.Platforms, Dependencies: pm.Dependencies}, nil
	}
	for cur := strings.TrimSpace(wfRoot); cur != ""; cur = filepath.Dir(cur) {
		p := filepath.Join(cur, infrastructure.PackageManifestFilename)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			pm, err := domain.ParsePackageManifest(p)
			if err != nil {
				return packageDependencyMetadata{}, err
			}
			return packageDependencyMetadata{Platforms: pm.Platforms, Dependencies: pm.Dependencies}, nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
	}
	return packageDependencyMetadata{}, nil
}

func yamlUnmarshalPackageManifest(b []byte, pm *domain.PackageManifest) error {
	if err := yaml.Unmarshal(b, pm); err != nil {
		return err
	}
	domain.NormalizePackageManifestYAMLAliases(pm)
	return domain.ValidatePackageManifest(pm)
}

func missingRequiredHostDependencies(deps []hostDependencyCandidate) []missingHostDependency {
	var missing []missingHostDependency
	for _, candidate := range deps {
		dep := candidate.Dep
		if dep.Required != nil && !*dep.Required {
			continue
		}
		cmd := hostDependencyCommand(dep)
		if cmd == "" {
			continue
		}
		if _, err := resolveDependencyCommandPath(cmd); err != nil {
			missing = append(missing, missingHostDependency(candidate))
		}
	}
	return missing
}

func resolveDependencyCommandPath(command string) (string, error) {
	if path, err := dependencyLookPathFn(command); err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		if path, err := dependencyPowerShellLookupFn(command); err == nil && strings.TrimSpace(path) != "" {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func deduplicateHostDependencies(deps []hostDependencyCandidate) []hostDependencyCandidate {
	seen := map[string]struct{}{}
	out := make([]hostDependencyCandidate, 0, len(deps))
	for _, candidate := range deps {
		dep := candidate.Dep
		key := hostDependencyCommand(dep)
		if key == "" {
			key = hostDependencyID(dep)
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func hostDependencyID(dep domain.HostDependency) string {
	if id := strings.TrimSpace(dep.ID); id != "" {
		return id
	}
	return hostDependencyCommand(dep)
}

func hostDependencyCommand(dep domain.HostDependency) string {
	if cmd := strings.TrimSpace(dep.Command); cmd != "" {
		return cmd
	}
	return strings.TrimSpace(dep.ID)
}

func installCommandForCurrentPlatform(dep domain.HostDependency) string {
	hint := dep.Install
	switch currentDependencyPlatform() {
	case "windows":
		return strings.TrimSpace(hint.Windows)
	case "macos":
		return strings.TrimSpace(hint.MacOS)
	case "deb":
		if v := strings.TrimSpace(hint.Deb); v != "" {
			return v
		}
		return strings.TrimSpace(hint.Linux)
	default:
		return strings.TrimSpace(hint.Linux)
	}
}

func dependencyPlatformsSupportCurrent(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	current := currentDependencyPlatform()
	for _, platform := range platforms {
		if strings.EqualFold(strings.TrimSpace(platform), current) {
			return true
		}
		if current == "deb" && strings.EqualFold(strings.TrimSpace(platform), "linux") {
			return true
		}
	}
	return false
}

func currentDependencyPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "macos"
	case "linux":
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			return "deb"
		}
		return "linux"
	default:
		return runtime.GOOS
	}
}

func runDependencyInstallShellCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("empty install command")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dependencyLookupViaPowerShell(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("empty command")
	}
	ps := fmt.Sprintf(
		"$cmd = Get-Command -Name '%s' -ErrorAction SilentlyContinue; if ($cmd) { $cmd.Source }",
		strings.ReplaceAll(command, "'", "''"),
	)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", os.ErrNotExist
	}
	return path, nil
}

func envBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
