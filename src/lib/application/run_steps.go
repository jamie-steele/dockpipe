package application

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mattn/go-shellwords"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

var (
	dockerBuildFn       = infrastructure.DockerBuild
	dockerImageExistsFn = infrastructure.DockerImageExists
	dockerPullFn        = infrastructure.DockerPull
	composeLifecycleFn  = infrastructure.RunComposeLifecycle
	runContainerFn      = infrastructure.RunContainer
	sourceHostScriptFn  = infrastructure.SourceHostScript
	runHostScriptFn     = infrastructure.RunHostScript
	runHostCommandFn    = infrastructure.RunHostCommand
	osStatFn            = os.Stat
	getwdFn             = os.Getwd
)

type runStepsOpts struct {
	wf             *domain.Workflow
	wfRoot         string
	wfConfig       string
	repoRoot       string
	projectRoot    string // DockPipe project dir (--workdir / cwd); script resolution for scripts/…
	cliArgs        []string
	envMap         map[string]string
	envSlice       []string
	locked         map[string]bool
	userIsolate    string
	userAct        string
	firstStepExtra []string
	opts           *CliOpts
	dataVol        string
	dataDir        string
	resolver       string // display/branch label (ProfileLabelForEnv)
	templateName   string
	// strategyHandlesCommit: parent workflow uses a named strategy whose after hook runs bundled commit.
	// Suppress per-step bundled commit so CommitOnHost runs once after all steps (strategy after).
	strategyHandlesCommit bool
}

// runSteps executes workflow steps. Blocking steps run alone. Explicit async groups
// are flattened into one parallel batch:
//   - Inputs: each step sees env from the last blocking barrier only, plus its own vars
//     and pre-scripts (no sibling parallel mutations).
//   - Outputs: after all parallel steps finish, outputs files are merged in YAML order
//     into the shared env for the next blocking step.
func runSteps(o runStepsOpts) error {
	dockerEnv := domain.EnvSliceToMap(o.opts.ExtraEnvLines)
	n := len(o.wf.Steps)
	i := 0
	for i < n {
		step := o.wf.Steps[i]
		if step.IsBlocking() {
			if err := runBlockingStep(&o, i, n, dockerEnv); err != nil {
				return err
			}
			i++
			continue
		}
		j := i
		for j < n && !o.wf.Steps[j].IsBlocking() {
			j++
		}
		if err := runParallelBatch(&o, i, j, n, dockerEnv); err != nil {
			return err
		}
		i = j
	}
	return nil
}

// resolveStepIsolationNames returns per-step runtime and resolver profile names when set.
// Empty empty empty means keep parent workflow / CLI fallbacks.
func resolveStepIsolationNames(o *runStepsOpts, step domain.Step, stepIndex int) (rtName, rsName string, err error) {
	rtName = strings.TrimSpace(step.Runtime)
	rsName = strings.TrimSpace(step.Resolver)
	if rtName == "" && rsName == "" {
		return "", "", nil
	}
	return rtName, rsName, nil
}

func loadStepResolver(o *runStepsOpts, step domain.Step, stepIndex int) (*domain.ResolverAssignments, string, string, error) {
	rtName, rsName, err := resolveStepIsolationNames(o, step, stepIndex)
	if err != nil {
		return nil, "", "", err
	}
	if rtName == "" && rsName == "" {
		return nil, "", "", nil
	}
	m, err := infrastructure.LoadIsolationProfile(o.repoRoot, rtName, rsName)
	if err != nil {
		return nil, "", "", fmt.Errorf("step %s: isolation profile: %w", step.DisplayName(stepIndex), err)
	}
	ra := domain.FromResolverMap(m)
	label := ProfileLabelForEnv(rtName, rsName)
	if rk := strings.TrimSpace(ra.RuntimeKind); rk != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] Step %s: profile %q (runtime.type: %s)\n", step.DisplayName(stepIndex), label, rk)
	}
	if strings.TrimSpace(ra.Workflow) != "" && strings.TrimSpace(ra.HostIsolate) != "" {
		return nil, "", "", fmt.Errorf("step %s: profile %q: set only one of DOCKPIPE_RUNTIME_WORKFLOW / DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RUNTIME_HOST_SCRIPT / DOCKPIPE_RESOLVER_HOST_ISOLATE", step.DisplayName(stepIndex), label)
	}
	return &ra, rtName, rsName, nil
}

func stepUsesHostIsolate(ra *domain.ResolverAssignments) bool {
	return ra != nil && strings.TrimSpace(ra.HostIsolate) != ""
}

