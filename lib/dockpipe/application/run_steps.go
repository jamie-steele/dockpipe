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
	resolver       string
	templateName   string
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

func runBlockingStep(o *runStepsOpts, i, n int, dockerEnv map[string]string) error {
	step := o.wf.Steps[i]
	fmt.Fprintf(os.Stderr, "[dockpipe] --- Step %d/%d ---\n", i+1, n)

	mergeStepVars(o, step, dockerEnv)
	if err := runStepPreScripts(o, i, step); err != nil {
		return err
	}

	if step.SkipContainer {
		wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir, mustGetwd())
		applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked, nil, "")
		return nil
	}

	argv, runOpts, buildDir, buildCtx, err := buildStepContainer(o, i, n, step, o.envMap, dockerEnv)
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
		_, runOpts, buildDir, buildCtx, err := buildStepContainer(o, idx, n, step, localEnv, localDocker)
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

	argv, runOpts, _, _, err := buildStepContainer(o, idx, n, step, localEnv, localDocker)
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
func buildStepContainer(o *runStepsOpts, i, n int, step domain.Step, envMap, dockerEnv map[string]string) (
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
			commitOnHost = true
			actionPath = ""
			applyBranchPrefix(envMap, o.resolver, tmpl)
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
