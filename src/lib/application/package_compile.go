package application

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

// injectCompileWorkdirFromProjectConfig prepends --workdir <dir> when args does not already
// set it, where dir is the directory containing dockpipe.config.json found by walking up
// from the current working directory (or cwd if the file is absent).
func injectCompileWorkdirFromProjectConfig(args []string) ([]string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" && i+1 < len(args) {
			return args, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := domain.FindProjectRootWithDockpipeConfig(cwd)
	if err != nil {
		return nil, err
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	if root != cwdAbs {
		fmt.Fprintf(os.Stderr, "[dockpipe] using project root %s (%s)\n", root, domain.DockpipeProjectConfigFileName)
	}
	return append([]string{"--workdir", root}, args...), nil
}

func cmdPackageCompile(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(packageCompileUsageText)
		return nil
	}
	switch args[0] {
	case "for-workflow":
		return cmdPackageCompileForWorkflow(args[1:])
	case "workflow":
		return cmdPackageCompileWorkflow(args[1:])
	case "core":
		return cmdPackageCompileCore(args[1:])
	case "resolvers":
		return cmdPackageCompileResolvers(args[1:])
	case "bundles":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			fmt.Print(packageCompileBundlesUsageText)
			return nil
		}
		// compile.bundles paths are merged into compile.workflows; same recursive config.yml walk.
		return cmdPackageCompileWorkflowsBatch(args[1:])
	case "workflows":
		return cmdPackageCompileWorkflowsBatch(args[1:])
	case "all":
		return cmdPackageCompileAll(args[1:])
	default:
		return fmt.Errorf("unknown package compile target %q (try: dockpipe package compile --help; use for-workflow for dependency-scoped compile)", args[0])
	}
}

func cmdPackageCompileWorkflow(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileWorkflowUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		src     string
		name    string
		force   bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			src = args[i+1]
			i++
		case args[i] == "--name" && i+1 < len(args):
			name = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile workflow --help)", args[i])
		default:
			if src == "" {
				src = args[i]
				continue
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("missing source directory (use --from <path> or a positional path)")
	}
	srcAbs, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return err
	}
	return compileWorkflowOne(workdir, srcAbs, name, force)
}

// compileWorkflowOne validates YAML, materializes a streamable dockpipe-workflow-<name>-<ver>.tar.gz
// under packages/workflows/ (no expanded directory trees in the store).
func compileWorkflowOne(workdir, srcAbs, name string, force bool) error {
	cfgPath := filepath.Join(srcAbs, "config.yml")
	if _, err := os.Stat(cfgPath); err != nil {
		return fmt.Errorf("workflow source must contain config.yml: %w", err)
	}
	if err := infrastructure.ValidateWorkflowYAML(cfgPath); err != nil {
		return fmt.Errorf("validate workflow: %w", err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	wf, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	for i, hook := range wf.CompileHooks {
		hook = strings.TrimSpace(hook)
		if hook == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] compile_hooks[%d]: %s\n", i, hook)
		cmd := exec.Command("sh", "-c", hook)
		cmd.Dir = srcAbs
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("compile_hooks[%d]: %w", i, err)
		}
	}
	pkgName := strings.TrimSpace(name)
	if pkgName == "" {
		pkgName = strings.TrimSpace(wf.Name)
	}
	if pkgName == "" {
		pkgName = filepath.Base(srcAbs)
	}
	pw, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(pw, 0o755); err != nil {
		return err
	}
	tarGlob := filepath.Join(pw, fmt.Sprintf("dockpipe-workflow-%s-*.tar.gz", packagebuild.SafeTarballToken(pkgName)))
	legacyDir := filepath.Join(pw, pkgName)
	if !force {
		if matches, _ := filepath.Glob(tarGlob); len(matches) > 0 {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip workflow compile (already exists): %s (--force to rebuild)\n", matches[0])
			return nil
		}
		if _, err := os.Stat(legacyDir); err == nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip workflow compile (legacy dir exists): %s (--force to rebuild)\n", legacyDir)
			return nil
		}
	} else {
		_ = infrastructure.RemoveGlob(tarGlob)
		_ = os.RemoveAll(legacyDir)
	}
	staging, err := os.MkdirTemp("", "dockpipe-compile-wf-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := copyDir(srcAbs, staging); err != nil {
		return fmt.Errorf("copy workflow: %w", err)
	}
	manifestPath := filepath.Join(staging, infrastructure.PackageManifestFilename)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		pm := map[string]any{
			"schema":               1,
			"name":                 pkgName,
			"version":              "0.1.0",
			"title":                pkgName,
			"description":          "Compiled from " + srcAbs,
			"kind":                 "workflow",
			"requires_capabilities": []string{strings.TrimSpace(wf.Capability)},
			"allow_clone":          true,
			"distribution":         "source",
		}
		repoRoot, err := filepath.Abs(workdir)
		if err != nil {
			return err
		}
		if ns := strings.TrimSpace(wf.Namespace); ns != "" {
			pm["namespace"] = ns
		} else if pc, err := loadDockpipeProjectConfig(repoRoot); err == nil && pc != nil && pc.Packages.Namespace != nil {
			if def := strings.TrimSpace(*pc.Packages.Namespace); def != "" {
				pm["namespace"] = def
			}
		}
		out, err := yaml.Marshal(pm)
		if err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
			return err
		}
	}
	pmParsed, err := domain.ParsePackageManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("package manifest: %w", err)
	}
	ver := strings.TrimSpace(pmParsed.Version)
	if ver == "" {
		ver = "0.1.0"
	}
	outPath := filepath.Join(pw, fmt.Sprintf("dockpipe-workflow-%s-%s.tar.gz", packagebuild.SafeTarballToken(pkgName), packagebuild.SafeTarballToken(ver)))
	if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, outPath, "workflows/"+pkgName); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compiled workflow package → %s\n", outPath)
	return nil
}

