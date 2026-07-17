package application

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

const sdkUsageText = `dockpipe sdk — bootstrap SDK helpers for supported languages

Usage:
  dockpipe sdk [--workdir <path>]
  dockpipe sdk shell-env [--workdir <path>]

By default, sdk emits shell code suitable for:

  eval "$(dockpipe sdk)"

When --workdir is omitted, sdk prefers DOCKPIPE_WORKDIR from the
environment and otherwise falls back to the current working directory.

This bootstraps the shell SDK object from the canonical core helper.
`

const getUsageText = `dockpipe get — print generic DockPipe context fields

Usage:
  dockpipe get <field> [--workdir <path>]

Fields:
  workdir
  dockpipe_bin
  workflow_name
  script_dir
  package_root
  assets_dir
  state_dir
  artifact_root
  event_log
  event_index
  output_root
  package_id
  package_state_dir

When --workdir is omitted, get prefers DOCKPIPE_WORKDIR from the
environment and otherwise falls back to the current working directory.

Fields such as script_dir, package_root, and assets_dir are env-backed and
print the current injected values when available.
`

const scopeUsageText = `dockpipe scope — resolve workflow/package scope objects and paths

Usage:
  dockpipe scope [--workdir <path>]
  dockpipe scope <scope> [path ...] [--workdir <path>]
  dockpipe scope --package <name> [path ...] [--workdir <path>]
  dockpipe scope workflow <name> [path ...] [--workdir <path>]
  dockpipe scope resolver <name> <field> [--workdir <path>]

Built-in scopes:
  source, repo, workdir
  artifacts, artifact, output

With no positional scope, prints the current workflow scope object as JSON.
Custom scope names resolve under the current workflow output root:
  <output-root>/scopes/<scope>

With --package and no path, prints the package scope object as JSON. With path
segments, prints a path under that package scope.

Workflow scope paths resolve under a named workflow artifact root. Use this when a
workflow needs to read artifacts from another workflow.

Resolver scope fields are read from the resolver profile:
  auth-dir, container-auth-dir, auth-mount-mode, config-file, container-config-file

Examples:
  dockpipe scope
  dockpipe scope source src
  dockpipe scope artifacts providers/codex/result.json
  dockpipe scope name.123213
  dockpipe scope workflow docs.orchestrate orchestrate
  dockpipe scope --package dorkpipe
  dockpipe scope --package dorkpipe training metrics.jsonl
  dockpipe scope resolver codex auth-dir
`

func cmdSDK(args []string) error {
	if len(args) == 0 {
		return cmdSDKShellEnv(nil)
	}
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Print(sdkUsageText)
		return nil
	}
	if strings.HasPrefix(args[0], "-") {
		return cmdSDKShellEnv(args)
	}
	switch args[0] {
	case "shell-env":
		return cmdSDKShellEnv(args[1:])
	default:
		return fmt.Errorf("unknown sdk subcommand %q (try: dockpipe sdk --help)", args[0])
	}
}

func cmdGet(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(getUsageText)
		return nil
	}
	field := normalizeGetField(args[0])
	workdir, err := parseSDKWorkdirFlag(args[1:])
	if err != nil {
		return err
	}
	switch field {
	case "workdir":
		fmt.Println(workdir)
		return nil
	case "dockpipe_bin":
		bin, err := resolveDockpipeBinForSDK(workdir)
		if err != nil {
			return err
		}
		fmt.Println(bin)
		return nil
	case "workflow_name":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_WORKFLOW_NAME is not set")
		}
		fmt.Println(v)
		return nil
	case "script_dir":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_SCRIPT_DIR"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_SCRIPT_DIR is not set")
		}
		fmt.Println(v)
		return nil
	case "package_root":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_PACKAGE_ROOT"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_PACKAGE_ROOT is not set")
		}
		fmt.Println(v)
		return nil
	case "assets_dir":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_ASSETS_DIR"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_ASSETS_DIR is not set")
		}
		fmt.Println(v)
		return nil
	case "state_dir":
		v := strings.TrimSpace(os.Getenv(infrastructure.EnvStateDir))
		if v == "" {
			v, err = infrastructure.StateRoot(workdir)
			if err != nil {
				return err
			}
		}
		fmt.Println(v)
		return nil
	case "artifact_root":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
		if v == "" {
			v, err = workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
			if err != nil {
				return err
			}
		}
		fmt.Println(v)
		return nil
	case "event_log":
		v, err := resolveEventLogPath(workdir)
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	case "event_index":
		v, err := resolveEventIndexPath(workdir)
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	case "output_root":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_OUTPUT_ROOT"))
		if v == "" {
			v = strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
		}
		if v == "" {
			v, err = workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
			if err != nil {
				return err
			}
		}
		fmt.Println(v)
		return nil
	case "package_id":
		v := strings.TrimSpace(os.Getenv(infrastructure.EnvPackageID))
		if v == "" {
			root := strings.TrimSpace(os.Getenv("DOCKPIPE_PACKAGE_ROOT"))
			if root != "" {
				v = filepath.Base(root)
			}
		}
		if v == "" {
			return fmt.Errorf("DOCKPIPE_PACKAGE_ID is not set")
		}
		fmt.Println(infrastructure.SanitizePackageStateScope(v))
		return nil
	case "package_state_dir":
		v := strings.TrimSpace(os.Getenv(infrastructure.EnvPackageStateDir))
		if v == "" {
			scope := strings.TrimSpace(os.Getenv(infrastructure.EnvPackageID))
			if scope == "" {
				root := strings.TrimSpace(os.Getenv("DOCKPIPE_PACKAGE_ROOT"))
				if root != "" {
					scope = filepath.Base(root)
				}
			}
			if scope == "" {
				return fmt.Errorf("DOCKPIPE_PACKAGE_STATE_DIR is not set")
			}
			v, err = infrastructure.PackageStateDir(workdir, scope)
			if err != nil {
				return err
			}
		}
		fmt.Println(v)
		return nil
	default:
		return fmt.Errorf("unknown get field %q (try: dockpipe get --help)", args[0])
	}
}

