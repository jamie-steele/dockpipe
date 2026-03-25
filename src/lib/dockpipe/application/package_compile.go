package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"

	"gopkg.in/yaml.v3"
)

func cmdPackageCompile(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(packageCompileUsageText)
		return nil
	}
	switch args[0] {
	case "workflow":
		return cmdPackageCompileWorkflow(args[1:])
	case "core":
		return cmdPackageCompileCore(args[1:])
	case "resolvers":
		return cmdPackageCompileResolvers(args[1:])
	case "bundles":
		return cmdPackageCompileBundles(args[1:])
	case "workflows":
		return cmdPackageCompileWorkflowsBatch(args[1:])
	case "all":
		return cmdPackageCompileAll(args[1:])
	default:
		return fmt.Errorf("unknown package compile target %q (try: dockpipe package compile --help)", args[0])
	}
}

func cmdPackageCompileWorkflow(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileWorkflowUsageText)
		return nil
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

// compileWorkflowOne validates YAML, copies into packages/workflows/<name>/, seeds package.yml if missing.
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
	pkgName := strings.TrimSpace(name)
	if pkgName == "" {
		pkgName = strings.TrimSpace(wf.Name)
	}
	if pkgName == "" {
		pkgName = filepath.Base(srcAbs)
	}
	destRoot, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return err
	}
	destRoot = filepath.Join(destRoot, pkgName)
	if _, err := os.Stat(destRoot); err == nil {
		if !force {
			return fmt.Errorf("destination exists: %s (use --force to replace)", destRoot)
		}
		if err := os.RemoveAll(destRoot); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}
	if err := copyDir(srcAbs, destRoot); err != nil {
		return fmt.Errorf("copy workflow: %w", err)
	}
	manifestPath := filepath.Join(destRoot, infrastructure.PackageManifestFilename)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		pm := map[string]any{
			"schema":       1,
			"name":         pkgName,
			"version":      "0.1.0",
			"title":        pkgName,
			"description":  "Compiled from " + srcAbs,
			"kind":         "workflow",
			"allow_clone":  true,
			"distribution": "source",
		}
		out, err := yaml.Marshal(pm)
		if err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compiled workflow package → %s\n", destRoot)
	return nil
}

func cmdPackageCompileCore(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileCoreUsageText)
		return nil
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
	destRoot, err := infrastructure.PackagesCoreDir(workdir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(destRoot); err == nil {
		if !force {
			return fmt.Errorf("destination exists: %s (use --force to replace)", destRoot)
		}
		if err := os.RemoveAll(destRoot); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}
	if err := copyDir(srcAbs, destRoot); err != nil {
		return fmt.Errorf("copy core: %w", err)
	}
	manifestPath := filepath.Join(destRoot, infrastructure.PackageManifestFilename)
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
	fmt.Fprintf(os.Stderr, "[dockpipe] compiled core package → %s\n", destRoot)
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

func requirePackagesCore(workdir string) (string, error) {
	dest, err := infrastructure.PackagesCoreDir(workdir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dest); err != nil {
		return "", fmt.Errorf("missing compiled core at %s — run `dockpipe package compile core` first (or `dockpipe package compile all`)", dest)
	}
	return dest, nil
}

func cmdPackageCompileResolvers(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileResolversUsageText)
		return nil
	}
	var (
		workdir    string
		from       []string
		noStaging  bool
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
		return fmt.Errorf("no resolver source directories (set compile.resolvers in %s or pass --from)", domain.DockpipeProjectConfigFileName)
	}
	destCore, err := requirePackagesCore(workdir)
	if err != nil {
		return err
	}
	destRes := filepath.Join(destCore, "resolvers")
	if err := os.MkdirAll(destRes, 0o755); err != nil {
		return err
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
		n, err := mergeChildPackages(srcAbs, destRes, "resolver")
		if err != nil {
			return err
		}
		total += n
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] merged %d resolver director(y/ies) → %s\n", total, destRes)
	return nil
}