func cmdPackageCompileCore(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileCoreUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		src     string
		force   bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			src = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile core --help)", args[i])
		default:
			if src == "" {
				src = args[i]
				continue
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	repoRoot, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(src) == "" {
		cfg, err := loadDockpipeProjectConfig(repoRoot)
		if err != nil {
			return err
		}
		if p, err := coreFromConfig(cfg, repoRoot); err != nil {
			return err
		} else if strings.TrimSpace(p) != "" {
			src = p
		}
	}
	if strings.TrimSpace(src) == "" {
		src, err = defaultCoreSource(repoRoot)
		if err != nil {
			return err
		}
	}
	srcAbs, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return err
	}
	if st, err := os.Stat(srcAbs); err != nil || !st.IsDir() {
		return fmt.Errorf("core source must be a directory: %s", srcAbs)
	}
	coreDir, err := infrastructure.PackagesCoreDir(workdir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		return err
	}
	coreTarGlob := filepath.Join(coreDir, "dockpipe-core-*.tar.gz")
	if !force {
		if matches, _ := filepath.Glob(coreTarGlob); len(matches) > 0 {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip core compile (already exists): %s (--force to rebuild)\n", matches[0])
			return nil
		}
		if st, err := os.Stat(filepath.Join(coreDir, "runtimes")); err == nil && st.IsDir() {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip core compile (legacy tree exists): %s (--force to rebuild)\n", coreDir)
			return nil
		}
	} else {
		_ = infrastructure.RemoveGlob(coreTarGlob)
		_ = infrastructure.RemoveLegacyPackageSubdirs(coreDir)
	}
	staging, err := os.MkdirTemp("", "dockpipe-compile-core-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	exclude := map[string]bool{"resolvers": true, "bundles": true, "workflows": true}
	if err := copyDirExcludingTopLevel(srcAbs, staging, exclude); err != nil {
		return fmt.Errorf("copy core: %w", err)
	}
	manifestPath := filepath.Join(staging, infrastructure.PackageManifestFilename)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		pm := map[string]any{
			"schema":       1,
			"name":         "dockpipe.core",
			"version":      "0.1.0",
			"title":        "Compiled core slice",
			"description":  "Compiled from " + srcAbs,
			"kind":         "core",
			"allow_clone":  true,
			"distribution": "source",
			"depends":      []string{},
		}
		out, err := yaml.Marshal(pm)
		if err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
			return err
		}
	}
	pmParsed, err := domain.ParsePackageManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("package manifest: %w", err)
	}
	ver := strings.TrimSpace(pmParsed.Version)
	if ver == "" {
		ver = "0.1.0"
	}
	outPath := filepath.Join(coreDir, fmt.Sprintf("dockpipe-core-%s.tar.gz", packagebuild.SafeTarballToken(ver)))
	if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, outPath, "core"); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compiled core package → %s\n", outPath)
	return nil
}