func cmdScope(args []string) error {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(scopeUsageText)
		return nil
	}
	parsed, err := parseScopeArgs(args)
	if err != nil {
		return err
	}
	if parsed.packageName != "" {
		obj, err := packageScopeObject(parsed.workdir, parsed.packageName)
		if err != nil {
			return err
		}
		if len(parsed.suffix) == 0 {
			return printScopeObject(obj)
		}
		parts := append([]string{obj.Root}, parsed.suffix...)
		fmt.Println(filepath.Clean(filepath.Join(parts...)))
		return nil
	}
	if parsed.scope == "" {
		obj, err := workflowScopeObject(parsed.workdir)
		if err != nil {
			return err
		}
		return printScopeObject(obj)
	}
	if normalizeGetField(parsed.scope) == "resolver" {
		value, err := resolverScopeValue(parsed.workdir, parsed.suffix)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	}
	if normalizeGetField(parsed.scope) == "workflow" {
		value, err := workflowScopePath(parsed.workdir, parsed.suffix)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	}
	root, err := resolveScopeRoot(parsed.scope, parsed.workdir)
	if err != nil {
		return err
	}
	parts := append([]string{root}, parsed.suffix...)
	fmt.Println(filepath.Clean(filepath.Join(parts...)))
	return nil
}

type scopeArgs struct {
	scope       string
	packageName string
	workdir     string
	suffix      []string
}

type scopeObject struct {
	Kind         string `json:"kind"`
	Name         string `json:"name,omitempty"`
	Scope        string `json:"scope"`
	Root         string `json:"root"`
	DockpipeBin  string `json:"dockpipe_bin,omitempty"`
	SourceRoot   string `json:"source_root,omitempty"`
	ArtifactRoot string `json:"artifact_root,omitempty"`
	EventLog     string `json:"event_log,omitempty"`
	EventIndex   string `json:"event_index,omitempty"`
	OutputRoot   string `json:"output_root,omitempty"`
	StateRoot    string `json:"state_root,omitempty"`
	Workdir      string `json:"workdir"`
}

func printScopeObject(obj scopeObject) error {
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func cmdSDKShellEnv(args []string) error {
	workdir, err := parseSDKWorkdirFlag(args)
	if err != nil {
		return err
	}
	sdkPath, err := resolveShellSDKPath(workdir)
	if err != nil {
		return err
	}

	fmt.Printf("export DOCKPIPE_WORKDIR=%s\n", shellQuote(workdir))
	fmt.Printf("source %s\n", shellQuote(sdkPath))
	return nil
}

func parseScopeArgs(args []string) (scopeArgs, error) {
	parsed := scopeArgs{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workdir":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("--workdir requires a path")
			}
			parsed.workdir = strings.TrimSpace(args[i+1])
			i++
		case "--package":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("--package requires a name")
			}
			parsed.packageName = strings.TrimSpace(args[i+1])
			i++
		case "-h", "--help":
			return parsed, fmt.Errorf("usage: dockpipe scope [<scope> [path ...] | --package <name> [path ...]] [--workdir <path>]")
		default:
			positionals = append(positionals, args[i])
		}
	}
	if parsed.packageName == "" && len(positionals) > 0 {
		parsed.scope = strings.TrimSpace(positionals[0])
		parsed.suffix = positionals[1:]
	} else {
		parsed.suffix = positionals
	}
	workdir, err := resolveSDKWorkdir(parsed.workdir)
	if err != nil {
		return parsed, err
	}
	parsed.workdir = workdir
	return parsed, nil
}

