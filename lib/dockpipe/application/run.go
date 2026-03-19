package application

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

// Run is the CLI entry (after stripping os.Args[0]).
func Run(argv []string, baseEnviron []string) error {
	if len(argv) == 0 {
		printUsage()
		return nil
	}
	if argv[0] == "init" {
		return cmdInit(argv[1:])
	}
	if argv[0] == "action" {
		return cmdAction(argv[1:])
	}
	if argv[0] == "pre" {
		return cmdPre(argv[1:])
	}
	if argv[0] == "template" {
		return cmdTemplate(argv[1:])
	}

	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return err
	}

	rest, opts, err := ParseFlags(repoRoot, argv)
	if err != nil {
		return err
	}
	if opts.Help {
		printUsage()
		return nil
	}

	var wf *domain.Workflow
	var wfRoot, wfConfig string
	stepsMode := false
	if opts.Workflow != "" {
		wfRoot = filepath.Join(repoRoot, "templates", opts.Workflow)
		wfConfig = filepath.Join(wfRoot, "config.yml")
		if _, err := os.Stat(wfConfig); err != nil {
			return fmt.Errorf("workflow %q not found (expected %s)", opts.Workflow, wfConfig)
		}
		wf, err = infrastructure.LoadWorkflow(wfConfig)
		if err != nil {
			return fmt.Errorf("parse config: %w", err)
		}
		if len(wf.Steps) > 0 {
			stepsMode = true
			fmt.Fprintf(os.Stderr, "[dockpipe] Multi-step workflow (%s)\n", opts.Workflow)
		}
	}

	envMap := domain.EnvironToMap(baseEnviron)
	if wf != nil {
		buildWorkflowEnvInto(envMap, wf, wfRoot, repoRoot, opts)
	}

	resolver := opts.Resolver
	if resolver == "" && wf != nil {
		if stepsMode {
			resolver = wf.Resolver
			if resolver == "" {
				resolver = wf.DefaultResolver
			}
		} else {
			resolver = wf.Isolate
			if resolver == "" {
				resolver = wf.DefaultResolver
			}
		}
		if resolver != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Using resolver from workflow: %s\n", resolver)
		}
	}

	templateName := ""
	var preFromResolver, actFromResolver string
	if resolver != "" {
		resBase := filepath.Join(repoRoot, "templates", "llm-worktree", "resolvers")
		if wfRoot != "" {
			resBase = filepath.Join(wfRoot, "resolvers")
		}
		resFile := filepath.Join(resBase, resolver)
		rm, err := infrastructure.LoadResolverFile(resFile)
		if err != nil {
			return fmt.Errorf("resolver %q not found (expected %s)", resolver, resFile)
		}
		ra := domain.FromResolverMap(rm)
		templateName = ra.Template
		preFromResolver = ra.PreScript
		actFromResolver = ra.Action
		if opts.Workflow == "" {
			if len(opts.PreScripts) == 0 && preFromResolver != "" {
				opts.PreScripts = []string{filepath.Join(repoRoot, preFromResolver)}
			}
			if opts.Action == "" && actFromResolver != "" {
				opts.Action = filepath.Join(repoRoot, actFromResolver)
			}
		}
	}

	locked := lockedKeys(opts.VarOverrides)

	if !stepsMode && wf != nil {
		if len(opts.PreScripts) == 0 && len(wf.Run) > 0 {
			for _, r := range wf.Run {
				opts.PreScripts = append(opts.PreScripts, infrastructure.ResolveWorkflowScript(r, wfRoot, repoRoot))
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] Using run from workflow\n")
		}
		if opts.Action == "" {
			act := wf.Act
			if act == "" {
				act = wf.Action
			}
			if act != "" {
				opts.Action = infrastructure.ResolveWorkflowScript(act, wfRoot, repoRoot)
				fmt.Fprintf(os.Stderr, "[dockpipe] Using act from workflow\n")
			}
		}
	}

	userIso := opts.Isolate
	userAct := opts.Action

	var image, buildDir, buildCtx string
	effectiveTemplate := templateName
	commitOnHost := false
	actionForContainer := opts.Action

	if !stepsMode {
		if opts.Isolate != "" {
			if im, dir, ok := infrastructure.TemplateBuild(repoRoot, opts.Isolate); ok {
				effectiveTemplate = opts.Isolate
				image, buildDir, buildCtx = im, dir, repoRoot
			} else {
				image = opts.Isolate
			}
		} else if templateName != "" {
			effectiveTemplate = templateName
			if im, dir, ok := infrastructure.TemplateBuild(repoRoot, templateName); ok {
				image, buildDir, buildCtx = im, dir, repoRoot
			}
		}
		if image == "" {
			image, buildDir = "dockpipe-base-dev", filepath.Join(repoRoot, "images/base-dev")
			buildCtx = repoRoot
		}
		image = infrastructure.MaybeVersionTag(repoRoot, image)

		cwd, _ := os.Getwd()
		if opts.Action != "" {
			ap, err := infrastructure.ResolveActionPath(opts.Action, repoRoot, cwd)
			if err != nil {
				return err
			}
			if _, err := os.Stat(ap); err != nil {
				return fmt.Errorf("action script not found: %s", ap)
			}
			opts.Action = ap
			actionForContainer = ap
			if infrastructure.IsBundledCommitWorktree(ap, repoRoot) {
				commitOnHost = true
				actionForContainer = ""
				applyBranchPrefix(envMap, resolver, effectiveTemplate)
				mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
			}
		}
	}

	dataVol := opts.DataVolume
	dataDir := opts.DataDir
	if opts.NoData {
		dataVol, dataDir = "", ""
	} else if dataDir == "" && dataVol == "" {
		dataVol = "dockpipe-data"
	}

	if opts.RepoURL != "" && opts.RepoBranch == "" {
		prefix := envMap["DOCKPIPE_BRANCH_PREFIX"]
		if prefix == "" {
			prefix = resolver
		}
		if prefix == "" {
			prefix = domain.BranchPrefixForTemplate(effectiveTemplate)
		}
		wb := opts.WorkBranch
		if wb == "" {
			wb = prefix + "/agent-" + time.Now().Format("20060102-150405")
		}
		opts.RepoBranch = wb
		envMap["DOCKPIPE_REPO_BRANCH"] = wb
		fmt.Fprintf(os.Stderr, "[dockpipe] No --branch; using new branch: %s\n", wb)
	}

	if opts.RepoURL != "" && len(opts.PreScripts) == 0 && !stepsMode {
		if dataDir == "" {
			home, _ := os.UserHomeDir()
			if home == "" {
				home = "/tmp"
			}
			dataDir = filepath.Join(home, ".dockpipe")
			fmt.Fprintf(os.Stderr, "[dockpipe] Using %s for worktree (set --data-dir to override)\n", dataDir)
			dataVol = ""
		}
		opts.PreScripts = []string{filepath.Join(repoRoot, "scripts/clone-worktree.sh")}
	}

	envSlice := domain.EnvMapToSlice(envMap)
	// Inject standard repo / job env for pre-scripts
	envSlice = appendUniqueEnv(envSlice, "DOCKPIPE_REPO_ROOT="+repoRoot)
	if opts.RepoURL != "" {
		envSlice = appendUniqueEnv(envSlice, "DOCKPIPE_REPO_URL="+opts.RepoURL)
	}
	if opts.RepoBranch != "" {
		envSlice = appendUniqueEnv(envSlice, "DOCKPIPE_REPO_BRANCH="+opts.RepoBranch)
	}
	if dataDir != "" {
		envSlice = appendUniqueEnv(envSlice, "DOCKPIPE_DATA_DIR="+dataDir)
	}
	if resolver != "" {
		envSlice = appendUniqueEnv(envSlice, "RESOLVER="+resolver)
	}
	if templateName != "" {
		envSlice = appendUniqueEnv(envSlice, "TEMPLATE="+templateName)
	}
	if opts.Workdir != "" {
		envSlice = appendUniqueEnv(envSlice, "DOCKPIPE_WORKDIR="+opts.Workdir)
	}

	var firstStepExtra []string
	if stepsMode {
		for _, p := range opts.PreScripts {
			firstStepExtra = append(firstStepExtra, infrastructure.ResolvePreScriptPath(p, repoRoot))
		}
		opts.PreScripts = nil
	} else {
		resolvedPre := make([]string, 0, len(opts.PreScripts))
		for _, p := range opts.PreScripts {
			resolvedPre = append(resolvedPre, infrastructure.ResolvePreScriptPath(p, repoRoot))
		}
		opts.PreScripts = resolvedPre
	}

	if !stepsMode {
		for _, p := range opts.PreScripts {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("pre-script not found: %s", p)
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] Running pre-script: %s\n", p)
			em, err := infrastructure.SourceHostScript(p, envSlice)
			if err != nil {
				return err
			}
			for k, v := range em {
				envMap[k] = v
			}
			envSlice = domain.EnvMapToSlice(envMap)
		}
	}

	if commitOnHost {
		mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
		envSlice = domain.EnvMapToSlice(envMap)
	}

	if !opts.SeenDash && !stepsMode {
		return fmt.Errorf("expected -- before command (e.g. dockpipe -- ls -la)")
	}
	if len(rest) == 0 && !stepsMode {
		return fmt.Errorf("no command given after --")
	}

	extraDocker := domain.EnvMapToSlice(domain.EnvSliceToMap(opts.ExtraEnvLines))

	if stepsMode {
		return runSteps(runStepsOpts{
			wf:             wf,
			wfRoot:         wfRoot,
			repoRoot:       repoRoot,
			cliArgs:        rest,
			envMap:         envMap,
			envSlice:       envSlice,
			locked:         locked,
			userIsolate:    userIso,
			userAct:        userAct,
			firstStepExtra: firstStepExtra,
			opts:           opts,
			dataVol:        dataVol,
			dataDir:        dataDir,
			resolver:       resolver,
			templateName:   templateName,
		})
	}

	if buildDir != "" && buildCtx != "" {
		if err := infrastructure.DockerBuild(image, buildDir, buildCtx); err != nil {
			return err
		}
	}

	rc, err := infrastructure.RunContainer(infrastructure.RunOpts{
		Image:         image,
		WorkdirHost:   firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir),
		WorkPath:      opts.WorkPath,
		ActionPath:    actionForContainer,
		ExtraMounts:   opts.ExtraMounts,
		ExtraEnv:      extraDocker,
		DataVolume:    dataVol,
		DataDir:       dataDir,
		Reinit:        opts.Reinit,
		Force:         opts.Force,
		Detach:        opts.Detach,
		CommitOnHost:  commitOnHost,
		CommitMessage: envMap["DOCKPIPE_COMMIT_MESSAGE"],
		BundleOut:     firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], opts.BundleOut),
	}, rest)
	if err != nil {
		return err
	}
	if rc != 0 {
		os.Exit(rc)
	}
	return nil
}