func defaultCoreSource(repoRoot string) (string, error) {
	srcCore := filepath.Join(repoRoot, "src", "core")
	if st, err := os.Stat(filepath.Join(srcCore, "runtimes")); err == nil && st.IsDir() {
		return filepath.Abs(srcCore)
	}
	tc := filepath.Join(repoRoot, "templates", "core")
	if st, err := os.Stat(filepath.Join(tc, "runtimes")); err == nil && st.IsDir() {
		return filepath.Abs(tc)
	}
	return "", fmt.Errorf("no default core tree (expected src/core/runtimes or templates/core/runtimes); use --from <dir>")
}

func cmdPackageCompileResolvers(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileResolversUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir   string
		from      []string
		noStaging bool
		force     bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			from = append(from, args[i+1])
			i++
		case args[i] == "--no-staging":
			noStaging = true
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile resolvers --help)", args[i])
		default:
			if len(from) == 0 {
				from = append(from, args[i])
				continue
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	repoRoot, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if len(from) == 0 {
		cfg, err := loadDockpipeProjectConfig(repoRoot)
		if err != nil {
			return err
		}
		from = effectiveResolverCompileRoots(cfg, repoRoot, noStaging)
	}
	if len(from) == 0 {
		return fmt.Errorf("no resolver source directories (set compile.workflows in %s or pass --from)", domain.DockpipeProjectConfigFileName)
	}
	destRes, err := infrastructure.PackagesResolversDir(workdir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destRes, 0o755); err != nil {
		return err
	}
	var defResolverNamespace string
	if cfg, err := loadDockpipeProjectConfig(repoRoot); err == nil && cfg != nil && cfg.Packages.Namespace != nil {
		defResolverNamespace = strings.TrimSpace(*cfg.Packages.Namespace)
	}
	total := 0
	for _, root := range from {
		srcAbs, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			return err
		}
		if st, err := os.Stat(srcAbs); err != nil || !st.IsDir() {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip missing resolvers root: %s\n", srcAbs)
			continue
		}
		n, err := mergeChildPackages(srcAbs, destRes, "resolver", defResolverNamespace, force)
		if err != nil {
			return err
		}
		total += n
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] resolver packages: wrote %d tarball(s) (one dockpipe-resolver-* per profile; shared installable units, same store as workflows) → %s\n", total, destRes)
	return nil
}

const resolverMetaFilename = "resolver.yaml"

// readResolverNamespaceYAML returns the optional namespace from <dir>/resolver.yaml (empty if absent).
func readResolverNamespaceYAML(dir string) (string, error) {
	p := filepath.Join(dir, resolverMetaFilename)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var aux struct {
		Namespace string `yaml:"namespace"`
	}
	if err := yaml.Unmarshal(b, &aux); err != nil {
		return "", fmt.Errorf("parse %s: %w", p, err)
	}
	if err := domain.ValidateNamespace(aux.Namespace); err != nil {
		return "", err
	}
	return strings.TrimSpace(aux.Namespace), nil
}

// collectResolverPackRoots returns directories whose immediate children are resolver profile dirs
// (each child has profile/). Order: top-level resolvers/, packages/*/resolvers/, dockpipe/packages/*/resolvers/
// (legacy), then dockpipe/<package>/resolvers/ (maintainer: agent, ide, secrets — package roots with resolver "plugins").
//
// When compile.workflows includes "packages/" as a root, srcRoot is that directory itself — patterns like
// "<srcRoot>/packages/*/resolvers" look for packages/packages/... and miss "<srcRoot>/dorkpipe/resolvers".
// Monorepo suite layouts are covered by "<srcRoot>/*/resolvers", "<srcRoot>/*/*/resolvers", and
// "<srcRoot>/*/*/resolvers/*" (e.g. cloud/storage/resolvers/r2).
// Each resolver child still becomes its own dockpipe-resolver-<name>-*.tar.gz for the store.
func collectResolverPackRoots(srcRoot string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(filepath.Join(srcRoot, "resolvers"))
	for _, pat := range []string{
		filepath.Join(srcRoot, "packages", "*", "resolvers"),
		filepath.Join(srcRoot, "dockpipe", "packages", "*", "resolvers"),
		filepath.Join(srcRoot, "dockpipe", "*", "resolvers"),
		filepath.Join(srcRoot, "*", "resolvers"),
		filepath.Join(srcRoot, "*", "*", "resolvers"),
		filepath.Join(srcRoot, "*", "*", "resolvers", "*"),
	} {
		matches, _ := filepath.Glob(pat)
		for _, m := range matches {
			add(m)
		}
	}
	return out
}