func stepUsesResolverWorkflow(ra *domain.ResolverAssignments) bool {
	return ra != nil && strings.TrimSpace(ra.Workflow) != ""
}

// stepUsesResolverDelegate is true when the resolver runs a bundled workflow or a host isolate script instead of docker.
func stepUsesResolverDelegate(ra *domain.ResolverAssignments) bool {
	return stepUsesHostIsolate(ra) || stepUsesResolverWorkflow(ra)
}

func branchResolverName(o *runStepsOpts, step domain.Step, stepIndex int) string {
	if s := ProfileLabelForEnv(strings.TrimSpace(step.Runtime), strings.TrimSpace(step.Resolver)); s != "" {
		return s
	}
	rt, rs, err := resolveStepIsolationNames(o, step, stepIndex)
	if err != nil {
		return o.resolver
	}
	if s := ProfileLabelForEnv(rt, rs); s != "" {
		return s
	}
	return o.resolver
}

func effActPathForStep(o *runStepsOpts, step domain.Step, ra *domain.ResolverAssignments) string {
	if act := step.ActPath(); act != "" {
		return act
	}
	if ra != nil && ra.Action != "" {
		return ra.Action
	}
	if o.userAct != "" {
		return o.userAct
	}
	return ""
}

// runStepResolverWorkflow runs templates/<DOCKPIPE_RESOLVER_WORKFLOW>/config.yml after pre-scripts.
func runStepResolverWorkflow(o *runStepsOpts, step domain.Step, dockerEnv map[string]string, ra *domain.ResolverAssignments, stepIndex int) error {
	// Use runSteps directly (not runStepsAppFn) so package init has no cycle: runStepsAppFn → runSteps → … → runStepsAppFn.
	if err := runEmbeddedResolverWorkflow(strings.TrimSpace(ra.Workflow), o.repoRoot, o.envMap, o.opts, o.cliArgs, o.locked, o.dataVol, o.dataDir, branchResolverName(o, step, stepIndex), o.templateName, runSteps); err != nil {
		return err
	}
	o.envSlice = domain.EnvMapToSlice(o.envMap)
	return finalizeResolverStepAfterHost(o, step, dockerEnv, ra, stepIndex)
}

// runStepHostIsolate runs DOCKPIPE_RESOLVER_HOST_ISOLATE after pre-scripts (same idea as single-command run with host isolate).
func runStepHostIsolate(o *runStepsOpts, step domain.Step, dockerEnv map[string]string, ra *domain.ResolverAssignments, stepIndex int) error {
	if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
		return err
	}
	rel := strings.TrimSpace(ra.HostIsolate)
	scriptAbs := infrastructure.ResolveWorkflowScript(rel, o.wfRoot, o.repoRoot, o.projectRoot)
	if _, err := osStatFn(scriptAbs); err != nil {
		return fmt.Errorf("host isolate script not found: %s: %w", scriptAbs, err)
	}
	if _, err := parseStepArgv(step.CmdLine()); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Host isolate: %s\n", rel)
	if strings.TrimSpace(o.envMap["DOCKPIPE_WORKDIR"]) != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] Mount /work ← %s\n", o.envMap["DOCKPIPE_WORKDIR"])
	}
	if err := runHostScriptFn(scriptAbs, envSliceWithScriptContext(o.envSlice, scriptAbs)); err != nil {
		return err
	}
	return finalizeResolverStepAfterHost(o, step, dockerEnv, ra, stepIndex)
}

func finalizeResolverStepAfterHost(o *runStepsOpts, step domain.Step, dockerEnv map[string]string, ra *domain.ResolverAssignments, stepIndex int) error {
	workHost := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	effAct := effActPathForStep(o, step, ra)
	if effAct != "" {
		actAbs := infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot, o.projectRoot)
		if _, err := osStatFn(actAbs); err != nil {
			return fmt.Errorf("action script not found: %s", actAbs)
		}
		if infrastructure.IsBundledCommitWorktree(actAbs, o.repoRoot) {
			if o.strategyHandlesCommit {
				// Strategy after hook will commit once after the workflow completes.
			} else {
				mergeCommitEnvFromLines(o.envMap, o.opts.ExtraEnvLines)
				tmpl := ra.Template
				if tmpl == "" {
					tmpl = ProfileLabelForEnv(strings.TrimSpace(step.Runtime), strings.TrimSpace(step.Resolver))
				}
				applyBranchPrefix(o.envMap, branchResolverName(o, step, stepIndex), tmpl)
				if err := infrastructure.CommitOnHost(workHost, o.envMap["DOCKPIPE_COMMIT_MESSAGE"], firstNonEmpty(o.envMap["DOCKPIPE_BUNDLE_OUT"], o.opts.BundleOut), strings.TrimSpace(o.envMap["DOCKPIPE_BUNDLE_ALL"]) == "1"); err != nil {
					return err
				}
			}
		}
	}
	wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	if wd == "" {
		wd, _ = getwdFn()
	}
	applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked, nil, "")
	return nil
}

