package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

var (
	repoRootAppFn         = infrastructure.RepoRoot
	loadWorkflowAppFn     = infrastructure.LoadWorkflow
	loadResolverFileAppFn = infrastructure.LoadResolverFile
	templateBuildAppFn    = infrastructure.TemplateBuild
	maybeVersionTagAppFn  = infrastructure.MaybeVersionTag
	resolveActionPathFn   = infrastructure.ResolveActionPath
	sourceHostScriptAppFn = infrastructure.SourceHostScript
	dockerBuildAppFn      = infrastructure.DockerBuild
	runContainerAppFn     = infrastructure.RunContainer
	resolvePreScriptAppFn = infrastructure.ResolvePreScriptPath
	resolveWorkflowAppFn  = infrastructure.ResolveWorkflowScript
	isBundledCommitAppFn  = infrastructure.IsBundledCommitWorktree
	runStepsAppFn         = runSteps
	osExitAppFn           = os.Exit
)

func preScriptsIncludeCloneWorktree(paths []string) bool {
	for _, p := range paths {
		if strings.HasSuffix(p, "clone-worktree.sh") {
			return true
		}
	}
	return false
}

// setUserRepoRootForWorktree sets DOCKPIPE_USER_REPO_ROOT when cwd/--workdir is a clone whose
// origin matches repoURL, so clone-worktree.sh can use git worktree add from that checkout
// (current HEAD + optional stash of uncommitted work) instead of a mirror from origin/HEAD.
func setUserRepoRootForWorktree(env map[string]string, opts *CliOpts, repoURL string) {
	gitDir, err := os.Getwd()
	if err != nil {
		return
	}
	if opts.Workdir != "" {
		gitDir = opts.Workdir
	}
	top, err := infrastructure.GitTopLevel(gitDir)
	if err != nil {
		return
	}
	origin, err := infrastructure.GitRemoteGetURL(gitDir, "origin")
	if err != nil {
		return
	}
	if !infrastructure.RepoURLsEquivalent(origin, repoURL) {
		return
	}
	env["DOCKPIPE_USER_REPO_ROOT"] = top
	fmt.Fprintf(os.Stderr, "[dockpipe] Worktree will branch from your local repo: %s\n", top)
}

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
	if argv[0] == "windows" {
		return cmdWindows(argv[1:])
	}

	repoRoot, err := repoRootAppFn()
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
	if err := ensureHostBash(); err != nil {
		return err
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
		wf, err = loadWorkflowAppFn(wfConfig)
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
		rm, err := loadResolverFileAppFn(resFile)
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
				opts.PreScripts = append(opts.PreScripts, resolveWorkflowAppFn(r, wfRoot, repoRoot))
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] Using run from workflow\n")
		}
		if opts.Action == "" {
			act := wf.Act
			if act == "" {
				act = wf.Action
			}
			if act != "" {
				opts.Action = resolveWorkflowAppFn(act, wfRoot, repoRoot)
				fmt.Fprintf(os.Stderr, "[dockpipe] Using act from workflow\n")
			}
		}
	}

	// llm-worktree flow: clone-worktree.sh needs DOCKPIPE_REPO_URL. Without --repo we infer
	// origin from the current checkout (or --workdir) so workflow runs don't silently skip.
	if !stepsMode && opts.RepoURL == "" && preScriptsIncludeCloneWorktree(opts.PreScripts) {
		gitDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot infer --repo (getwd): %w", err)
		}
		if opts.Workdir != "" {
			gitDir = opts.Workdir
		}
		u, err := infrastructure.GitRemoteGetURL(gitDir, "origin")
		if err != nil {
			return fmt.Errorf("workflow run includes clone-worktree.sh but --repo was not set and origin URL could not be read from %s: %w\n(hint: pass --repo <url> or run from a git clone with `git remote add origin ...`)", gitDir, err)
		}
		opts.RepoURL = u
		fmt.Fprintf(os.Stderr, "[dockpipe] No --repo; using git origin URL from %s\n", gitDir)
	}

	userIso := opts.Isolate
	userAct := opts.Action

	var image, buildDir, buildCtx string
	effectiveTemplate := templateName
	commitOnHost := false
	actionForContainer := opts.Action

	if !stepsMode {
		if opts.Isolate != "" {
			if im, dir, ok := templateBuildAppFn(repoRoot, opts.Isolate); ok {
				effectiveTemplate = opts.Isolate
				image, buildDir, buildCtx = im, dir, repoRoot
			} else {
				image = opts.Isolate
			}
		} else if templateName != "" {
			effectiveTemplate = templateName
			if im, dir, ok := templateBuildAppFn(repoRoot, templateName); ok {
				image, buildDir, buildCtx = im, dir, repoRoot
			}
		}
		if image == "" {
			image, buildDir = "dockpipe-base-dev", filepath.Join(repoRoot, "images/base-dev")
			buildCtx = repoRoot
		}
		image = maybeVersionTagAppFn(repoRoot, image)

		cwd, _ := os.Getwd()
		if opts.Action != "" {
			ap, err := resolveActionPathFn(opts.Action, repoRoot, cwd)
			if err != nil {
				return err
			}
			if _, err := os.Stat(ap); err != nil {
				return fmt.Errorf("action script not found: %s", ap)
			}
			opts.Action = ap
			actionForContainer = ap
			if isBundledCommitAppFn(ap, repoRoot) {
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
			slug, err := domain.RandomWorkBranchSlug()
			if err != nil {
				return fmt.Errorf("generate work branch name: %w", err)
			}
			wb = prefix + "/" + slug
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

	// Drop inherited DOCKPIPE_WORKDIR before pre-scripts: a leftover export (or workflow .env)
	// would be passed into bash and survive env -0 if clone-worktree ever exited without exporting.
	if !stepsMode && preScriptsIncludeCloneWorktree(opts.PreScripts) {
		if opts.RepoURL != "" {
			setUserRepoRootForWorktree(envMap, opts, opts.RepoURL)
		}
		delete(envMap, "DOCKPIPE_WORKDIR")
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
			firstStepExtra = append(firstStepExtra, resolvePreScriptAppFn(p, repoRoot))
		}
		opts.PreScripts = nil
	} else {
		resolvedPre := make([]string, 0, len(opts.PreScripts))
		for _, p := range opts.PreScripts {
			resolvedPre = append(resolvedPre, resolvePreScriptAppFn(p, repoRoot))
		}
		opts.PreScripts = resolvedPre
	}

	needsHostGit := opts.RepoURL != "" || commitOnHost
	if !stepsMode {
		needsHostGit = needsHostGit || preScriptsIncludeCloneWorktree(opts.PreScripts)
	} else {
		for _, p := range firstStepExtra {
			if strings.HasSuffix(p, "clone-worktree.sh") {
				needsHostGit = true
				break
			}
		}
	}
	if needsHostGit {
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("git not found in PATH — required on the host for clone/worktree/commit flows (by design; dockpipe invokes your normal git). Install git (e.g. Git for Windows on Windows, git from your OS package manager on Linux). HTTPS/SSH auth uses your existing git setup (Credential Manager, SSH keys, etc.)")
		}
	}

	if !stepsMode {
		for _, p := range opts.PreScripts {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("pre-script not found: %s", p)
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] Running pre-script: %s\n", p)
			em, err := sourceHostScriptAppFn(p, envSlice)
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

	dockerEnvMap := domain.EnvSliceToMap(opts.ExtraEnvLines)
	workHostForEnv := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
	mergeWorktreeGitDockerEnv(dockerEnvMap, workHostForEnv)
	extraDocker := domain.EnvMapToSlice(dockerEnvMap)

	if stepsMode {
		return runStepsAppFn(runStepsOpts{
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
		if err := dockerBuildAppFn(image, buildDir, buildCtx); err != nil {
			return err
		}
	}

	workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
	if commitOnHost && strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] Mounting /work from host: %s\n", envMap["DOCKPIPE_WORKDIR"])
	}
	if strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
		if b := dockerEnvMap["DOCKPIPE_WORKTREE_BRANCH"]; b != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Passing to container: DOCKPIPE_WORKTREE_BRANCH=%s\n", b)
		} else if dockerEnvMap["DOCKPIPE_WORKTREE_DETACHED"] == "1" {
			fmt.Fprintln(os.Stderr, "[dockpipe] Passing to container: DOCKPIPE_WORKTREE_DETACHED=1")
		}
	}

	rc, err := runContainerAppFn(infrastructure.RunOpts{
		Image:         image,
		WorkdirHost:   workHost,
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
		BundleAll:     strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1",
	}, rest)
	if err != nil {
		return err
	}
	if rc != 0 {
		osExitAppFn(rc)
	}
	return nil
}
