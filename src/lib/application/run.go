package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

var (
	repoRootAppFn          = infrastructure.RepoRoot
	loadWorkflowAppFn      = infrastructure.LoadWorkflow
	loadResolverFileAppFn  = infrastructure.LoadResolverFile
	templateBuildAppFn     = infrastructure.TemplateBuild
	maybeVersionTagAppFn   = infrastructure.MaybeVersionTag
	resolveActionPathFn    = infrastructure.ResolveActionPath
	sourceHostScriptAppFn  = infrastructure.SourceHostScript
	runHostScriptAppFn     = infrastructure.RunHostScript
	dockerBuildAppFn       = infrastructure.DockerBuild
	dockerImageExistsAppFn = infrastructure.DockerImageExists
	dockerPullAppFn        = infrastructure.DockerPull
	runContainerAppFn      = infrastructure.RunContainer
	resolvePreScriptAppFn  = infrastructure.ResolvePreScriptPath
	resolveWorkflowAppFn   = infrastructure.ResolveWorkflowScript
	isBundledCommitAppFn   = infrastructure.IsBundledCommitWorktree
	runStepsAppFn          = runSteps
	osExitAppFn            = os.Exit
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
// (current HEAD; uncommitted changes copied to worktree by default, or stash if DOCKPIPE_STASH_UNCOMMITTED=1) instead of a mirror from origin/HEAD.
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
	fmt.Fprintf(os.Stderr, "[dockpipe] Worktree base: %s (same origin as --repo)\n", top)
}