func parseSDKWorkdirFlag(args []string) (string, error) {
	workdir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workdir":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--workdir requires a path")
			}
			workdir = strings.TrimSpace(args[i+1])
			i++
		case "-h", "--help":
			return "", fmt.Errorf("usage: dockpipe sdk shell-env [--workdir <path>]")
		default:
			return "", fmt.Errorf("unknown option %q", args[i])
		}
	}

	return resolveSDKWorkdir(workdir)
}

func resolveSDKWorkdir(workdir string) (string, error) {
	if strings.TrimSpace(workdir) == "" {
		workdir = strings.TrimSpace(os.Getenv("DOCKPIPE_WORKDIR"))
		if workdir == "" {
			wd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			workdir = wd
		}
	}
	workdir = infrastructure.HostPathForGit(workdir)
	if abs, err := filepath.Abs(workdir); err != nil {
		return "", err
	} else {
		workdir = abs
	}
	return workdir, nil
}

func workflowScopeObject(workdir string) (scopeObject, error) {
	outputRoot, err := resolveOutputRoot(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	stateRoot, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	artifactRoot := strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
	if artifactRoot == "" {
		artifactRoot, err = workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
		if err != nil {
			return scopeObject{}, err
		}
	}
	eventLog, err := resolveEventLogPath(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	eventIndex, err := resolveEventIndexPath(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	sourceRoot := firstNonEmpty(strings.TrimSpace(os.Getenv("DOCKPIPE_SOURCE_ROOT")), workdir)
	name := strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME"))
	dockpipeBin, err := resolveDockpipeBinForSDK(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	return scopeObject{
		Kind:         "workflow",
		Name:         name,
		Scope:        sanitizeWorkflowStateScope(name),
		Root:         outputRoot,
		DockpipeBin:  dockpipeBin,
		SourceRoot:   sourceRoot,
		ArtifactRoot: artifactRoot,
		EventLog:     eventLog,
		EventIndex:   eventIndex,
		OutputRoot:   outputRoot,
		StateRoot:    filepath.Join(stateRoot, "workflows", sanitizeWorkflowStateScope(name)),
		Workdir:      workdir,
	}, nil
}

func packageScopeObject(workdir, packageName string) (scopeObject, error) {
	scope := infrastructure.SanitizePackageStateScope(packageName)
	root, err := infrastructure.PackageStateDir(workdir, scope)
	if err != nil {
		return scopeObject{}, err
	}
	stateRoot, err := infrastructure.StateRoot(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	dockpipeBin, err := resolveDockpipeBinForSDK(workdir)
	if err != nil {
		return scopeObject{}, err
	}
	return scopeObject{
		Kind:        "package",
		Name:        packageName,
		Scope:       scope,
		Root:        root,
		DockpipeBin: dockpipeBin,
		StateRoot:   stateRoot,
		Workdir:     workdir,
	}, nil
}

func resolveScopeRoot(scope, workdir string) (string, error) {
	normalized := normalizeGetField(scope)
	switch normalized {
	case "source", "repo", "workdir":
		return firstNonEmpty(strings.TrimSpace(os.Getenv("DOCKPIPE_SOURCE_ROOT")), workdir), nil
	case "artifacts", "artifact", "output":
		return resolveOutputRoot(workdir)
	default:
		outputRoot, err := resolveOutputRoot(workdir)
		if err != nil {
			return "", err
		}
		return filepath.Join(outputRoot, "scopes", sanitizeNamedScope(scope)), nil
	}
}

func resolverScopeValue(workdir string, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: dockpipe scope resolver <name> <auth-dir|container-auth-dir|auth-mount-mode|config-file|container-config-file>")
	}
	resolverName := strings.TrimSpace(args[0])
	field := normalizeGetField(args[1])
	profilePath, err := infrastructure.ResolveResolverFilePath(workdir, resolverName)
	if err != nil {
		return "", err
	}
	profile, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return "", err
	}
	switch field {
	case "auth_dir":
		return resolveResolverHostPath(profile, "DOCKPIPE_RESOLVER_AUTH_DIR_ENV", "DOCKPIPE_RESOLVER_AUTH_DIR")
	case "container_auth_dir":
		return strings.TrimSpace(profile["DOCKPIPE_RESOLVER_CONTAINER_AUTH_DIR"]), nil
	case "auth_mount_mode":
		return firstNonEmpty(strings.TrimSpace(profile["DOCKPIPE_RESOLVER_AUTH_MOUNT_MODE"]), "rw"), nil
	case "config_file":
		return resolveResolverHostPath(profile, "DOCKPIPE_RESOLVER_CONFIG_FILE_ENV", "DOCKPIPE_RESOLVER_CONFIG_FILE")
	case "container_config_file":
		return strings.TrimSpace(profile["DOCKPIPE_RESOLVER_CONTAINER_CONFIG_FILE"]), nil
	default:
		return "", fmt.Errorf("unknown resolver scope field %q", args[1])
	}
}

func workflowScopePath(workdir string, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: dockpipe scope workflow <name> [path ...]")
	}
	workflowName := strings.TrimSpace(args[0])
	root, err := workflowArtifactRoot(workdir, workflowName)
	if err != nil {
		return "", err
	}
	parts := append([]string{root}, args[1:]...)
	return filepath.Clean(filepath.Join(parts...)), nil
}

func resolveResolverHostPath(profile map[string]string, envKey, defaultKey string) (string, error) {
	if envName := strings.TrimSpace(profile[envKey]); envName != "" {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			return value, nil
		}
	}
	value := strings.TrimSpace(profile[defaultKey])
	if value == "" {
		return "", nil
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = strings.TrimSpace(userHome)
		}
	}
	if home == "" {
		return "", fmt.Errorf("user home directory is not set")
	}
	return filepath.Join(home, filepath.Clean(value)), nil
}

