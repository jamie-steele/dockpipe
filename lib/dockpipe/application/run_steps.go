package application

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mattn/go-shellwords"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

var (
	dockerBuildFn      = infrastructure.DockerBuild
	runContainerFn     = infrastructure.RunContainer
	sourceHostScriptFn = infrastructure.SourceHostScript
	runHostScriptFn    = infrastructure.RunHostScript
	osStatFn           = os.Stat
	getwdFn            = os.Getwd
)

type runStepsOpts struct {
	wf             *domain.Workflow
	wfRoot         string
	repoRoot       string
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

// runSteps executes workflow steps. Blocking steps run alone. Consecutive steps with
// is_blocking: false form one parallel batch:
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

func loadStepResolver(o *runStepsOpts, step domain.Step, stepIndex int) (*domain.ResolverAssignments, error) {
	rtName := strings.TrimSpace(step.Runtime)
	rsName := strings.TrimSpace(step.Resolver)
	if rtName == "" && rsName == "" {
		return nil, nil
	}
	m, err := infrastructure.LoadIsolationProfile(o.repoRoot, rtName, rsName)
	if err != nil {
		return nil, fmt.Errorf("step %s: isolation profile: %w", step.DisplayName(stepIndex), err)
	}
	ra := domain.FromResolverMap(m)
	label := ProfileLabelForEnv(rtName, rsName)
	if rk := strings.TrimSpace(ra.RuntimeKind); rk != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] Step %s: profile %q (runtime.type: %s)\n", step.DisplayName(stepIndex), label, rk)
	}
	if strings.TrimSpace(ra.Workflow) != "" && strings.TrimSpace(ra.HostIsolate) != "" {
		return nil, fmt.Errorf("step %s: profile %q: set only one of DOCKPIPE_RUNTIME_WORKFLOW / DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RUNTIME_HOST_SCRIPT / DOCKPIPE_RESOLVER_HOST_ISOLATE", step.DisplayName(stepIndex), label)
	}
	return &ra, nil
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

