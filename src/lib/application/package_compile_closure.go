package application

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"

	"gopkg.in/yaml.v3"
)

// compileClosureForWorkflow compiles core (if missing), then resolver tarballs and workflow tarballs
// for the transitive closure of workflowName: config.yml inject:, package.yml depends, requires_resolvers,
// resolver/runtime names on the workflow and steps, and nested delegate workflows from merged isolation profiles.
// projectRoot is the DockPipe project directory (contains bin/.dockpipe and usually dockpipe.config.json).
func compileClosureForWorkflow(projectRoot, workflowName string, force bool) error {
	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return err
	}
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	cfg, err := loadDockpipeProjectConfig(projectRoot)
	if err != nil {
		return err
	}

	startDir, err := infrastructure.WorkflowCompileStartDir(repoRoot, projectRoot, workflowName)
	if err != nil {
		return fmt.Errorf("compile for-workflow: workflow %q: %w", workflowName, err)
	}

	order, resNames, err := closureWorkflowOrderAndResolvers(repoRoot, projectRoot, startDir, cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compile for-workflow %q: %d workflow(s), %d resolver name(s)\n", workflowName, len(order), len(resNames))

	if err := ensureCoreCompiled(projectRoot, cfg, force); err != nil {
		return err
	}

	destRes, err := infrastructure.PackagesResolversDir(projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destRes, 0o755); err != nil {
		return err
	}
	var defResolverNS string
	if cfg != nil && cfg.Packages.Namespace != nil {
		defResolverNS = strings.TrimSpace(*cfg.Packages.Namespace)
	}
	for name := range resNames {
		leaves := infrastructure.NestedResolverLeafDirs(name, infrastructure.ResolverCompileRootsCached(projectRoot))
		if len(leaves) == 0 {
			fmt.Fprintf(os.Stderr, "[dockpipe] compile for-workflow: skip resolver %q (no profile tree under compile roots)\n", name)
			continue
		}
		from := leaves[0]
		if err := compileSingleResolverDir(destRes, from, filepath.Base(from), defResolverNS, authoredPackageVersion(projectRoot), force); err != nil {
			return fmt.Errorf("resolver %q: %w", name, err)
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] compiled resolver %q → %s\n", name, destRes)
	}

	for _, wfDir := range order {
		if err := compileWorkflowOne(projectRoot, wfDir, "", force); err != nil {
			return fmt.Errorf("workflow %s: %w", wfDir, err)
		}
	}
	return validateCompileOutputs(projectRoot)
}