func resolveOutputRoot(workdir string) (string, error) {
	v := strings.TrimSpace(os.Getenv("DOCKPIPE_OUTPUT_ROOT"))
	if v == "" {
		v = strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
	}
	if v != "" {
		return v, nil
	}
	return workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
}

func resolveEventLogPath(workdir string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(infrastructure.EnvDockpipeEventLog)); v != "" {
		return v, nil
	}
	artifactRoot := strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
	if artifactRoot == "" {
		var err error
		artifactRoot, err = workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(artifactRoot, "events.jsonl"), nil
}

func resolveEventIndexPath(workdir string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(infrastructure.EnvDockpipeEventIndex)); v != "" {
		return v, nil
	}
	artifactRoot := strings.TrimSpace(os.Getenv("DOCKPIPE_ARTIFACT_ROOT"))
	if artifactRoot == "" {
		var err error
		artifactRoot, err = workflowArtifactRoot(workdir, strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME")))
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(artifactRoot, "events-index.json"), nil
}

func sanitizeNamedScope(scope string) string {
	scope = strings.TrimSpace(scope)
	var b strings.Builder
	lastDash := false
	for _, r := range scope {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}

func resolveDockpipeBinForSDK(workdir string) (string, error) {
	if local := resolveRepoLocalDockpipeBin(workdir); local != "" {
		return local, nil
	}
	if configured := strings.TrimSpace(os.Getenv("DOCKPIPE_BIN")); configured != "" {
		return configured, nil
	}
	path, err := exec.LookPath("dockpipe")
	if err != nil {
		return "", fmt.Errorf("dockpipe binary not found; set DOCKPIPE_BIN or add dockpipe to PATH")
	}
	return path, nil
}

var osExecutableFn = os.Executable

func resolveDockpipeBinForChildProcess(workdir string) (string, error) {
	if exe, err := osExecutableFn(); err == nil {
		exe = strings.TrimSpace(exe)
		if exe != "" {
			if st, statErr := os.Stat(exe); statErr == nil && !st.IsDir() {
				return exe, nil
			}
		}
	}
	return resolveDockpipeBinForSDK(workdir)
}

func resolveRepoLocalDockpipeBin(workdir string) string {
	candidates := []string{
		filepath.Join(workdir, "src", "bin", "dockpipe.exe"),
		filepath.Join(workdir, "src", "bin", "dockpipe"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return ""
}

func normalizeGetField(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func resolveShellSDKPath(workdir string) (string, error) {
	for cur := workdir; ; cur = filepath.Dir(cur) {
		if infrastructure.DockpipeAuthoringSourceTree(cur) {
			return filepath.Join(infrastructure.CoreDir(cur), "assets", "scripts", "lib", "dockpipe-sdk.sh"), nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
	}
	rr, err := infrastructure.RepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(infrastructure.CoreDir(rr), "assets", "scripts", "lib", "dockpipe-sdk.sh"), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