func branchResolverName(o *runStepsOpts, step domain.Step) string {
	if s := ProfileLabelForEnv(strings.TrimSpace(step.Runtime), strings.TrimSpace(step.Resolver)); s != "" {
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
	if o.wf.Act != "" {
		return o.wf.Act
	}
	return o.wf.Action
}

// runStepResolverWorkflow runs templates/<DOCKPIPE_RESOLVER_WORKFLOW>/config.yml after pre-scripts.
func runStepResolverWorkflow(o *runStepsOpts, step domain.Step, dockerEnv map[string]string, ra *domain.ResolverAssignments) error {
	// Use runSteps directly (not runStepsAppFn) so package init has no cycle: runStepsAppFn → runSteps → … → runStepsAppFn.
	if err := runEmbeddedResolverWorkflow(strings.TrimSpace(ra.Workflow), o.repoRoot, o.envMap, o.opts, o.cliArgs, o.locked, o.dataVol, o.dataDir, branchResolverName(o, step), o.templateName, runSteps); err != nil {
		return err
	}
	o.envSlice = domain.EnvMapToSlice(o.envMap)
	return finalizeResolverStepAfterHost(o, step, dockerEnv, ra)
}

// runStepHostIsolate runs DOCKPIPE_RESOLVER_HOST_ISOLATE after pre-scripts (same idea as single-command run with host isolate).
func runStepHostIsolate(o *runStepsOpts, step domain.Step, dockerEnv map[string]string, ra *domain.ResolverAssignments) error {
	if err := infrastructure.EnsureDockerReachable(os.Stderr); err != nil {
		return err
	}
	rel := strings.TrimSpace(ra.HostIsolate)
	scriptAbs := infrastructure.ResolveWorkflowScript(rel, o.wfRoot, o.repoRoot)
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
	if err := runHostScriptFn(scriptAbs, o.envSlice); err != nil {
		return err
	}
	return finalizeResolverStepAfterHost(o, step, dockerEnv, ra)
}

func finalizeResolverStepAfterHost(o *runStepsOpts, step domain.Step, dockerEnv map[string]string, ra *domain.ResolverAssignments) error {
	workHost := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	effAct := effActPathForStep(o, step, ra)
	if effAct != "" {
		actAbs := infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot)
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
				applyBranchPrefix(o.envMap, branchResolverName(o, step), tmpl)
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

func runBlockingStep(o *runStepsOpts, i, n int, dockerEnv map[string]string) error {
	step := o.wf.Steps[i]
	fmt.Fprintf(os.Stderr, "[dockpipe] --- Step %d/%d ---\n", i+1, n)

	mergeStepVars(o, step, dockerEnv)
	if err := runStepPreScripts(o, i, step); err != nil {
		return err
	}

	ra, err := loadStepResolver(o, step, i)
	if err != nil {
		return err
	}
	if step.SkipContainer && stepUsesResolverDelegate(ra) {
		return fmt.Errorf("step %d: profile %q uses DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RESOLVER_HOST_ISOLATE — remove skip_container: true", i+1, ProfileLabelForEnv(strings.TrimSpace(step.Runtime), strings.TrimSpace(step.Resolver)))
	}

	if step.SkipContainer {
		wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir, mustGetwd())
		applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked, nil, "")
		return nil
	}

	if stepUsesResolverWorkflow(ra) {
		return runStepResolverWorkflow(o, step, dockerEnv, ra)
	}
	if stepUsesHostIsolate(ra) {
		return runStepHostIsolate(o, step, dockerEnv, ra)
	}

	argv, runOpts, buildDir, buildCtx, err := buildStepContainer(o, i, n, step, o.envMap, dockerEnv, ra)
	if err != nil {
		return err
	}
	if buildDir != "" && buildCtx != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] Building image (docker)…\n")
		if err := dockerBuildFn(runOpts.Image, buildDir, buildCtx); err != nil {
			return err
		}
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
		ra, err := loadStepResolver(o, step, i)
		if err != nil {
			return err
		}
		if stepUsesResolverDelegate(ra) {
			return fmt.Errorf("parallel step %d: profile %q uses DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RESOLVER_HOST_ISOLATE — not supported in async groups (use is_blocking: true)", i+1, ProfileLabelForEnv(strings.TrimSpace(step.Runtime), strings.TrimSpace(step.Resolver)))
		}
	}
	return nil
}

func validateParallelNoHostCommit(o *runStepsOpts, from, to int) error {
	for i := from; i < to; i++ {
		step := o.wf.Steps[i]
		if step.SkipContainer {
			continue
		}
		effAct := step.ActPath()
		if effAct == "" {
			effAct = o.userAct
		}
		if effAct == "" {
			effAct = o.wf.Act
			if effAct == "" {
				effAct = o.wf.Action
			}
		}
		if effAct == "" {
			continue
		}
		actAbs := infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot)
		if infrastructure.IsBundledCommitWorktree(actAbs, o.repoRoot) {
			return fmt.Errorf("step %d: host commit-worktree action cannot run inside a parallel (is_blocking: false) batch", i+1)
		}
	}
	return nil
}