// hasNestedResolverPackLayout reports whether dir looks like a grouped resolver tree (at least one
// immediate child directory has no profile/ — recurse into group folders until profile/ is found).
func hasNestedResolverPackLayout(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "profile")); err != nil {
			return true
		}
	}
	return false
}

// mergeChildPackages packs each immediate child directory from srcRoot into
// dockpipe-resolver-<name>-<ver>.tar.gz under destRoot (no expanded trees).
// For resolvers, when the tree is grouped (child dirs without profile/), we descend until profile/ is found.
//
// Resolver authoring: one or more pack roots are collected (see collectResolverPackRoots):
//   - srcRoot/resolvers/ — flat vendor tree
//   - srcRoot/packages/<group>/resolvers/ — per-package groups (each group is its own folder; same
//     resolver names must not appear twice across groups)
//   - srcRoot/dockpipe/<package>/resolvers/ — DockPipe official packages (e.g. agent → codex, ide → vscode)
// Each resolver child still becomes its own dockpipe-resolver-<name>-*.tar.gz for the store.
func mergeChildPackages(srcRoot, destRoot string, kind string, defaultNamespace string, force bool) (int, error) {
	if kind == "resolver" {
		roots := collectResolverPackRoots(srcRoot)
		// Drop top-level resolvers/ if it does not exist (collectResolverPackRoots still added it — fix)
		roots = filterExistingResolverRoots(roots)
		if len(roots) > 0 {
			total := 0
			for _, root := range roots {
				n, err := mergeChildPackagesWalk(root, destRoot, kind, defaultNamespace, force)
				total += n
				if err != nil {
					return total, err
				}
			}
			return total, nil
		}
	}
	return mergeChildPackagesWalk(srcRoot, destRoot, kind, defaultNamespace, force)
}

func filterExistingResolverRoots(roots []string) []string {
	var out []string
	for _, p := range roots {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// compileSingleResolverDir packs one resolver profile directory (contains profile) into
// dockpipe-resolver-<name>-<ver>.tar.gz under destRoot.
func compileSingleResolverDir(destRoot, from, name string, defaultNamespace string, force bool) error {
	kind := "resolver"
	tarGlob := filepath.Join(destRoot, fmt.Sprintf("dockpipe-resolver-%s-*.tar.gz", packagebuild.SafeTarballToken(name)))
	legacyDir := filepath.Join(destRoot, name)
	if !force {
		if matches, _ := filepath.Glob(tarGlob); len(matches) > 0 {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip %s compile %q (already exists): %s (--force to rebuild)\n", kind, name, matches[0])
			return nil
		}
		if _, err := os.Stat(legacyDir); err == nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip %s compile %q (legacy dir exists): %s (--force to rebuild)\n", kind, name, legacyDir)
			return nil
		}
	} else {
		_ = infrastructure.RemoveGlob(tarGlob)
		_ = os.RemoveAll(legacyDir)
	}
	staging, err := os.MkdirTemp("", "dockpipe-compile-"+kind+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := copyDir(from, staging); err != nil {
		return fmt.Errorf("copy %s %s: %w", kind, name, err)
	}
	manifestPath := filepath.Join(staging, infrastructure.PackageManifestFilename)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		pm := map[string]any{
			"schema":       1,
			"name":         name,
			"version":      "0.1.0",
			"title":        name,
			"description":  "Compiled from " + from,
			"kind":         kind,
			"allow_clone":  true,
			"distribution": "source",
		}
		ns, err := readResolverNamespaceYAML(staging)
		if err != nil {
			return fmt.Errorf("resolver %s: %w", name, err)
		}
		if ns != "" {
			pm["namespace"] = ns
		} else if strings.TrimSpace(defaultNamespace) != "" {
			pm["namespace"] = strings.TrimSpace(defaultNamespace)
		}
		out, err := yaml.Marshal(pm)
		if err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
			return err
		}
	}
	pmParsed, err := domain.ParsePackageManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("%s %s: %w", kind, name, err)
	}
	ver := strings.TrimSpace(pmParsed.Version)
	if ver == "" {
		ver = "0.1.0"
	}
	prefix := "resolvers/" + name
	base := fmt.Sprintf("dockpipe-resolver-%s-%s.tar.gz", packagebuild.SafeTarballToken(name), packagebuild.SafeTarballToken(ver))
	outPath := filepath.Join(destRoot, base)
	if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, outPath, prefix); err != nil {
		return fmt.Errorf("%s %s: %w", kind, name, err)
	}
	return nil
}

