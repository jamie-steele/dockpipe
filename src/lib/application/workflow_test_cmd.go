package application

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
)

type workflowTestTarget struct {
	Name        string
	WorkflowDir string
	ConfigPath  string
	ScriptRel   string
	ScriptAbs   string
}

func cmdWorkflowTest(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(workflowTestUsageText)
		return nil
	}
	var (
		workdir string
		only    string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--only" && i+1 < len(args):
			only = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe workflow test --help)", args[i])
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
	return RunWorkflowTestsFromFlags(workdir, only)
}

func RunWorkflowTestsFromFlags(workdir, only string) error {
	root, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return err
	}
	targets, err := discoverWorkflowTestTargets(root, only)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if strings.TrimSpace(only) != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] workflow test: no workflow test matched %q\n", strings.TrimSpace(only))
		}
		return nil
	}
	dockpipeBin, _ := resolveDockpipeBinForSDK(root)
	for _, target := range targets {
		fmt.Fprintf(os.Stderr, "[dockpipe] workflow test: %s (%s)\n", target.Name, target.ScriptRel)
		if err := runWorkflowTestTarget(root, target, dockpipeBin); err != nil {
			return fmt.Errorf("workflow %q test: %w", target.Name, err)
		}
	}
	return nil
}

func discoverWorkflowTestTargets(workdir, only string) ([]workflowTestTarget, error) {
	only = strings.TrimSpace(only)
	if only != "" && strings.ContainsRune(only, os.PathListSeparator) {
		return nil, fmt.Errorf("workflow test --only accepts one workflow name")
	}
	cfg, err := loadDockpipeProjectConfig(workdir)
	if err != nil {
		return nil, err
	}
	roots := effectiveWorkflowCompileRoots(cfg, workdir)
	seen := make(map[string]struct{})
	var targets []workflowTestTarget
	for _, root := range roots {
		rootAbs, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(rootAbs); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
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
			if only != "" && wfName != only {
				return nil
			}
			if _, ok := seen[wfName]; ok {
				return nil
			}
			scriptRel, scriptAbs := resolveWorkflowTestScript(wfDir)
			if scriptAbs == "" {
				return nil
			}
			seen[wfName] = struct{}{}
			targets = append(targets, workflowTestTarget{
				Name:        wfName,
				WorkflowDir: wfDir,
				ConfigPath:  path,
				ScriptRel:   scriptRel,
				ScriptAbs:   scriptAbs,
			})
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets, nil
}

func resolveWorkflowTestScript(workflowDir string) (string, string) {
	for _, rel := range []string{"tests/run.sh", "tests/run.ps1", "tests/run.cmd", "tests/run.bat"} {
		abs := filepath.Join(workflowDir, filepath.FromSlash(rel))
		if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
			return rel, abs
		}
	}
	return "", ""
}

func runWorkflowTestTarget(workdir string, target workflowTestTarget, exe string) error {
	cmd, err := dockpipeScriptCommand(target.ScriptAbs)
	if err != nil {
		return err
	}
	cmd.Dir = target.WorkflowDir
	cmd.Env = append(os.Environ(),
		"DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_WORKFLOW_TEST=1",
		"DOCKPIPE_WORKFLOW_NAME="+target.Name,
		"DOCKPIPE_WORKFLOW_ROOT="+target.WorkflowDir,
		"DOCKPIPE_WORKFLOW_CONFIG="+target.ConfigPath,
		"DOCKPIPE_WORKFLOW_TEST_SCRIPT="+target.ScriptRel,
		"DOCKPIPE_BIN="+exe,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const workflowTestUsageText = `dockpipe workflow test

Run workflow-local tests from the current project/workdir. Workflow tests are
discovered from on-disk workflow directories that contain:
  tests/run.sh
  tests/run.ps1
  tests/run.cmd
  tests/run.bat

Usage:
  dockpipe workflow test [--workdir <path>] [--only <workflow>]

Options:
  --workdir <path>   Project/worktree root (default: current directory)
  --only <workflow>  Run one workflow by directory name
`
