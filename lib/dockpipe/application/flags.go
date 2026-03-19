package application

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CliOpts holds parsed CLI flags (before --).
type CliOpts struct {
	Help          bool
	Detach        bool
	Force         bool
	Reinit        bool
	DataVolume    string
	DataDir       string
	NoData        bool
	PreScripts    []string
	Isolate       string
	Action        string
	Workflow      string
	Workdir       string
	RepoURL       string
	RepoBranch    string
	WorkPath      string
	WorkBranch    string
	BundleOut     string
	Resolver      string
	ExtraMounts   []string
	ExtraEnvLines []string
	EnvFiles      []string
	VarOverrides  []string
	BuildPath     string
	SeenDash      bool
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
		case "--resolver":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--resolver requires an argument")
			}
			o.Resolver = argv[i+1]
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
		case "--var":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--var requires KEY=VAL")
			}
			if !strings.Contains(argv[i+1], "=") {
				return nil, nil, fmt.Errorf("--var requires KEY=VAL")
			}
			o.VarOverrides = append(o.VarOverrides, argv[i+1])
			i += 2
		case "--build":
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("--build requires an argument")
			}
			p := argv[i+1]
			if filepath.IsAbs(p) {
				o.BuildPath = p
			} else {
				o.BuildPath = filepath.Join(repoRoot, p)
			}
			i += 2
		default:
			if strings.HasPrefix(a, "-") {
				return nil, nil, fmt.Errorf("unknown option %s", a)
			}
			return nil, nil, fmt.Errorf("unexpected argument %q (expected options before --)", a)
		}
	}
	return nil, o, nil
}