func mergeChildPackagesWalk(srcRoot, destRoot string, kind string, defaultNamespace string, force bool) (int, error) {
	entries, err := os.ReadDir(srcRoot)
	if err != nil {
		return 0, err
	}
	nestedPack := kind == "resolver" && hasNestedResolverPackLayout(srcRoot)
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		from := filepath.Join(srcRoot, name)
		if nestedPack {
			if _, err := os.Stat(filepath.Join(from, "profile")); err != nil {
				sub, err := mergeChildPackagesWalk(from, destRoot, kind, defaultNamespace, force)
				if err != nil {
					return n, err
				}
				n += sub
				continue
			}
		}
		if kind != "resolver" {
			return n, fmt.Errorf("mergeChildPackages: unknown kind %q", kind)
		}
		if err := compileSingleResolverDir(destRoot, from, name, defaultNamespace, force); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func cmdPackageCompileWorkflowsBatch(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileWorkflowsUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir   string
		from      []string
		force     bool
		noStaging bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			from = append(from, args[i+1])
			i++
		case args[i] == "--force":
			force = true
		case args[i] == "--no-staging":
			noStaging = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile workflows --help)", args[i])
		default:
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	repoRoot, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if len(from) == 0 {
		cfg, err := loadDockpipeProjectConfig(repoRoot)
		if err != nil {
			return err
		}
		from = effectiveWorkflowCompileRoots(cfg, repoRoot, noStaging)
	}
	seen := make(map[string]struct{})
	total := 0
	for _, root := range from {
		rootAbs, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			return err
		}
		if _, err := os.Stat(rootAbs); err != nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip missing workflows root: %s\n", rootAbs)
			continue
		}
		if err := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "config.yml" {
				return nil
			}
			wfDir := filepath.Dir(path)
			wfName := filepath.Base(wfDir)
			if strings.HasPrefix(wfName, ".") {
				return nil
			}
			if _, ok := seen[wfName]; ok {
				fmt.Fprintf(os.Stderr, "[dockpipe] skip duplicate workflow name %q (already compiled from an earlier --from)\n", wfName)
				return nil
			}
			if err := compileWorkflowOne(workdir, wfDir, "", force); err != nil {
				return fmt.Errorf("workflow %q: %w", wfName, err)
			}
			seen[wfName] = struct{}{}
			total++
			return nil
		}); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] workflow packages: compiled %d tarball(s) under .dockpipe/internal/packages/workflows/\n", total)
	return nil
}

func cmdPackageCompileAll(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileAllUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir   string
		force     bool
		noStaging bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case args[i] == "--no-staging":
			noStaging = true
		case args[i] == "--with-bundles", args[i] == "--skip-bundles":
			// Ignored: bundle roots compile as workflows (compile.bundles merged into compile.workflows).
		case args[i] == "--help" || args[i] == "-h":
			fmt.Print(packageCompileAllUsageText)
			return nil
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile all --help)", args[i])
		default:
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	repoRoot, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	cfg, err := loadDockpipeProjectConfig(repoRoot)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compile all: core spine → resolver packages (one tarball per profile) → workflow packages\n")
	if err := cmdPackageCompileCore(workdirAndForceArgs(workdir, force)); err != nil {
		return err
	}
	resRoots := effectiveResolverCompileRoots(cfg, repoRoot, noStaging)
	if len(resRoots) == 0 {
		fmt.Fprintf(os.Stderr, "[dockpipe] compile all: skip resolver packages (no resolver source dirs)\n")
	} else {
		resArgs := []string{"--workdir", workdir}
		if force {
			resArgs = append(resArgs, "--force")
		}
		for _, p := range resRoots {
			resArgs = append(resArgs, "--from", p)
		}
		if err := cmdPackageCompileResolvers(resArgs); err != nil {
			return err
		}
	}
	wfArgs := workdirAndForceArgs(workdir, force)
	for _, p := range effectiveWorkflowCompileRoots(cfg, repoRoot, noStaging) {
		wfArgs = append(wfArgs, "--from", p)
	}
	if err := cmdPackageCompileWorkflowsBatch(wfArgs); err != nil {
		return err
	}
	return validateCompileOutputs(workdir)
}

func workdirAndForceArgs(workdir string, force bool) []string {
	out := []string{"--workdir", workdir}
	if force {
		out = append(out, "--force")
	}
	return out
}