func cmdPackageCompileBundles(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileBundlesUsageText)
		return nil
	}
	var (
		workdir   string
		from      []string
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
		case args[i] == "--no-staging":
			noStaging = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile bundles --help)", args[i])
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
		from = effectiveBundleCompileRoots(cfg, repoRoot, noStaging)
	}
	if len(from) == 0 {
		return fmt.Errorf("no bundle source directories (set compile.bundles in %s, pass --from, or create .staging/bundles)", domain.DockpipeProjectConfigFileName)
	}
	destCore, err := requirePackagesCore(workdir)
	if err != nil {
		return err
	}
	destB := filepath.Join(destCore, "bundles")
	if err := os.MkdirAll(destB, 0o755); err != nil {
		return err
	}
	total := 0
	for _, root := range from {
		srcAbs, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			return err
		}
		if st, err := os.Stat(srcAbs); err != nil || !st.IsDir() {
			fmt.Fprintf(os.Stderr, "[dockpipe] skip missing bundles root: %s\n", srcAbs)
			continue
		}
		n, err := mergeChildPackages(srcAbs, destB, "bundle")
		if err != nil {
			return err
		}
		total += n
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] merged %d bundle director(y/ies) → %s\n", total, destB)
	return nil
}

// mergeChildPackages copies each immediate child directory from srcRoot into destRoot/<name>,
// replacing any existing destination of the same name (overlay merge for compile resolvers/bundles).
func mergeChildPackages(srcRoot, destRoot string, kind string) (int, error) {
	entries, err := os.ReadDir(srcRoot)
	if err != nil {
		return 0, err
	}
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
		to := filepath.Join(destRoot, name)
		if _, err := os.Stat(to); err == nil {
			if err := os.RemoveAll(to); err != nil {
				return n, fmt.Errorf("remove %s: %w", to, err)
			}
		}
		if err := copyDir(from, to); err != nil {
			return n, fmt.Errorf("copy %s %s: %w", kind, name, err)
		}
		manifestPath := filepath.Join(to, infrastructure.PackageManifestFilename)
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
			out, err := yaml.Marshal(pm)
			if err != nil {
				return n, err
			}
			if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
				return n, err
			}
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
		entries, err := os.ReadDir(rootAbs)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			wfDir := filepath.Join(rootAbs, name)
			cfg := filepath.Join(wfDir, "config.yml")
			if _, err := os.Stat(cfg); err != nil {
				continue
			}
			if _, ok := seen[name]; ok {
				fmt.Fprintf(os.Stderr, "[dockpipe] skip duplicate workflow name %q (already compiled from an earlier --from)\n", name)
				continue
			}
			if err := compileWorkflowOne(workdir, wfDir, "", force); err != nil {
				return fmt.Errorf("workflow %q: %w", name, err)
			}
			seen[name] = struct{}{}
			total++
		}
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compiled %d workflow package(s) under .dockpipe/internal/packages/workflows/\n", total)
	return nil
}