func prefetchDockerBuildsForBatch(o *runStepsOpts, from, to, n int, baseEnv, baseDocker map[string]string) error {
	done := make(map[string]struct{})
	buildAnnounced := false
	for idx := from; idx < to; idx++ {
		step := o.wf.Steps[idx]
		if step.SkipContainer {
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
		ra, err := loadStepResolver(o, step, idx)
		if err != nil {
			return err
		}
		if stepUsesResolverDelegate(ra) {
			return fmt.Errorf("internal: resolver delegate in parallel batch should have been rejected")
		}
		_, runOpts, buildDir, buildCtx, err := buildStepContainer(o, idx, n, step, localEnv, localDocker, ra)
		if err != nil {
			return err
		}
		if buildDir == "" || buildCtx == "" {
			continue
		}
		key := runOpts.Image + "\x00" + buildDir + "\x00" + buildCtx
		if _, ok := done[key]; ok {
			continue
		}
		done[key] = struct{}{}
		if !buildAnnounced {
			fmt.Fprintf(os.Stderr, "[dockpipe] Building image (docker)…\n")
			buildAnnounced = true
		}
		if err := dockerBuildFn(runOpts.Image, buildDir, buildCtx); err != nil {
			return err
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
		pre = append(pre, infrastructure.ResolveWorkflowScript(r, o.wfRoot, o.repoRoot))
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
		if step.SkipContainer {
			if err := runHostScriptFn(p, envSlice); err != nil {
				return err
			}
			continue
		}
		em, err := sourceHostScriptFn(p, envSlice)
		if err != nil {
			return err
		}
		for k, v := range em {
			localEnv[k] = v
		}
		envSlice = domain.EnvMapToSlice(localEnv)
	}

	if step.SkipContainer {
		return nil
	}

	ra, err := loadStepResolver(o, step, idx)
	if err != nil {
		return err
	}
	if stepUsesResolverDelegate(ra) {
		return fmt.Errorf("internal: resolver delegate in parallel batch should have been rejected")
	}
	argv, runOpts, _, _, err := buildStepContainer(o, idx, n, step, localEnv, localDocker, ra)
	if err != nil {
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

func runStepPreScripts(o *runStepsOpts, i int, step domain.Step) error {
	var pre []string
	for _, r := range step.RunPaths() {
		pre = append(pre, infrastructure.ResolveWorkflowScript(r, o.wfRoot, o.repoRoot))
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
		if step.SkipContainer {
			// skip_container run: must exec with inherited stdio — SourceHostScript sources and
			// captures CombinedOutput(), so users would see nothing (e.g. cursor-dev step 2, vscode).
			fmt.Fprintf(os.Stderr, "[dockpipe] Host setup\n")
			if err := runHostScriptFn(p, o.envSlice); err != nil {
				return err
			}
			continue
		}
		stop := infrastructure.StartLineSpinner(os.Stderr, hostSpinnerLabel(p))
		em, err := sourceHostScriptFn(p, o.envSlice)
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
	argv []string, runOpts infrastructure.RunOpts, buildDir, buildCtx string, err error,
) {
	argv, err = parseStepArgv(step.CmdLine())
	if err != nil {
		return nil, runOpts, "", "", err
	}
	if i == n-1 && len(argv) == 0 && len(o.cliArgs) > 0 {
		argv = append(argv, o.cliArgs...)
	}
	if len(argv) == 0 {
		return nil, runOpts, "", "", fmt.Errorf("step %d has no cmd/command and no command after --", i+1)
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
	if effAct == "" {
		effAct = o.wf.Act
		if effAct == "" {
			effAct = o.wf.Action
		}
	}
	var actAbs string
	if effAct != "" {
		actAbs = infrastructure.ResolveWorkflowScript(effAct, o.wfRoot, o.repoRoot)
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
		image, dockerfileDir = "dockpipe-base-dev", filepath.Join(o.repoRoot, "images/base-dev")
		contextDir = o.repoRoot
	}
	image = infrastructure.MaybeVersionTag(o.repoRoot, image)

	actionPath := actAbs
	commitOnHost := false
	if actionPath != "" {
		if _, err := osStatFn(actionPath); err != nil {
			return nil, runOpts, "", "", fmt.Errorf("action script not found: %s", actionPath)
		}
		if infrastructure.IsBundledCommitWorktree(actionPath, o.repoRoot) {
			if !o.strategyHandlesCommit {
				commitOnHost = true
				actionPath = ""
				applyBranchPrefix(envMap, branchResolverName(o, step), tmpl)
			} else {
				actionPath = ""
			}
		}
	}

	workHost := firstNonEmpty(envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
	dockerForRun := maps.Clone(dockerEnv)
	mergeWorktreeGitDockerEnv(dockerForRun, workHost)

	runOpts = infrastructure.RunOpts{
		Image:         image,
		WorkdirHost:   workHost,
		WorkPath:      o.opts.WorkPath,
		ActionPath:    actionPath,
		ExtraMounts:   o.opts.ExtraMounts,
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
	return argv, runOpts, dockerfileDir, contextDir, nil
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
