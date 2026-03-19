package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
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

func runSteps(o runStepsOpts) error {
	dockerEnv := domain.EnvSliceToMap(o.opts.ExtraEnvLines)
	n := len(o.wf.Steps)
	for i, step := range o.wf.Steps {
		fmt.Fprintf(os.Stderr, "[dockpipe] --- Step %d/%d ---\n", i+1, n)
		for k, v := range step.Vars {
			if !o.locked[k] {
				o.envMap[k] = v
				dockerEnv[k] = v
				o.envSlice = domain.EnvMapToSlice(o.envMap)
			}
		}
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
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("pre-script not found: %s", p)
			}
			fmt.Fprintf(os.Stderr, "[dockpipe] Running pre-script: %s\n", p)
			em, err := infrastructure.SourceHostScript(p, o.envSlice)
			if err != nil {
				return err
			}
			for k, v := range em {
				o.envMap[k] = v
			}
			o.envSlice = domain.EnvMapToSlice(o.envMap)
		}

		if step.SkipContainer {
			wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir, mustGetwd())
			applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked)
			continue
		}

		argv, err := parseStepArgv(step.CmdLine())
		if err != nil {
			return err
		}
		if i == n-1 && len(argv) == 0 && len(o.cliArgs) > 0 {
			argv = append(argv, o.cliArgs...)
		}
		if len(argv) == 0 {
			return fmt.Errorf("step %d has no cmd/command and no command after --", i+1)
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

		var image, buildDir, buildCtx string
		var tmpl string
		if im, dir, ok := infrastructure.TemplateBuild(o.repoRoot, effIso); ok {
			tmpl = effIso
			image, buildDir, buildCtx = im, dir, o.repoRoot
		} else {
			image = effIso
		}
		if image == "" {
			image, buildDir = "dockpipe-base-dev", filepath.Join(o.repoRoot, "images/base-dev")
			buildCtx = o.repoRoot
		}
		image = infrastructure.MaybeVersionTag(o.repoRoot, image)

		actionPath := actAbs
		commitOnHost := false
		if actionPath != "" {
			if _, err := os.Stat(actionPath); err != nil {
				return fmt.Errorf("action script not found: %s", actionPath)
			}
			if infrastructure.IsBundledCommitWorktree(actionPath, o.repoRoot) {
				commitOnHost = true
				actionPath = ""
				applyBranchPrefix(o.envMap, o.resolver, tmpl)
			}
		}
		o.envSlice = domain.EnvMapToSlice(o.envMap)

		if buildDir != "" && buildCtx != "" {
			if err := infrastructure.DockerBuild(image, buildDir, buildCtx); err != nil {
				return err
			}
		}
		rc, err := infrastructure.RunContainer(infrastructure.RunOpts{
			Image:         image,
			WorkdirHost:   firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir),
			WorkPath:      o.opts.WorkPath,
			ActionPath:    actionPath,
			ExtraMounts:   o.opts.ExtraMounts,
			ExtraEnv:      domain.EnvMapToSlice(dockerEnv),
			DataVolume:    o.dataVol,
			DataDir:       o.dataDir,
			Reinit:        o.opts.Reinit,
			Force:         o.opts.Force,
			Detach:        o.opts.Detach,
			CommitOnHost:  commitOnHost,
			CommitMessage: o.envMap["DOCKPIPE_COMMIT_MESSAGE"],
			BundleOut:     firstNonEmpty(o.envMap["DOCKPIPE_BUNDLE_OUT"], o.opts.BundleOut),
		}, argv)
		if err != nil {
			return err
		}
		if rc != 0 {
			fmt.Fprintf(os.Stderr, "[dockpipe] Step %d failed with exit code %d\n", i+1, rc)
			os.Exit(rc)
		}
		wd := firstNonEmpty(o.envMap["DOCKPIPE_WORKDIR"], o.opts.Workdir)
		if wd == "" {
			wd, _ = os.Getwd()
		}
		applyOutputsFile(filepath.Join(wd, step.OutputsPath()), o.envMap, dockerEnv, o.locked)
	}
	return nil
}

func mustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

func parseStepArgv(cmd string) ([]string, error) {
	if strings.TrimSpace(cmd) == "" {
		return nil, nil
	}
	return shellwords.Parse(cmd)
}