// runStepPackageWorkflow runs a nested workflow selected by workflow: plus
// package: (namespace).
// See infrastructure.ResolvePackagedWorkflowConfigPath.
func runStepPackageWorkflow(o *runStepsOpts, i, n int, step domain.Step, dockerEnv map[string]string) error {
	wfName := strings.TrimSpace(step.WorkflowName)
	ns := strings.TrimSpace(step.Package)
	if wfName == "" {
		return fmt.Errorf("step %s: packaged workflow step requires workflow: <name>", step.DisplayName(i))
	}
	if ns == "" {
		return fmt.Errorf("step %s: packaged workflow step requires package: <namespace> (must match nested workflow namespace:)", step.DisplayName(i))
	}
	workdir := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	wfPath, err := infrastructure.ResolvePackagedWorkflowConfigPath(o.repoRoot, workdir, wfName, ns)
	if err != nil {
		return fmt.Errorf("step %s: %w", step.DisplayName(i), err)
	}
	subWf, err := loadWorkflowAppFn(wfPath)
	if err != nil {
		return fmt.Errorf("step %s: package workflow: %w", step.DisplayName(i), err)
	}
	if len(subWf.Steps) == 0 {
		return fmt.Errorf("step %s: packaged workflow %q has no steps", step.DisplayName(i), wfName)
	}
	if err := domain.ValidateLoadedWorkflow(subWf); err != nil {
		return fmt.Errorf("step %s: %w", step.DisplayName(i), err)
	}
	wfRoot := filepath.Dir(wfPath)
	if err := buildWorkflowEnvInto(o.envMap, subWf, wfRoot, o.repoRoot, o.opts); err != nil {
		return fmt.Errorf("step %s: %w", step.DisplayName(i), err)
	}
	o.envSlice = domain.EnvMapToSlice(o.envMap)
	if WorkflowNeedsDockerReachableResolved(subWf, workdir, o.repoRoot) {
		if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Package workflow %q (namespace %s)\n", wfName, ns)
	// Call runSteps directly (not runStepsAppFn) to avoid init cycle with run.go.
	if err := runSteps(runStepsOpts{
		wf:                    subWf,
		wfRoot:                wfRoot,
		wfConfig:              wfPath,
		repoRoot:              o.repoRoot,
		projectRoot:           o.projectRoot,
		cliArgs:               o.cliArgs,
		envMap:                o.envMap,
		envSlice:              o.envSlice,
		locked:                o.locked,
		userIsolate:           "",
		userAct:               "",
		firstStepExtra:        nil,
		opts:                  o.opts,
		dataVol:               o.dataVol,
		dataDir:               o.dataDir,
		resolver:              wfName,
		templateName:          "",
		strategyHandlesCommit: o.strategyHandlesCommit,
	}); err != nil {
		return err
	}
	wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	if wd == "" {
		wd, _ = getwdFn()
	}
	applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked, nil, "")
	return nil
}