// Run is the CLI entry (after stripping os.Args[0]).
func Run(argv []string, baseEnviron []string) error {
	if baseEnviron == nil {
		baseEnviron = os.Environ()
	}
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
	if argv[0] == "workflow" {
		return cmdWorkflow(argv[1:])
	}
	if argv[0] == "catalog" {
		return cmdCatalog(argv[1:])
	}
	if argv[0] == "pipelang" {
		return cmdPipeLang(argv[1:])
	}
	if argv[0] == "doctor" {
		return cmdDoctor(argv[1:])
	}
	if argv[0] == "runs" {
		return cmdRuns(argv[1:])
	}
	if argv[0] == "install" {
		return cmdInstall(argv[1:])
	}
	if argv[0] == "package" {
		return cmdPackage(argv[1:])
	}
	if argv[0] == "build" {
		return cmdBuild(argv[1:])
	}
	if argv[0] == "clean" {
		return cmdClean(argv[1:])
	}
	if argv[0] == "rebuild" {
		return cmdRebuild(argv[1:])
	}
	if argv[0] == "compile" {
		return cmdPackage(append([]string{"compile"}, argv[1:]...))
	}
	if argv[0] == "clone" {
		return cmdClone(argv[1:])
	}
	if argv[0] == "release" {
		return cmdRelease(argv[1:])
	}
	if argv[0] == "core" {
		return cmdCore(argv[1:])
	}
	if argv[0] == "terraform" {
		return cmdTerraform(argv[1:])
	}
	if argv[0] == "sdk" {
		return cmdSDK(argv[1:])
	}
	if argv[0] == "get" {
		return cmdGet(argv[1:])
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
	if opts.WorkflowsDir != "" {
		infrastructure.SetWorkflowsDirForProcess(opts.WorkflowsDir)
		defer infrastructure.SetWorkflowsDirForProcess("")
	}
	if err := ensureHostBash(); err != nil {
		return err
	}

	if compileDepsWanted(opts) && opts.Workflow != "" && opts.WorkflowFile == "" {
		effWd := effectiveWorkdirForWorkflowOpts(opts)
		if err := compileClosureForWorkflow(effWd, opts.Workflow, opts.Force); err != nil {
			return err
		}
	}

	var wf *domain.Workflow
	var wfRoot, wfConfig string
	stepsMode := false
	if opts.Workflow != "" && opts.WorkflowFile != "" {
		return fmt.Errorf("use only one of --workflow and --workflow-file")
	}
	if opts.WorkflowFile != "" {
		wfPath, err := filepath.Abs(opts.WorkflowFile)
		if err != nil {
			return fmt.Errorf("workflow file: %w", err)
		}
		if _, err := os.Stat(wfPath); err != nil {
			return fmt.Errorf("workflow file: %w", err)
		}
		wf, err = loadWorkflowAppFn(wfPath)
		if err != nil {
			return fmt.Errorf("parse config: %w", err)
		}
		wfRoot = filepath.Dir(wfPath)
		wfConfig = wfPath
		if len(wf.Steps) > 0 {
			stepsMode = true
		}
	} else if opts.Workflow != "" {
		var statErr error
		effWd := effectiveWorkdirForWorkflowOpts(opts)
		wfConfig, statErr = infrastructure.ResolveWorkflowConfigPathWithWorkdir(repoRoot, effWd, opts.Workflow)
		if statErr != nil {
			if os.Getenv("DOCKPIPE_REPO_ROOT") == "" && infrastructure.EmbeddedWorkflowConfigExists(opts.Workflow) {
				if invErr := infrastructure.InvalidateBundledCache(); invErr == nil {
					repoRoot, err = repoRootAppFn()
					if err != nil {
						return err
					}
					wfConfig, statErr = infrastructure.ResolveWorkflowConfigPathWithWorkdir(repoRoot, effWd, opts.Workflow)
				}
			}
			if statErr != nil {
				names, _ := infrastructure.ListWorkflowNamesInRepoRootAndPackages(repoRoot, effWd)
				msg := fmt.Sprintf("workflow %q not found — tried workflows/ (or DOCKPIPE_WORKFLOWS_DIR), extra roots from dockpipe.config.json compile.workflows, installed workflow packages under bin/.dockpipe/internal/packages/workflows/, built-in bundled workflows for this dockpipe build, and namespaced workflow tarballs (dockpipe-workflow-%[1]s-*.tar.gz under release/artifacts or packages.tarball_dir when config.yml inside the archive sets namespace:)", opts.Workflow)
				if len(names) > 0 {
					msg += fmt.Sprintf(" (available in this install: %s)", strings.Join(names, ", "))
				}
				if !infrastructure.EmbeddedWorkflowConfigExists(opts.Workflow) {
					msg += ". This dockpipe build does not include that workflow; install a newer release or use --workflow-file path/to/config.yml."
				} else {
					msg += ". Try deleting the bundled cache folder under your user cache (dockpipe/bundled-*) or set DOCKPIPE_REPO_ROOT to a full git checkout."
				}
				return fmt.Errorf("%s", msg)
			}
		}
		if strings.HasPrefix(wfConfig, "tar://") {
			wfRoot = filepath.Join(infrastructure.WorkflowsRootDir(repoRoot), opts.Workflow)
		} else {
			wfRoot = filepath.Dir(wfConfig)
		}
		wf, err = loadWorkflowAppFn(wfConfig)
		if err != nil {
			return fmt.Errorf("parse config: %w", err)
		}
		if len(wf.Steps) > 0 {
			stepsMode = true
		}
	}

	effWd := effectiveWorkdirForWorkflowOpts(opts)
	projectRoot := effWd
	if ap, err := filepath.Abs(effWd); err == nil {
		projectRoot = ap
	}
	if wf != nil {
		if err := domain.ValidateLoadedWorkflow(wf); err != nil {
			return err
		}
		if err := infrastructure.CheckWorkflowPackageRequiresCapabilities(effWd, repoRoot, wfRoot, wfConfig); err != nil {
			return err
		}
	}

	envMap := domain.EnvironToMap(baseEnviron)
	if wf != nil {
		if err := buildWorkflowEnvInto(envMap, wf, wfRoot, repoRoot, opts); err != nil {
			return err
		}
		wn := strings.TrimSpace(wf.Name)
		if wn == "" {
			wn = strings.TrimSpace(opts.Workflow)
		}
		if wn != "" {
			domain.MergeIfUnset(envMap, map[string]string{"DOCKPIPE_WORKFLOW_NAME": wn})
		}
	}
	mergePromptSafetyCLIIntoEnv(envMap, opts)
	if err := mergeTerraformCLIIntoEnv(envMap, opts); err != nil {
		return fmt.Errorf("terraform flags: %w", err)
	}
	if stepsMode {
		MergeExtraEnvCLIIntoSteps(envMap, opts.ExtraEnvLines, opts.VarOverrides)
	}
	// CLI --workdir must live in envMap, not only on envSlice: strategy pre-scripts rebuild
	// envSlice from envMap (run.go) and would otherwise drop DOCKPIPE_WORKDIR from appendUniqueEnv.
	if opts.Workdir != "" {
		envMap["DOCKPIPE_WORKDIR"] = opts.Workdir
	}
	if strings.TrimSpace(envMap["DOCKPIPE_BIN"]) == "" {
		if dp, derr := resolveDockpipeBinForSDK(effWd); derr == nil && strings.TrimSpace(dp) != "" {
			envMap["DOCKPIPE_BIN"] = dp
		}
	}
	if dp := strings.TrimSpace(envMap["DOCKPIPE_BIN"]); dp != "" {
		if filepath.IsAbs(dp) {
			dpDir := filepath.Dir(dp)
			curPath := strings.TrimSpace(envMap["PATH"])
			prefix := dpDir + string(os.PathListSeparator)
			if curPath == "" {
				envMap["PATH"] = dpDir
			} else if !strings.HasPrefix(curPath, prefix) && curPath != dpDir {
				envMap["PATH"] = dpDir + string(os.PathListSeparator) + curPath
			}
		}
	}

	effStrat := EffectiveStrategyName(opts, wf)
	var stratBeforeAbs, stratAfterAbs []string
	strategyHandlesCommit := false
	if effStrat != "" {
		if err := ValidateStrategyAllowlist(wf, effStrat); err != nil {
			return err
		}
		sa, _, err := LoadStrategyAssignments(repoRoot, wfRoot, effStrat)
		if err != nil {
			return err
		}
		stratBeforeAbs = ResolveStrategyScriptPaths(sa.Before, wfRoot, repoRoot, projectRoot)
		stratAfterAbs = ResolveStrategyScriptPaths(sa.After, wfRoot, repoRoot, projectRoot)
		strategyHandlesCommit = StrategyAfterHandlesBundledCommit(stratAfterAbs, repoRoot)
		if wf != nil {
			if err := ValidateNoDuplicateClone(wf, wfRoot, repoRoot, projectRoot, effStrat == "worktree", stratBeforeAbs); err != nil {
				return err
			}
		}
	}
	if effStrat == "commit" && opts.RepoURL != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] warning: --repo set with strategy commit (no clone; strategy only commits after success)\n")
	}

	rtName := EffectiveRuntimeProfileName(opts, wf, stepsMode)
	rsName := EffectiveResolverProfileName(opts, wf, stepsMode)
	rtName, rsName, err = applyWorkflowCapabilityIsolation(effWd, repoRoot, wf, rtName, rsName)
	if err != nil {
		return err
	}
	rtName = infrastructure.NormalizeRuntimeProfileName(rtName)
	if rtName == "" && rsName == "" {
		if leg := EffectiveLegacyIsolateName(wf); leg != "" {
			rtName, rsName = leg, leg
		}
	}
	if err := applyDockpipeStateEnv(envMap, effWd, workflowStateScopeHint(opts, wfRoot, wf, rtName, rsName)); err != nil {
		return err
	}
	profileLabel := ProfileLabelForEnv(rtName, rsName)
	if rtName != "" {
		if err := ValidateRuntimeAllowlist(wf, rtName); err != nil {
			return err
		}
	}

	templateName := ""
	var preFromResolver, actFromResolver string
	hostIsolate := ""
	resolverWorkflow := ""
	var resolverEnvHint string
	if rtName != "" || rsName != "" {
		rm, err := infrastructure.LoadIsolationProfile(repoRoot, rtName, rsName)
		if err != nil {
			return fmt.Errorf("isolation profile: %w", err)
		}
		ra := domain.FromResolverMap(rm)
		resolverEnvHint = ra.EnvHint
		if rtName != "" && rsName != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Runtime: %s  Resolver: %s", rtName, rsName)
			if rk := strings.TrimSpace(ra.RuntimeKind); rk != "" {
				fmt.Fprintf(os.Stderr, "  runtime.type: %s", rk)
			}
			fmt.Fprintln(os.Stderr)
		} else if rk := strings.TrimSpace(ra.RuntimeKind); rk != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Profile: %s (runtime.type: %s)\n", profileLabel, rk)
		} else {
			fmt.Fprintf(os.Stderr, "[dockpipe] Profile: %s\n", profileLabel)
		}
		templateName = ra.Template
		hostIsolate = strings.TrimSpace(ra.HostIsolate)
		resolverWorkflow = strings.TrimSpace(ra.Workflow)
		if resolverWorkflow != "" && hostIsolate != "" {
			return fmt.Errorf("profile %q: set only one of DOCKPIPE_RUNTIME_WORKFLOW / DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RUNTIME_HOST_SCRIPT / DOCKPIPE_RESOLVER_HOST_ISOLATE", profileLabel)
		}
		preFromResolver = ra.PreScript
		actFromResolver = ra.Action
		if ra.Experimental {
			fmt.Fprintf(os.Stderr, "[dockpipe] Profile %q is experimental — see templates/core and docs.\n", profileLabel)
		}
		if opts.Workflow == "" && opts.WorkflowFile == "" {
			if len(opts.PreScripts) == 0 && preFromResolver != "" {
				opts.PreScripts = []string{filepath.Join(repoRoot, preFromResolver)}
			}
			if opts.Action == "" && actFromResolver != "" {
				opts.Action = filepath.Join(repoRoot, actFromResolver)
			}
		}
	}

	// Banner after resolver: workflow / host-isolate resolvers may omit argv after --.
	if !opts.Detach && (stepsMode || (opts.SeenDash && (len(rest) > 0 || hostIsolate != "" || resolverWorkflow != ""))) {
		infrastructure.PrintLaunchBanner(os.Stdout, os.Stderr)
	}
	if wf != nil && (opts.Workflow != "" || opts.WorkflowFile != "") {
		disp := strings.TrimSpace(wf.Name)
		if disp == "" {
			if opts.Workflow != "" {
				disp = opts.Workflow
			} else {
				disp = filepath.Base(opts.WorkflowFile)
			}
		}
		stepCount := 0
		if stepsMode {
			stepCount = len(wf.Steps)
		}
		fmt.Fprint(os.Stderr, workflowTaskLines(disp, wf.Description, stepCount))
	}

	locked := lockedKeys(opts.VarOverrides)

	if !stepsMode {
		if wf != nil {
			cliPre := opts.PreScripts
			opts.PreScripts = nil
			opts.PreScripts = append(opts.PreScripts, stratBeforeAbs...)
			if len(cliPre) == 0 && len(wf.Run) > 0 {
				for _, r := range wf.Run {
					opts.PreScripts = append(opts.PreScripts, resolveWorkflowAppFn(r, wfRoot, repoRoot, projectRoot))
				}
			}
			opts.PreScripts = append(opts.PreScripts, cliPre...)
			if opts.Action == "" {
				act := wf.Act
				if act == "" {
					act = wf.Action
				}
				if act != "" && !(strategyHandlesCommit && ActWouldBeBundledCommit(act, wfRoot, repoRoot, projectRoot)) {
					opts.Action = resolveWorkflowAppFn(act, wfRoot, repoRoot, projectRoot)
				}
			}
		} else {
			opts.PreScripts = append(stratBeforeAbs, opts.PreScripts...)
		}
	}

	// worktree strategy flow: clone-worktree.sh needs DOCKPIPE_REPO_URL. Without --repo we infer
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
		fmt.Fprintf(os.Stderr, "[dockpipe] Repo: using origin from %s\n", gitDir)
	}

	userIso := opts.Isolate
	userAct := opts.Action

	var image, buildDir, buildCtx string
	effectiveTemplate := templateName
	if effectiveTemplate == "" && (hostIsolate != "" || resolverWorkflow != "") {
		effectiveTemplate = profileLabel
	}
	commitOnHost := false
	actionForContainer := opts.Action

	if !stepsMode {
		if hostIsolate != "" || resolverWorkflow != "" {
			// Host script or embedded workflow replaces docker isolate (e.g. cursor-dev / vscode templates).
			image, buildDir, buildCtx = "", "", ""
		} else if opts.Isolate != "" {
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
		if hostIsolate == "" && resolverWorkflow == "" {
			if image == "" {
				image, buildDir = "dockpipe-base-dev", filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "images", "base-dev")
				buildCtx = repoRoot
			}
			image = maybeVersionTagAppFn(repoRoot, image)
		}

		cwd, _ := os.Getwd()
		if opts.Action != "" {
			ap, err := resolveActionPathFn(opts.Action, repoRoot, cwd, projectRoot)
			if err != nil {
				return err
			}
			if _, err := os.Stat(ap); err != nil {
				return fmt.Errorf("action script not found: %s", ap)
			}
			opts.Action = ap
			actionForContainer = ap
			if isBundledCommitAppFn(ap, repoRoot) {
				if strategyHandlesCommit {
					return fmt.Errorf("strategy %q already runs commit in its after hook; omit --act / workflow act that points at commit-worktree.sh", effStrat)
				}
				commitOnHost = true
				actionForContainer = ""
				applyBranchPrefix(envMap, profileLabel, effectiveTemplate)
				mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
			}
		}
	}
	compiledWorkflowManifest, err := loadCompiledRuntimeManifestForWorkflow(wfConfig, wfRoot)
	if err != nil {
		return err
	}
	image, buildDir, buildCtx = applyCompiledImageSelectionInputs(repoRoot, compiledWorkflowManifest, image, buildDir, buildCtx)

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
			prefix = profileLabel
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
		fmt.Fprintf(os.Stderr, "[dockpipe] Branch: %s (new)\n", wb)
	}

	if opts.RepoURL != "" && len(opts.PreScripts) == 0 && !stepsMode {
		if dataDir == "" {
			home, _ := os.UserHomeDir()
			if home == "" {
				home = "/tmp"
			}
			dataDir = filepath.Join(home, ".dockpipe")
			fmt.Fprintf(os.Stderr, "[dockpipe] Data dir for worktrees: %s (override with --data-dir)\n", dataDir)
			dataVol = ""
		}
		opts.PreScripts = []string{"scripts/clone-worktree.sh"}
	}

	// Drop inherited DOCKPIPE_WORKDIR before pre-scripts: a leftover export (or workflow .env)
	// would be passed into bash and survive env -0 if clone-worktree ever exited without exporting.
	if !stepsMode && preScriptsIncludeCloneWorktree(opts.PreScripts) {
		if opts.RepoURL != "" {
			setUserRepoRootForWorktree(envMap, opts, opts.RepoURL)
		}
		delete(envMap, "DOCKPIPE_WORKDIR")
		if opts.Workdir != "" {
			envMap["DOCKPIPE_WORKDIR"] = opts.Workdir
		}
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
	if rsName != "" {
		envSlice = appendUniqueEnv(envSlice, "RESOLVER="+rsName)
	}
	if rtName != "" {
		envSlice = appendUniqueEnv(envSlice, "RUNTIME="+rtName)
	}
	if rsName == "" && rtName == "" && profileLabel != "" {
		envSlice = appendUniqueEnv(envSlice, "RESOLVER="+profileLabel)
		envSlice = appendUniqueEnv(envSlice, "RUNTIME="+profileLabel)
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
			firstStepExtra = append(firstStepExtra, resolvePreScriptAppFn(p, repoRoot, projectRoot))
		}
		opts.PreScripts = nil
	} else {
		resolvedPre := make([]string, 0, len(opts.PreScripts))
		for _, p := range opts.PreScripts {
			resolvedPre = append(resolvedPre, resolvePreScriptAppFn(p, repoRoot, projectRoot))
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
		if !needsHostGit {
			for _, p := range stratBeforeAbs {
				if strings.HasSuffix(p, "clone-worktree.sh") {
					needsHostGit = true
					break
				}
			}
		}
	}
	if needsHostGit {
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("git not found in PATH — required on the host for clone/worktree/commit flows (by design; dockpipe invokes your normal git). Install git (e.g. Git for Windows on Windows, git from your OS package manager on Linux). HTTPS/SSH auth uses your existing git setup (Credential Manager, SSH keys, etc.)")
		}
	}

	if stepsMode {
		for _, p := range stratBeforeAbs {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("strategy before script not found: %s: %w", p, err)
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] Host setup (strategy)\n")
			stop := infrastructure.StartLineSpinner(os.Stderr, hostSpinnerLabel(p))
			em, err := sourceHostScriptAppFn(p, envSlice)
			stop()
			if err != nil {
				return err
			}
			for k, v := range em {
				envMap[k] = v
			}
			envSlice = domain.EnvMapToSlice(envMap)
		}
	}

	if !stepsMode {
		for _, p := range opts.PreScripts {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("pre-script not found: %s", p)
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] Host setup\n")
			stop := infrastructure.StartLineSpinner(os.Stderr, hostSpinnerLabel(p))
			em, err := sourceHostScriptAppFn(p, envSlice)
			stop()
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
	if len(rest) == 0 && !stepsMode && hostIsolate == "" && resolverWorkflow == "" {
		return fmt.Errorf("no command given after --")
	}

	if !stepsMode && resolverWorkflow != "" {
		if err := runEmbeddedResolverWorkflow(resolverWorkflow, repoRoot, envMap, opts, rest, locked, dataVol, dataDir, profileLabel, templateName, runStepsAppFn); err != nil {
			return err
		}
		workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
		if commitOnHost && strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Mount /work ← %s\n", envMap["DOCKPIPE_WORKDIR"])
		}
		if strategyHandlesCommit {
			mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
			applyBranchPrefix(envMap, profileLabel, effectiveTemplate)
			envSlice = domain.EnvMapToSlice(envMap)
		}
		if len(stratAfterAbs) > 0 {
			if err := RunStrategyAfterScripts(stratAfterAbs, repoRoot, envMap, envSlice, opts); err != nil {
				return err
			}
		}
		if commitOnHost && !strategyHandlesCommit {
			if err := infrastructure.CommitOnHost(workHost, envMap["DOCKPIPE_COMMIT_MESSAGE"], firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], opts.BundleOut), strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1"); err != nil {
				return err
			}
		}
		return nil
	}

	if !stepsMode && hostIsolate != "" {
		if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
			return err
		}
		scriptAbs := resolveWorkflowAppFn(hostIsolate, wfRoot, repoRoot, projectRoot)
		if _, err := os.Stat(scriptAbs); err != nil {
			return fmt.Errorf("host isolate script not found: %s: %w", scriptAbs, err)
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] Host isolate: %s\n", hostIsolate)
		workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
		if commitOnHost && strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Mount /work ← %s\n", envMap["DOCKPIPE_WORKDIR"])
		}
		if err := runHostScriptAppFn(scriptAbs, envSliceWithScriptContext(envSlice, scriptAbs)); err != nil {
			return err
		}
		if strategyHandlesCommit {
			mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
			applyBranchPrefix(envMap, profileLabel, effectiveTemplate)
			envSlice = domain.EnvMapToSlice(envMap)
		}
		if len(stratAfterAbs) > 0 {
			if err := RunStrategyAfterScripts(stratAfterAbs, repoRoot, envMap, envSlice, opts); err != nil {
				return err
			}
		}
		if commitOnHost && !strategyHandlesCommit {
			if err := infrastructure.CommitOnHost(workHost, envMap["DOCKPIPE_COMMIT_MESSAGE"], firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], opts.BundleOut), strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1"); err != nil {
				return err
			}
		}
		return nil
	}

	dockerEnvMap := domain.EnvSliceToMap(opts.ExtraEnvLines)
	workHostForEnv := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
	mergeWorktreeGitDockerEnv(dockerEnvMap, workHostForEnv)
	mergeEnvHintKeys(dockerEnvMap, envMap, resolverEnvHint)
	mergePolicyProxyEnvFromHost(dockerEnvMap, envMap)
	networkMode := infrastructure.DockerNetworkModeFromEnv(dockerEnvMap)
	if networkMode == "" {
		networkMode = strings.TrimSpace(envMap["DOCKPIPE_DOCKER_NETWORK"])
	}
	if networkMode == "" {
		networkMode = strings.TrimSpace(os.Getenv("DOCKPIPE_DOCKER_NETWORK"))
	}
	extraDocker := domain.EnvMapToSlice(dockerEnvMap)

	if stepsMode {
		if rm, err := applyCompiledRuntimePolicy(nil, wfConfig, wfRoot); err != nil {
			return err
		} else {
			for _, line := range compiledRuntimePolicyLogLines(rm) {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", line)
			}
		}
		if wf != nil && WorkflowNeedsDockerReachableResolved(wf, effWd, repoRoot) {
			if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
				return err
			}
		}
		if err := runStepsAppFn(runStepsOpts{
			wf:                    wf,
			wfRoot:                wfRoot,
			wfConfig:              wfConfig,
			repoRoot:              repoRoot,
			projectRoot:           projectRoot,
			cliArgs:               rest,
			envMap:                envMap,
			envSlice:              envSlice,
			locked:                locked,
			userIsolate:           userIso,
			userAct:               userAct,
			firstStepExtra:        firstStepExtra,
			opts:                  opts,
			dataVol:               dataVol,
			dataDir:               dataDir,
			resolver:              profileLabel,
			templateName:          templateName,
			strategyHandlesCommit: strategyHandlesCommit,
		}); err != nil {
			return err
		}
		if strategyHandlesCommit {
			mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
			applyBranchPrefix(envMap, profileLabel, templateName)
			envSlice = domain.EnvMapToSlice(envMap)
		}
		if len(stratAfterAbs) > 0 {
			return RunStrategyAfterScripts(stratAfterAbs, repoRoot, envMap, envSlice, opts)
		}
		return nil
	}

	if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
		return err
	}

	imageDecision := ""
	if buildDir != "" && buildCtx != "" {
		skipBuild, msg, err := maybeSkipDockerBuildForWorkflow(effWd, wfConfig, wfRoot, image, buildDir, buildCtx)
		if err != nil {
			return err
		}
		if skipBuild {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			imageDecision = msg
		} else {
			if strings.TrimSpace(msg) != "" {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] image: materializing image artifact (docker)…\n")
			if err := dockerBuildAppFn(image, buildDir, buildCtx); err != nil {
				return err
			}
			wfName := strings.TrimSpace(opts.Workflow)
			if wf != nil && wfName == "" {
				wfName = strings.TrimSpace(wf.Name)
			}
			policyFingerprint, _ := runtimePolicyFingerprintForRun(wfConfig, wfRoot)
			if artifact, err := buildImageArtifactManifest(repoRoot, wfName, "", templateName, image, buildDir, buildCtx, policyFingerprint, domain.ImageArtifactProvenance{Isolate: templateName, DockpipeVersion: authoredPackageVersion(repoRoot)}); err == nil {
				artifact.ArtifactState = "materialized"
				_ = persistCachedImageArtifactForIsolate(effWd, image, artifact)
				_ = persistImageArtifactIndexRecord(effWd, artifact)
			}
			imageDecision = "image: materialized image artifact for current run"
		}
	} else if compiledWorkflowManifest != nil {
		msg, err := ensureCompiledRegistryImageForWorkflow(compiledWorkflowManifest)
		if err != nil {
			return err
		}
		if msg != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			imageDecision = msg
		}
	}

	workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], opts.Workdir)
	if commitOnHost && strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] Mount /work ← %s\n", envMap["DOCKPIPE_WORKDIR"])
	}
	if strings.TrimSpace(envMap["DOCKPIPE_WORKDIR"]) != "" {
		if b := dockerEnvMap["DOCKPIPE_WORKTREE_BRANCH"]; b != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Container: DOCKPIPE_WORKTREE_BRANCH=%s\n", b)
		} else if dockerEnvMap["DOCKPIPE_WORKTREE_DETACHED"] == "1" {
			fmt.Fprintln(os.Stderr, "[dockpipe] Container: DOCKPIPE_WORKTREE_DETACHED=1")
		}
	}

	runOpts := infrastructure.RunOpts{
		Image:         image,
		WorkdirHost:   workHost,
		WorkPath:      opts.WorkPath,
		ActionPath:    actionForContainer,
		ExtraMounts:   opts.ExtraMounts,
		NetworkMode:   networkMode,
		ExtraEnv:      extraDocker,
		DataVolume:    dataVol,
		DataDir:       dataDir,
		Reinit:        opts.Reinit,
		Force:         opts.Force,
		Detach:        opts.Detach,
		CommitOnHost:  commitOnHost && !strategyHandlesCommit,
		CommitMessage: envMap["DOCKPIPE_COMMIT_MESSAGE"],
		BundleOut:     firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], opts.BundleOut),
		BundleAll:     strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1",
	}
	if rm, err := applyCompiledRuntimePolicy(&runOpts, wfConfig, wfRoot); err != nil {
		return err
	} else {
		for _, line := range compiledRuntimePolicyLogLines(rm) {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", line)
		}
		wfName := strings.TrimSpace(opts.Workflow)
		if wf != nil && wfName == "" {
			wfName = strings.TrimSpace(wf.Name)
		}
		if err := writeRunPolicyRecord(effWd, wfName, wfConfig, "", image, imageDecision, rm); err != nil {
			return err
		}
	}

	rc, err := runContainerAppFn(runOpts, rest)
	if err != nil {
		return err
	}
	if rc != 0 {
		osExitAppFn(rc)
	}
	if strategyHandlesCommit {
		mergeCommitEnvFromLines(envMap, opts.ExtraEnvLines)
		applyBranchPrefix(envMap, profileLabel, effectiveTemplate)
		envSlice = domain.EnvMapToSlice(envMap)
	}
	if len(stratAfterAbs) > 0 {
		if err := RunStrategyAfterScripts(stratAfterAbs, repoRoot, envMap, envSlice, opts); err != nil {
			return err
		}
	}
	return nil
}

func effectiveWorkdirForWorkflowOpts(opts *CliOpts) string {
	if opts != nil && strings.TrimSpace(opts.Workdir) != "" {
		return opts.Workdir
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func compileDepsWanted(opts *CliOpts) bool {
	if opts == nil {
		return false
	}
	if opts.NoCompileDeps {
		return false
	}
	v := strings.TrimSpace(os.Getenv("DOCKPIPE_COMPILE_DEPS"))
	if v != "" {
		vl := strings.ToLower(v)
		return vl != "0" && vl != "false" && vl != "no" && vl != "off"
	}
	// Env unset: default on for named --workflow (not --workflow-file); same as package compile for-workflow.
	if opts.Workflow != "" && opts.WorkflowFile == "" {
		return true
	}
	return opts.CompileDeps
}