func cmdPackageCompileAll(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileAllUsageText)
		return nil
	}
	var (
		workdir    string
		force      bool
		noStaging  bool
		skipBundle bool
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
		case args[i] == "--skip-bundles":
			skipBundle = true
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
	fmt.Fprintf(os.Stderr, "[dockpipe] compile all: core → resolvers → bundles → workflows\n")
	if err := cmdPackageCompileCore(workdirAndForceArgs(workdir, force)); err != nil {
		return err
	}
	resRoots := effectiveResolverCompileRoots(cfg, repoRoot, noStaging)
	if len(resRoots) == 0 {
		fmt.Fprintf(os.Stderr, "[dockpipe] compile all: skip resolvers (no source dirs)\n")
	} else {
		resArgs := []string{"--workdir", workdir}
		for _, p := range resRoots {
			resArgs = append(resArgs, "--from", p)
		}
		if err := cmdPackageCompileResolvers(resArgs); err != nil {
			return err
		}
	}
	if !skipBundle && !noStaging {
		bundleRoots := effectiveBundleCompileRoots(cfg, repoRoot, noStaging)
		if len(bundleRoots) == 0 {
			fmt.Fprintf(os.Stderr, "[dockpipe] compile all: skip bundles (no roots)\n")
		} else {
			bArgs := []string{"--workdir", workdir}
			for _, p := range bundleRoots {
				bArgs = append(bArgs, "--from", p)
			}
			if err := cmdPackageCompileBundles(bArgs); err != nil {
				return err
			}
		}
	}
	wfArgs := workdirAndForceArgs(workdir, force)
	for _, p := range effectiveWorkflowCompileRoots(cfg, repoRoot, noStaging) {
		wfArgs = append(wfArgs, "--from", p)
	}
	return cmdPackageCompileWorkflowsBatch(wfArgs)
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
  dockpipe package compile bundles [options]
  dockpipe package compile workflows [options]
  dockpipe package compile all [options]
  dockpipe package compile workflow [options] [--from] <source-dir>

Order for a full local store: core first, then resolvers and bundles (merge into packages/core/),
then workflows (each under packages/workflows/<name>/). Use "compile all" to run that sequence.

`

const packageCompileWorkflowUsageText = `dockpipe package compile workflow <source-dir>

Runs workflow YAML validation (same rules as dockpipe workflow validate), then copies the
directory into <workdir>/.dockpipe/internal/packages/workflows/<name>/.

Options:
  --workdir <path>   Project directory (default: current directory)
  --from <path>      Source workflow directory (same as positional <source-dir>)
  --name <n>         Package folder name (default: workflow name from config.yml, else basename of source)
  --force            Replace existing package directory

`

const packageCompileCoreUsageText = `dockpipe package compile core

Copies a core authoring tree (default: src/core or templates/core when present) into
<workdir>/.dockpipe/internal/packages/core/ and writes package.yml (kind: core).

Optional dockpipe.config.json "compile.core_from" overrides the default core path when --from is omitted.

Options:
  --workdir <path>   Project directory (default: current directory)
  --from <path>      Source core root (must contain category dirs such as runtimes/, resolvers/)
  --force            Replace existing packages/core tree

`

const packageCompileResolversUsageText = `dockpipe package compile resolvers

Requires an existing compile core output. Merges each child directory from each --from
source into .dockpipe/internal/packages/core/resolvers/<name>/ (later --from wins on name clash).

Defaults come from dockpipe.config.json compile.resolvers when present, else
src/core/resolvers, templates/core/resolvers, then .staging/resolvers (existing dirs only).

Options:
  --workdir <path>      Project directory (default: current directory)
  --from <path>         Repeatable; each root's subdirectories are resolver profiles
  --no-staging          Skip paths under .staging/ when using defaults

`

const packageCompileBundlesUsageText = `dockpipe package compile bundles

Requires compile core. Merges each child directory from each --from into
.dockpipe/internal/packages/core/bundles/<name>/.

Defaults: dockpipe.config.json compile.bundles, or .staging/bundles when present.

Options:
  --workdir <path>      Project directory (default: current directory)
  --from <path>         Repeatable
  --no-staging          Skip .staging paths when using defaults

`

const packageCompileWorkflowsUsageText = `dockpipe package compile workflows

Compiles every immediate subdirectory that contains config.yml under each --from root.

Defaults: dockpipe.config.json compile.workflows, else workflows/ and .staging/workflows/ when present.

Options:
  --workdir <path>       Project directory (default: current directory)
  --from <path>          Repeatable; roots to scan for named workflow folders
  --force                Replace existing packages/workflows/<name>
  --no-staging           Skip .staging paths when using defaults

`

const packageCompileAllUsageText = `dockpipe package compile all

Runs: compile core → compile resolvers → compile bundles (when roots exist) → compile workflows.
Uses dockpipe.config.json for source lists when present (see package-model.md).

Options:
  --workdir <path>   Project directory (default: current directory)
  --force            Pass --force to core and workflow compile steps
  --no-staging       Filter out .staging/* paths when resolving defaults or config lists
  --skip-bundles     Do not run the bundles merge step

`