func runBlockingStep(o *runStepsOpts, i, n int, dockerEnv map[string]string) error {
	step := o.wf.Steps[i]
	fmt.Fprintf(os.Stderr, "[dockpipe] --- Step %d/%d ---\n", i+1, n)

	mergeStepVars(o, step, dockerEnv)
	if err := runStepPreScripts(o, i, step); err != nil {
		return err
	}
	if err := runStepHostBuiltin(o, step); err != nil {
		return err
	}
	if step.UsesPackagedWorkflow() {
		return runStepPackageWorkflow(o, i, n, step, dockerEnv)
	}

	ra, effRt, effRs, err := loadStepResolver(o, step, i)
	if err != nil {
		return err
	}
	if step.IsHostStep() && stepUsesResolverDelegate(ra) {
		return fmt.Errorf("step %d: profile %q uses DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RESOLVER_HOST_ISOLATE — remove kind: host", i+1, ProfileLabelForEnv(effRt, effRs))
	}

	if step.IsHostStep() {
		if cmd := strings.TrimSpace(step.CmdLine()); cmd != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] Host command\n")
			if err := runHostCommandFn(cmd, o.envSlice); err != nil {
				return err
			}
		}
		wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir, mustGetwd())
		applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked, nil, "")
		return nil
	}

	if stepUsesResolverWorkflow(ra) {
		return runStepResolverWorkflow(o, step, dockerEnv, ra, i)
	}
	if stepUsesHostIsolate(ra) {
		return runStepHostIsolate(o, step, dockerEnv, ra, i)
	}

	argv, runOpts, buildDir, buildCtx, rm, err := buildStepContainer(o, i, n, step, o.envMap, dockerEnv, ra)
	if err != nil {
		return err
	}
	imageDecision := ""
	policyFingerprint := ""
	if buildDir != "" && buildCtx != "" {
		if rm != nil {
			policyFingerprint = strings.TrimSpace(rm.PolicyFingerprint)
		}
		skipBuild, msg, err := maybeSkipDockerBuildForStep(o.projectRoot, o.repoRoot, o.wfConfig, o.wfRoot, stepRunPolicyID(step, i), policyFingerprint, runOpts.Image, buildDir, buildCtx)
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
			if err := dockerBuildFn(runOpts.Image, buildDir, buildCtx); err != nil {
				return err
			}
			policyFingerprint = ""
			if rm != nil {
				policyFingerprint = strings.TrimSpace(rm.PolicyFingerprint)
			}
			if artifact, err := buildImageArtifactManifest(o.repoRoot, strings.TrimSpace(o.wf.Name), "", stepRunPolicyID(step, i), runOpts.Image, buildDir, buildCtx, policyFingerprint, runStepImageArtifactProvenance(o.repoRoot, step)); err == nil {
				artifact.ArtifactState = "materialized"
				_ = persistCachedImageArtifactForIsolate(o.projectRoot, runOpts.Image, artifact)
				_ = persistImageArtifactIndexRecord(o.projectRoot, artifact)
			}
			imageDecision = "image: materialized image artifact for current run"
		}
	} else if rm != nil {
		msg, err := ensureCompiledRegistryImageForStep(rm)
		if err != nil {
			return err
		}
		if msg != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			imageDecision = msg
		}
	}
	workdir := firstNonEmpty(o.projectRoot, o.opts.Workdir, o.envMap["DOCKPIPE_WORKDIR"], o.repoRoot, mustGetwd())
	if rm != nil && rm.PolicySources.StepOverride {
		for _, line := range compiledRuntimePolicyLogLines(rm) {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", line)
		}
	}
	if err := writeRunPolicyRecord(workdir, strings.TrimSpace(o.wf.Name), o.wfConfig, stepRunPolicyID(step, i), runOpts.Image, imageDecision, rm); err != nil {
		return err
	}
	rc, err := runContainerFn(runOpts, argv)
	if err != nil {
		return err
	}
	if rc != 0 {
		fmt.Fprintf(os.Stderr, "[dockpipe] Step %d failed with exit code %d\n", i+1, rc)
		os.Exit(rc)
	}
	wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	if wd == "" {
		wd, _ = getwdFn()
	}
	applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked, nil, "")
	return nil
}