const packageCompileUsageText = `dockpipe package compile

Validate and materialize packages into .dockpipe/internal/packages/ (see docs/package-model.md).

Usage:
  dockpipe package compile core [options]
  dockpipe package compile resolvers [options]
  dockpipe package compile bundles [options]   (alias: same as compile workflows; compile.bundles merged into roots)
  dockpipe package compile workflows [options]
  dockpipe package compile all [options]
  dockpipe package compile for-workflow <name> [options]   (core + transitive resolvers/workflows only)
  dockpipe package compile workflow [options] [--from] <source-dir>

Order for a full local store: core (spine only) → resolvers → workflows (dockpipe-workflow-* tarballs only).
Use "compile all" to run the default sequence.

`

const packageCompileWorkflowUsageText = `dockpipe package compile workflow <source-dir>

Runs workflow YAML validation (same rules as dockpipe workflow validate), runs optional
compile_hooks from config.yml (shell, cwd = source dir), then writes the workflow tarball under
<workdir>/.dockpipe/internal/packages/workflows/.

Options:
  --workdir <path>   Project directory (default: current directory)
  --from <path>      Source workflow directory (same as positional <source-dir>)
  --name <n>         Package folder name (default: workflow name from config.yml, else basename of source)
  --force            Replace existing package directory

`

const packageCompileCoreUsageText = `dockpipe package compile core

Copies a core authoring tree (default: src/core or templates/core when present) into
<workdir>/.dockpipe/internal/packages/core/ and writes package.yml (kind: core).
Top-level resolvers/, bundles/, and workflows/ in the source are omitted — compile those with
"package compile resolvers|workflows" so they land under packages/resolvers/ or packages/workflows/.

Optional dockpipe.config.json "compile.core_from" overrides the default core path when --from is omitted.

Options:
  --workdir <path>   Project directory (default: current directory)
  --from <path>      Source core root (typically runtimes/, strategies/, assets/, …)
  --force            Replace existing packages/core tree

`

const packageCompileResolversUsageText = `dockpipe package compile resolvers

Merges each child directory from each --from source into
.dockpipe/internal/packages/resolvers/<name>/ (later --from wins on name clash).

Defaults: same roots as compile.workflows (plus legacy compile.bundles merged in), plus src/core/resolvers and
templates/core/resolvers when those directories exist. Deprecated compile.resolvers entries are merged if present.
Dirs with profile/ under each root become resolver tarballs.

Pack roots (each immediate child with profile/ becomes one store tarball):
  - <from>/resolvers/...                    (flat vendor tree)
  - <from>/packages/<group>/resolvers/...  (per-package groups, e.g. ides, agents)
  - <from>/dockpipe/packages/<group>/resolvers/...  (DockPipe official maintainer layout)
src/core/resolvers has no nested resolvers/ — unchanged.

Optional resolver.yaml next to each profile may set namespace: <label> (same rules as workflow namespace).

Options:
  --workdir <path>      Project directory (default: current directory)
  --from <path>         Repeatable; each root's subdirectories are resolver profiles
  --no-staging          Skip paths under .staging/ when using defaults

`

const packageCompileBundlesUsageText = `dockpipe package compile bundles

Same as "dockpipe package compile workflows". Legacy name only — compile.bundles paths are merged into
compile.workflows; bundle-listed trees are normal workflows (config.yml per directory).

Options:
  --workdir <path>      Project directory (default: current directory)
  --from <path>         Repeatable workflow roots
  --force               Replace existing tarballs
  --no-staging          Skip .staging paths when using defaults

`

const packageCompileWorkflowsUsageText = `dockpipe package compile workflows

Compiles every immediate subdirectory that contains config.yml under each --from root.

Defaults: dockpipe.config.json compile.workflows, else workflows/ when present.

Options:
  --workdir <path>       Project directory (default: current directory)
  --from <path>          Repeatable; roots to scan for named workflow folders
  --force                Replace existing packages/workflows/<name>
  --no-staging           Skip .staging paths when using defaults

`

const packageCompileAllUsageText = `dockpipe package compile all

Runs: compile core → compile resolver packages (one tarball per profile) → compile workflow packages (compile.bundles merged into compile.workflows when set).
Uses dockpipe.config.json compile.workflows for source lists when present (see package-model.md).

Note: dockpipe build runs this command with --force so existing compiled trees are replaced.

Options:
  --workdir <path>   Project directory (default: directory with dockpipe.config.json, walking up from cwd; else cwd)
  --force            Replace existing packages/core and tarball outputs under packages/workflows/
  --no-staging       Filter out .staging/* paths when resolving defaults or config lists
  --with-bundles, --skip-bundles   Ignored (compatibility no-ops)

`
