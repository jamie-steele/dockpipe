package application

import (
	"fmt"
	"path/filepath"
	"strings"
)

// isPosixAbsPath reports paths like /abs/foo (single leading slash, not UNC).
// filepath.IsAbs is false for these on Windows; they are still valid for --build (container) paths.
func isPosixAbsPath(p string) bool {
	p = strings.TrimSpace(p)
	if len(p) < 2 || p[0] != '/' {
		return false
	}
	return p[1] != '/'
}

// CliOpts holds parsed CLI flags (before --).
type CliOpts struct {
	Help                 bool
	Detach               bool
	ApproveSystemChanges bool
	Force                bool
	Reinit               bool
	DataVolume           string
	DataDir              string
	NoData               bool
	PreScripts           []string
	Isolate              string
	Action               string
	Workflow             string
	WorkflowFile         string
	WorkflowsDir         string
	Workdir              string
	RepoURL              string
	RepoBranch           string
	WorkPath             string
	WorkBranch           string
	BundleOut            string
	Runtime              string
	Resolver             string
	Strategy             string
	ExtraMounts          []string
	ExtraEnvLines        []string
	EnvFiles             []string
	VarOverrides         []string
	NoOpInject           bool // skip vault env resolution via op inject (when dockpipe.config.json sets op_inject_template)
	BuildPath            string
	// CompileDeps is legacy: transitive compile for --workflow is on by default when env is unset.
	CompileDeps bool
	// NoCompileDeps skips the default pre-run package compile for-workflow (see compileDepsWanted).
	NoCompileDeps bool
	// TfCommands sets DOCKPIPE_TF_COMMANDS for workflows/scripts that use terraform-pipeline.sh (e.g. plan, apply, init,plan).
	TfCommands string
	TfDryRun   bool // sets DOCKPIPE_TF_DRY_RUN=1
	// TfNoAutoApprove sets DOCKPIPE_TF_APPLY_AUTO_APPROVE=0 when apply runs.
	TfNoAutoApprove bool
	SeenDash        bool
}

// ParseFlags parses argv until "--" or end; returns remaining args after "--".
func ParseFlags(repoRoot string, argv []string) ([]string, *CliOpts, error) {
	o := &CliOpts{}
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch a {
		case "--help", "-h":
			o.Help = true
			i++
		case "-d", "--detach":
			o.Detach = true
			i++
		case "-y", "--yes", "--approve-system-changes":
			o.ApproveSystemChanges = true
			i++
		case "-f", "--force":
			o.Force = true
			i++
		case "--reinit":
			o.Reinit = true
			i++
		case "--no-data":
			o.NoData = true
			i++
		case "--":
			o.SeenDash = true
			return argv[i+1:], o, nil
		case "--data-vol", "--data-volume":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("%s requires an argument", a)
			}
			o.DataVolume = argv[i+1]
			i += 2
		case "--data-dir":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--data-dir requires an argument")
			}
			o.DataDir = argv[i+1]
			i += 2
		case "--run", "--pre-script":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("%s requires an argument", a)
			}
			o.PreScripts = append(o.PreScripts, argv[i+1])
			i += 2
		case "--isolate", "--template", "--image":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("%s requires an argument", a)
			}
			o.Isolate = argv[i+1]
			i += 2
		case "--act", "--action":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("%s requires an argument", a)
			}
			o.Action = argv[i+1]
			i += 2
		case "--workflow":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--workflow requires an argument")
			}
			o.Workflow = argv[i+1]
			i += 2
		case "--workflow-file":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--workflow-file requires a path to a YAML file (e.g. workflows/test/config.yml)")
			}
			o.WorkflowFile = argv[i+1]
			i += 2
		case "--workflows-dir":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--workflows-dir requires a path (repo-relative or absolute)")
			}
			o.WorkflowsDir = argv[i+1]
			i += 2
		case "--workdir":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--workdir requires an argument")
			}
			o.Workdir = argv[i+1]
			i += 2
		case "--repo":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--repo requires an argument")
			}
			o.RepoURL = argv[i+1]
			i += 2
		case "--branch":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--branch requires an argument")
			}
			o.RepoBranch = argv[i+1]
			i += 2
		case "--work-path":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--work-path requires an argument")
			}
			o.WorkPath = argv[i+1]
			i += 2
		case "--work-branch":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--work-branch requires an argument")
			}
			o.WorkBranch = argv[i+1]
			i += 2
		case "--bundle-out":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--bundle-out requires an argument")
			}
			o.BundleOut = argv[i+1]
			i += 2
		case "--runtime":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--runtime requires an argument")
			}
			o.Runtime = argv[i+1]
			i += 2
		case "--resolver":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--resolver requires an argument")
			}
			o.Resolver = argv[i+1]
			i += 2
		case "--strategy":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--strategy requires an argument")
			}
			o.Strategy = argv[i+1]
			i += 2
		case "--mount":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--mount requires an argument")
			}
			o.ExtraMounts = append(o.ExtraMounts, argv[i+1])
			i += 2
		case "--env":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--env requires an argument")
			}
			o.ExtraEnvLines = append(o.ExtraEnvLines, argv[i+1])
			i += 2
		case "--env-file":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--env-file requires an argument")
			}
			o.EnvFiles = append(o.EnvFiles, argv[i+1])
			i += 2
		case "--no-op-inject":
			o.NoOpInject = true
			i++
		case "--var":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--var requires KEY=VAL")
			}
			if !strings.Contains(argv[i+1], "=") {
				return nil, nil, fmt.Errorf("--var requires KEY=VAL")
			}
			o.VarOverrides = append(o.VarOverrides, argv[i+1])
			i += 2
		case "--compile-deps":
			o.CompileDeps = true
			i++
		case "--no-compile-deps":
			o.NoCompileDeps = true
			i++
		case "--tf":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--tf requires a command list (e.g. plan, apply, init,plan)")
			}
			o.TfCommands = argv[i+1]
			i += 2
		case "--tf-dry-run":
			o.TfDryRun = true
			i++
		case "--tf-no-auto-approve":
			o.TfNoAutoApprove = true
			i++
		case "--build":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--build requires an argument")
			}
			p := argv[i+1]
			// POSIX-style absolute (e.g. /abs/build) is not filepath.IsAbs on Windows but is valid for container paths.
			if isPosixAbsPath(p) {
				o.BuildPath = filepath.ToSlash(filepath.Clean(p))
			} else if filepath.IsAbs(p) {
				o.BuildPath = p
			} else {
				o.BuildPath = filepath.Join(repoRoot, p)
			}
			i += 2
		default:
			if strings.HasPrefix(a, "--tf=") {
				o.TfCommands = strings.TrimPrefix(a, "--tf=")
				if strings.TrimSpace(o.TfCommands) == "" {
					return nil, nil, fmt.Errorf("--tf= requires a non-empty value")
				}
				i++
				continue
			}
			if strings.HasPrefix(a, "-") {
				return nil, nil, fmt.Errorf("unknown option %s", a)
			}
			if a == "build" || a == "clean" || a == "rebuild" {
				return nil, nil, fmt.Errorf("%q is a subcommand in current dockpipe — your binary is outdated (run: make build && make install, or use ./src/bin/dockpipe from the repo)", a)
			}
			return nil, nil, fmt.Errorf("unexpected argument %q (expected options before --)", a)
		}
	}
	if o.Workflow != "" && o.WorkflowFile != "" {
		return nil, nil, fmt.Errorf("use either --workflow or --workflow-file, not both")
	}
	return nil, o, nil
}