func runParallelBatch(o *runStepsOpts, from, to, n int, dockerEnv map[string]string) error {
	if err := validateParallelOutputPaths(o.wf, from, to); err != nil {
		return err
	}
	if err := validateParallelNoResolverDelegate(o, from, to); err != nil {
		return err
	}
	if err := validateParallelNoHostCommit(o, from, to); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[dockpipe] --- Parallel batch steps %d–%d (non-blocking) ---\n", from+1, to)
	baseEnv := maps.Clone(o.envMap)
	baseDocker := maps.Clone(dockerEnv)

	if err := prefetchDockerBuildsForBatch(o, from, to, n, baseEnv, baseDocker); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var batchErr error
	for idx := from; idx < to; idx++ {
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runParallelStepWorker(o, idx, n, from, baseEnv, baseDocker); err != nil {
				mu.Lock()
				if batchErr == nil {
					batchErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if batchErr != nil {
		return batchErr
	}

	// Merge outputs in YAML / declaration order; later step wins on key collision (see [merge] logs).
	mergeLog := newParallelMergeState()
	for idx := from; idx < to; idx++ {
		step := o.wf.Steps[idx]
		wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
		if wd == "" {
			wd, _ = getwdFn()
		}
		src := step.DisplayName(idx)
		applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked, mergeLog, src)
	}
	o.envSlice = domain.EnvMapToSlice(o.envMap)
	return nil
}

func validateParallelOutputPaths(wf *domain.Workflow, from, to int) error {
	seen := make(map[string]struct{})
	for i := from; i < to; i++ {
		p := wf.Steps[i].OutputsPath()
		if _, ok := seen[p]; ok {
			return fmt.Errorf("parallel steps %d+: duplicate outputs path %q (set distinct outputs: per step)", i+1, p)
		}
		seen[p] = struct{}{}
	}
	return nil
}

func validateParallelNoResolverDelegate(o *runStepsOpts, from, to int) error {
	for i := from; i < to; i++ {
		step := o.wf.Steps[i]
		if step.UsesPackagedWorkflow() {
			return fmt.Errorf("parallel step %d: packaged workflow steps are not supported in async groups (use is_blocking: true)", i+1)
		}
		ra, effRt, effRs, err := loadStepResolver(o, step, i)
		if err != nil {
			return err
		}
		if stepUsesResolverDelegate(ra) {
			return fmt.Errorf("parallel step %d: profile %q uses DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RESOLVER_HOST_ISOLATE — not supported in async groups (use is_blocking: true)", i+1, ProfileLabelForEnv(effRt, effRs))
		}
	}
	return nil
}

func validateParallelNoHostCommit(o *runStepsOpts, from, to int) error {
	for i := from; i < to; i++ {
		step := o.wf.Steps[i]
		if step.IsHostStep() {
			continue
		}
		effAct := step.ActPath()
		if effAct == "" {
			effAct = o.userAct
		}
		if effAct == "" {
			continue
		}
		actAbs := infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot, o.projectRoot)
		if infrastructure.IsBundledCommitWorktree(actAbs, o.repoRoot) {
			return fmt.Errorf("step %d: host commit-worktree action cannot run inside an async group", i+1)
		}
	}
	return nil
}

func prefetchDockerBuildsForBatch(o *runStepsOpts, from, to, n int, baseEnv, baseDocker map[string]string) error {
	done := make(map[string]struct{})
	buildAnnounced := false
	for idx := from; idx < to; idx++ {
		step := o.wf.Steps[idx]
		if step.IsHostStep() {
			continue
		}
		if step.UsesPackagedWorkflow() {
			continue
		}
		localEnv := maps.Clone(baseEnv)
		localDocker := maps.Clone(baseDocker)
		for k, v := range step.Vars {
			if !o.locked[k] {
				localEnv[k] = v
				localDocker[k] = v
			}
		}
		ra, _, _, err := loadStepResolver(o, step, idx)
		if err != nil {
			return err
		}
		if stepUsesResolverDelegate(ra) {
			return fmt.Errorf("internal: resolver delegate in parallel batch should have been rejected")
		}
		_, runOpts, buildDir, buildCtx, rm, err := buildStepContainer(o, idx, n, step, localEnv, localDocker, ra)
		if err != nil {
			return err
		}
		if buildDir == "" || buildCtx == "" {
			if rm != nil {
				msg, err := ensureCompiledRegistryImageForStep(rm)
				if err != nil {
					return err
				}
				if msg != "" {
					fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
				}
			}
			continue
		}
		key := runOpts.Image + "\x00" + buildDir + "\x00" + buildCtx
		if _, ok := done[key]; ok {
			continue
		}
		done[key] = struct{}{}
		policyFingerprint := ""
		if rm != nil {
			policyFingerprint = strings.TrimSpace(rm.PolicyFingerprint)
		}
		skipBuild, msg, err := maybeSkipDockerBuildForStep(o.projectRoot, o.repoRoot, o.wfConfig, o.wfRoot, stepRunPolicyID(step, idx), policyFingerprint, runOpts.Image, buildDir, buildCtx)
		if err != nil {
			return err
		}
		if skipBuild {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
			continue
		}
		if strings.TrimSpace(msg) != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", msg)
		}
		if !buildAnnounced {
			fmt.Fprintf(os.Stderr, "[dockpipe] image: materializing image artifact (docker)…\n")
			buildAnnounced = true
		}
		if err := dockerBuildFn(runOpts.Image, buildDir, buildCtx); err != nil {
			return err
		}
		policyFingerprint = ""
		if rm != nil {
			policyFingerprint = strings.TrimSpace(rm.PolicyFingerprint)
		}
		if artifact, err := buildImageArtifactManifest(o.repoRoot, strings.TrimSpace(o.wf.Name), "", stepRunPolicyID(step, idx), runOpts.Image, buildDir, buildCtx, policyFingerprint, runStepImageArtifactProvenance(o.repoRoot, step)); err == nil {
			artifact.ArtifactState = "materialized"
			_ = persistCachedImageArtifactForIsolate(o.projectRoot, runOpts.Image, artifact)
			_ = persistImageArtifactIndexRecord(o.projectRoot, artifact)
		}
	}
	return nil
}