func ensureCoreCompiled(projectRoot string, cfg *domain.DockpipeProjectConfig, force bool) error {
	pkgs, err := infrastructure.PackagesRoot(projectRoot)
	if err != nil {
		return err
	}
	coreDir := filepath.Join(pkgs, "core")
	matches, _ := filepath.Glob(filepath.Join(coreDir, "dockpipe-core-*.tar.gz"))
	if len(matches) > 0 && !force {
		return nil
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compile for-workflow: compiling core spine (missing under %s)\n", coreDir)
	args := []string{"--workdir", projectRoot}
	if force {
		args = append(args, "--force")
	}
	return cmdPackageCompileCore(args)
}

// closureWorkflowOrderAndResolvers returns workflow source dirs in dependency order (dependencies first)
// and a set of resolver profile names to compile.
// dockpipeRepoRoot is the DockPipe engine checkout (templates/core); projectRoot is the project being compiled.
func closureWorkflowOrderAndResolvers(dockpipeRepoRoot, projectRoot, startDir string, cfg *domain.DockpipeProjectConfig) ([]string, map[string]bool, error) {
	wfRoots := domain.EffectiveWorkflowCompileRoots(cfg, projectRoot)
	visited := make(map[string]bool)
	var order []string
	resNames := make(map[string]bool)

	var visit func(string) error
	visit = func(wfDir string) error {
		k := filepath.Clean(wfDir)
		if visited[k] {
			return nil
		}
		visited[k] = true

		cfgPath := filepath.Join(k, "config.yml")
		b, err := os.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", cfgPath, err)
		}
		var wf domain.Workflow
		if err := yaml.Unmarshal(b, &wf); err != nil {
			return fmt.Errorf("parse %s: %w", cfgPath, err)
		}

		for _, ent := range wf.Inject {
			if name := ent.WorkflowManifestName(); name != "" {
				depDir := findWorkflowSourceDir(projectRoot, name, wfRoots)
				if depDir == "" {
					fmt.Fprintf(os.Stderr, "[dockpipe] compile for-workflow: warning: inject workflow %q not found under compile.workflows — skip\n", name)
					continue
				}
				if err := visit(depDir); err != nil {
					return err
				}
			}
			addResolverName(resNames, ent.Resolver)
		}

		pmPath := filepath.Join(k, infrastructure.PackageManifestFilename)
		if pb, err := os.ReadFile(pmPath); err == nil {
			var pm domain.PackageManifest
			if err := yaml.Unmarshal(pb, &pm); err == nil {
				for _, dep := range pm.Depends {
					dep = strings.TrimSpace(dep)
					if dep == "" {
						continue
					}
					depDir := findWorkflowSourceDir(projectRoot, dep, wfRoots)
					if depDir == "" {
						fmt.Fprintf(os.Stderr, "[dockpipe] compile for-workflow: warning: depends %q not found under compile.workflows — skip\n", dep)
						continue
					}
					if err := visit(depDir); err != nil {
						return err
					}
				}
				for _, r := range pm.RequiresResolvers {
					addResolverName(resNames, r)
				}
			}
		}

		addResolverName(resNames, EffectiveResolverProfileName(nil, &wf, true))
		addResolverName(resNames, EffectiveRuntimeProfileName(nil, &wf, true))
		if iso := strings.TrimSpace(wf.Isolate); iso != "" {
			addResolverName(resNames, iso)
		}
		for i := range wf.Steps {
			st := &wf.Steps[i]
			addResolverName(resNames, st.Runtime)
			addResolverName(resNames, st.Resolver)
			rt, rs := strings.TrimSpace(st.Runtime), strings.TrimSpace(st.Resolver)
			m, err := infrastructure.LoadIsolationProfile(dockpipeRepoRoot, rt, rs)
			if err != nil {
				continue
			}
			for _, key := range []string{"DOCKPIPE_RUNTIME_WORKFLOW", "DOCKPIPE_RESOLVER_WORKFLOW"} {
				nested := strings.TrimSpace(m[key])
				if nested == "" {
					continue
				}
				nestedDir := findWorkflowSourceDir(projectRoot, nested, wfRoots)
				if nestedDir == "" {
					fmt.Fprintf(os.Stderr, "[dockpipe] compile for-workflow: warning: nested workflow %q not found — skip\n", nested)
					continue
				}
				if err := visit(nestedDir); err != nil {
					return err
				}
			}
		}

		order = append(order, k)
		return nil
	}

	if err := visit(startDir); err != nil {
		return nil, nil, err
	}
	return order, resNames, nil
}

func addResolverName(set map[string]bool, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	nk := infrastructure.NormalizeRuntimeProfileName(name)
	if nk == "dockerimage" || nk == "dockerfile" || nk == "package" {
		return
	}
	set[name] = true
}

func findWorkflowSourceDir(projectRoot, ref string, wfRoots []string) string {
	if d := findWorkflowDirByPackageManifestName(projectRoot, ref, wfRoots); d != "" {
		return d
	}
	return infrastructure.FindNestedWorkflowDirByLeafName(projectRoot, ref)
}

func findWorkflowDirByPackageManifestName(projectRoot, ref string, wfRoots []string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	for _, root := range wfRoots {
		var hit string
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Base(path) != infrastructure.PackageManifestFilename {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			var pm domain.PackageManifest
			if err := yaml.Unmarshal(b, &pm); err != nil {
				return nil
			}
			if strings.TrimSpace(pm.Name) == ref {
				hit = filepath.Dir(path)
				return fs.SkipAll
			}
			return nil
		})
		if hit != "" {
			return hit
		}
	}
	return ""
}

func cmdPackageCompileForWorkflow(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileForWorkflowUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		wfName  string
		force   bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--workflow" && i+1 < len(args):
			wfName = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile for-workflow --help)", args[i])
		default:
			if wfName == "" {
				wfName = args[i]
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
	if strings.TrimSpace(wfName) == "" {
		return fmt.Errorf("missing workflow name (use --workflow <name> or a positional name)")
	}
	return compileClosureForWorkflow(workdir, wfName, force)
}

const packageCompileForWorkflowUsageText = `dockpipe package compile for-workflow <name>

Compiles only the core spine (if missing), resolver packs, and workflow packages needed for the
transitive closure of the named workflow: config.yml inject: (explicit workflow/resolver deps),
package.yml depends, requires_resolvers, resolver/runtime names on the workflow and steps, and nested
delegate workflows (DOCKPIPE_*_WORKFLOW from merged profiles). Dependencies are compiled before dependents.

Does not replace a full "package compile all" — only what this workflow needs.

Options:
  --workdir <path>   Project directory (default: current directory)
  --workflow <name>  Workflow name (same as dockpipe run --workflow)
  --force            Replace existing compiled tarballs

`