func runParallelStepWorker(o *runStepsOpts, idx, n, batchStart int, baseEnv, baseDocker map[string]string) error {
	step := o.wf.Steps[idx]
	localEnv := maps.Clone(baseEnv)
	localDocker := maps.Clone(baseDocker)

	for k, v := range step.Vars {
		if !o.locked[k] {
			localEnv[k] = v
			localDocker[k] = v
		}
	}
	envSlice := domain.EnvMapToSlice(localEnv)

	var pre []string
	for _, r := range step.RunPaths() {
		pre = append(pre, infrastructure.ResolveWorkflowScript(r, o.wfRoot, o.repoRoot, o.projectRoot))
	}
	if idx == batchStart && idx == 0 {
		for _, p := range o.firstStepExtra {
			pre = append(pre, p)
		}
	}
	for _, p := range pre {
		if p == "" {
			continue
		}
		if _, err := osStatFn(p); err != nil {
			return fmt.Errorf("pre-script not found: %s", p)
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] [parallel %d] Host setup\n", idx+1)
		if step.IsHostStep() {
			if err := runHostScriptFn(p, envSliceWithScriptContext(envSlice, p)); err != nil {
				return err
			}
			continue
		}
		em, err := sourceHostScriptFn(p, envSliceWithScriptContext(envSlice, p))
		if err != nil {
			return err
		}
		for k, v := range em {
			localEnv[k] = v
		}
		envSlice = domain.EnvMapToSlice(localEnv)
	}

	if step.UsesPackagedWorkflow() {
		return fmt.Errorf("parallel step %d: packaged workflow steps are not supported in async groups (use is_blocking: true)", idx+1)
	}

	if step.IsHostStep() {
		if cmd := strings.TrimSpace(step.CmdLine()); cmd != "" {
			if err := runHostCommandFn(cmd, envSlice); err != nil {
				return err
			}
		}
		return nil
	}

	ra, _, _, err := loadStepResolver(o, step, idx)
	if err != nil {
		return err
	}
	if stepUsesResolverDelegate(ra) {
		return fmt.Errorf("internal: resolver delegate in parallel batch should have been rejected")
	}
	argv, runOpts, _, _, rm, err := buildStepContainer(o, idx, n, step, localEnv, localDocker, ra)
	if err != nil {
		return err
	}
	workdir := firstNonEmpty(o.projectRoot, o.opts.Workdir, localEnv["DOCKPIPE_WORKDIR"], o.repoRoot, mustGetwd())
	if rm != nil && rm.PolicySources.StepOverride {
		for _, line := range compiledRuntimePolicyLogLines(rm) {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s\n", line)
		}
	}
	if err := writeRunPolicyRecord(workdir, strings.TrimSpace(o.wf.Name), o.wfConfig, stepRunPolicyID(step, idx), runOpts.Image, "", rm); err != nil {
		return err
	}
	rc, err := runContainerFn(runOpts, argv)
	if err != nil {
		return err
	}
	if rc != 0 {
		return fmt.Errorf("parallel step %d exited with code %d", idx+1, rc)
	}
	return nil
}

func mergeStepVars(o *runStepsOpts, step domain.Step, dockerEnv map[string]string) {
	for k, v := range step.Vars {
		if !o.locked[k] {
			o.envMap[k] = v
			dockerEnv[k] = v
		}
	}
	o.envSlice = domain.EnvMapToSlice(o.envMap)
}

func runStepHostBuiltin(o *runStepsOpts, step domain.Step) error {
	b := strings.TrimSpace(step.HostBuiltin)
	if b == "" {
		return nil
	}
	if !step.IsHostStep() {
		return fmt.Errorf("internal: host_builtin without kind: host")
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Host builtin: %s\n", b)
	switch b {
	case "package_build_store":
		wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
		if wd == "" {
			wd = mustGetwd()
		}
		wdAbs, err := filepath.Abs(filepath.Clean(wd))
		if err != nil {
			return err
		}
		return RunPackageBuildStoreFromEnv(wdAbs, o.envMap)
	case "compose_up", "compose_down", "compose_ps":
		return runWorkflowComposeHostBuiltin(o, b)
	default:
		return fmt.Errorf("unknown host_builtin %q", b)
	}
}

func runWorkflowComposeHostBuiltin(o *runStepsOpts, builtin string) error {
	if o == nil || o.wf == nil {
		return fmt.Errorf("compose builtin requires a workflow")
	}
	cfg := o.wf.Compose
	file := resolveWorkflowRelativePath(cfg.File, o.wfRoot)
	projectDir := resolveWorkflowRelativePath(cfg.ProjectDirectory, o.wfRoot)
	action := strings.TrimPrefix(strings.TrimSpace(builtin), "compose_")
	if action == "down" && !composeAutodownEnabled(cfg, o.envMap) {
		fmt.Fprintf(os.Stderr, "[dockpipe] Compose autodown disabled; leaving services running\n")
		return nil
	}
	if err := composeLifecycleFn(infrastructure.ComposeLifecycleOpts{
		Action:           action,
		File:             file,
		Project:          strings.TrimSpace(cfg.Project),
		ProjectDirectory: projectDir,
		Services:         append([]string(nil), cfg.Services...),
		Env:              domain.EnvMapToSlice(o.envMap),
	}); err != nil {
		return err
	}
	if action != "down" {
		applyComposeExports(o.envMap, cfg.Exports)
		o.envSlice = domain.EnvMapToSlice(o.envMap)
	}
	return nil
}

func composeAutodownEnabled(cfg domain.WorkflowComposeConfig, env map[string]string) bool {
	name := strings.TrimSpace(cfg.AutodownEnv)
	if name == "" {
		return true
	}
	value := strings.TrimSpace(env[name])
	if value == "" {
		value = strings.TrimSpace(os.Getenv(name))
	}
	switch strings.ToLower(value) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func applyComposeExports(env map[string]string, exports map[string]string) {
	if env == nil || len(exports) == 0 {
		return
	}
	for key, value := range exports {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		env[key] = strings.TrimSpace(value)
	}
}

func resolveWorkflowRelativePath(path, wfRoot string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(wfRoot, path))
}

func validateParallelNoHostBuiltin(o *runStepsOpts, from, to int) error {
	for i := from; i < to; i++ {
		if strings.TrimSpace(o.wf.Steps[i].HostBuiltin) != "" {
			return fmt.Errorf("parallel step %d: host_builtin is not supported in non-blocking batches", i+1)
		}
	}
	return nil
}

func runStepPreScripts(o *runStepsOpts, i int, step domain.Step) error {
	var pre []string
	for _, r := range step.RunPaths() {
		pre = append(pre, infrastructure.ResolveWorkflowScript(r, o.wfRoot, o.repoRoot, o.projectRoot))
	}
	if i == 0 {
		for _, p := range o.firstStepExtra {
			pre = append(pre, p)
		}
	}
	for _, p := range pre {
		if p == "" {
			continue
		}
		if _, err := osStatFn(p); err != nil {
			return fmt.Errorf("pre-script not found: %s", p)
		}
		if step.IsHostStep() {
			// host-step run: must exec with inherited stdio — SourceHostScript sources and
			// captures CombinedOutput(), so users would see nothing (e.g. cursor-dev step 2, vscode).
			fmt.Fprintf(os.Stderr, "[dockpipe] Host setup\n")
			if err := runHostScriptFn(p, envSliceWithScriptContext(o.envSlice, p)); err != nil {
				return err
			}
			continue
		}
		stop := infrastructure.StartLineSpinner(os.Stderr, hostSpinnerLabel(p))
		em, err := sourceHostScriptFn(p, envSliceWithScriptContext(o.envSlice, p))
		stop()
		if err != nil {
			return err
		}
		for k, v := range em {
			o.envMap[k] = v
		}
		o.envSlice = domain.EnvMapToSlice(o.envMap)
	}
	return nil
}

// buildStepContainer returns argv, docker run options, and Dockerfile build dir/context (if any).
// ra is optional assignments from a shared core runtime profile; must not describe host isolate (handled before this).
func buildStepContainer(o *runStepsOpts, i, n int, step domain.Step, envMap, dockerEnv map[string]string, ra *domain.ResolverAssignments) (
	argv []string, runOpts infrastructure.RunOpts, buildDir, buildCtx string, rm *domain.CompiledRuntimeManifest, err error,
) {
	argv, err = parseStepArgv(step.CmdLine())
	if err != nil {
		return nil, runOpts, "", "", nil, err
	}
	if i == n-1 && len(argv) == 0 && len(o.cliArgs) > 0 {
		argv = append(argv, o.cliArgs...)
	}
	if len(argv) == 0 {
		return nil, runOpts, "", "", nil, fmt.Errorf("step %d has no cmd/command and no command after --", i+1)
	}

	effIso := step.Isolate
	if effIso == "" && ra != nil && ra.Template != "" {
		effIso = ra.Template
	}
	if effIso == "" {
		effIso = o.userIsolate
	}
	if effIso == "" {
		effIso = o.wf.Isolate
	}
	if effIso == "" {
		effIso = o.resolver
	}

	effAct := step.ActPath()
	if effAct == "" && ra != nil && ra.Action != "" {
		effAct = ra.Action
	}
	if effAct == "" {
		effAct = o.userAct
	}
	var actAbs string
	if effAct != "" {
		actAbs = infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot, o.projectRoot)
	}

	var image, dockerfileDir, contextDir string
	var tmpl string
	if im, dir, ok := infrastructure.TemplateBuild(o.repoRoot, effIso); ok {
		tmpl = effIso
		image, dockerfileDir, contextDir = im, dir, o.repoRoot
	} else {
		image = effIso
	}
	if image == "" {
		image, dockerfileDir = "dockpipe-base-dev", filepath.Join(infrastructure.CoreDir(o.repoRoot), "assets", "images", "base-dev")
		contextDir = o.repoRoot
	}
	image = infrastructure.MaybeVersionTag(o.repoRoot, image)

	actionPath := actAbs
	commitOnHost := false
	if actionPath != "" {
		if _, err := osStatFn(actionPath); err != nil {
			return nil, runOpts, "", "", nil, fmt.Errorf("action script not found: %s", actionPath)
		}
		if infrastructure.IsBundledCommitWorktree(actionPath, o.repoRoot) {
			if !o.strategyHandlesCommit {
				commitOnHost = true
				actionPath = ""
				applyBranchPrefix(envMap, branchResolverName(o, step, i), tmpl)
			} else {
				actionPath = ""
			}
		}
	}

	workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	dockerForRun := maps.Clone(dockerEnv)
	mergeResolverAuthEnvFromHost(dockerForRun, envMap, ra)
	mergePolicyProxyEnvFromHost(dockerForRun, envMap)
	mergeWorktreeGitDockerEnv(dockerForRun, workHost)
	networkMode := infrastructure.DockerNetworkModeFromEnv(dockerForRun)
	if networkMode == "" {
		networkMode = strings.TrimSpace(envMap["DOCKPIPE_DOCKER_NETWORK"])
	}
	if networkMode == "" {
		networkMode = strings.TrimSpace(os.Getenv("DOCKPIPE_DOCKER_NETWORK"))
	}

	runOpts = infrastructure.RunOpts{
		Image:         image,
		WorkdirHost:   workHost,
		WorkPath:      o.opts.WorkPath,
		ActionPath:    actionPath,
		ExtraMounts:   o.opts.ExtraMounts,
		NetworkMode:   networkMode,
		ExtraEnv:      domain.EnvMapToSlice(dockerForRun),
		DataVolume:    o.dataVol,
		DataDir:       o.dataDir,
		Reinit:        o.opts.Reinit,
		Force:         o.opts.Force,
		Detach:        o.opts.Detach,
		CommitOnHost:  commitOnHost,
		CommitMessage: envMap["DOCKPIPE_COMMIT_MESSAGE"],
		BundleOut:     firstNonEmpty(envMap["DOCKPIPE_BUNDLE_OUT"], o.opts.BundleOut),
		BundleAll:     strings.TrimSpace(envMap["DOCKPIPE_BUNDLE_ALL"]) == "1",
	}
	rm, err = applyCompiledRuntimePolicyForStep(&runOpts, o.wf, o.wfConfig, o.wfRoot, step, stepRunPolicyID(step, i))
	if err != nil {
		return nil, runOpts, "", "", nil, err
	}
	image, dockerfileDir, contextDir = applyCompiledImageSelectionInputs(o.repoRoot, rm, image, dockerfileDir, contextDir)
	runOpts.Image = image
	return argv, runOpts, dockerfileDir, contextDir, rm, nil
}

func mustGetwd() string {
	wd, _ := getwdFn()
	return wd
}

func parseStepArgv(cmd string) ([]string, error) {
	if strings.TrimSpace(cmd) == "" {
		return nil, nil
	}
	return shellwords.Parse(cmd)
}

func runStepImageArtifactProvenance(repoRoot string, step domain.Step) domain.ImageArtifactProvenance {
	p := domain.ImageArtifactProvenance{DockpipeVersion: authoredPackageVersion(repoRoot)}
	switch {
	case strings.TrimSpace(step.Isolate) != "":
		p.Isolate = strings.TrimSpace(step.Isolate)
	case strings.TrimSpace(step.Resolver) != "":
		p.Resolver = strings.TrimSpace(step.Resolver)
	case strings.TrimSpace(step.Runtime) != "":
		p.Runtime = strings.TrimSpace(step.Runtime)
	}
	return p
}

func stepRunPolicyID(step domain.Step, idx int) string {
	if s := strings.TrimSpace(step.ID); s != "" {
		return s
	}
	return fmt.Sprintf("step-%d", idx+1)
}
