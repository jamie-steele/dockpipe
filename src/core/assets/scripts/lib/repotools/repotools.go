package repotools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

type SDK struct {
	Workdir      string
	DockpipeBin  string
	WorkflowName string
	ScriptDir    string
	PackageRoot  string
	AssetsDir    string
}

type ScopeObject struct {
	Kind         string `json:"kind"`
	Name         string `json:"name,omitempty"`
	Scope        string `json:"scope"`
	Root         string `json:"root"`
	DockpipeBin  string `json:"dockpipe_bin,omitempty"`
	SourceRoot   string `json:"source_root,omitempty"`
	ArtifactRoot string `json:"artifact_root,omitempty"`
	OutputRoot   string `json:"output_root,omitempty"`
	StateRoot    string `json:"state_root,omitempty"`
	Workdir      string `json:"workdir"`
}

type PromptSpec struct {
	Type              string   `json:"type"`
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Message           string   `json:"message"`
	Default           string   `json:"default"`
	Sensitive         bool     `json:"sensitive"`
	Intent            string   `json:"intent,omitempty"`
	AutomationGroup   string   `json:"automation_group,omitempty"`
	AllowAutoApprove  bool     `json:"allow_auto_approve,omitempty"`
	AutoApproveValue  string   `json:"auto_approve_value,omitempty"`
	PathMode          string   `json:"path_mode,omitempty"`
	FileFilter        string   `json:"file_filter,omitempty"`
	MustExist         bool     `json:"must_exist,omitempty"`
	BaseDir           string   `json:"base_dir,omitempty"`
	ResourceMode      string   `json:"resource_mode,omitempty"`
	ResourceSelection string   `json:"resource_selection,omitempty"`
	ResourceKind      string   `json:"resource_kind,omitempty"`
	Filters           []string `json:"filters,omitempty"`
	Options           []string `json:"options"`
}

func RepoRoot(root string) (string, error) {
	if root == "" {
		root = os.Getenv("DOCKPIPE_WORKDIR")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return filepath.Abs(root)
}

func ResolveDockpipeBin(root string) (string, error) {
	if configured := os.Getenv("DOCKPIPE_BIN"); configured != "" {
		return configured, nil
	}
	resolvedRoot, err := RepoRoot(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(resolvedRoot, "src", "bin", "dockpipe")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}
	return exec.LookPath("dockpipe")
}

func Load(root string) (SDK, error) {
	workdir, err := RepoRoot(root)
	if err != nil {
		return SDK{}, err
	}
	dockpipeBin, _ := ResolveDockpipeBin(workdir)
	return SDK{
		Workdir:      workdir,
		DockpipeBin:  dockpipeBin,
		WorkflowName: os.Getenv("DOCKPIPE_WORKFLOW_NAME"),
		ScriptDir:    os.Getenv("DOCKPIPE_SCRIPT_DIR"),
		PackageRoot:  os.Getenv("DOCKPIPE_PACKAGE_ROOT"),
		AssetsDir:    os.Getenv("DOCKPIPE_ASSETS_DIR"),
	}, nil
}

func (s SDK) scopeOutput(args ...string) ([]byte, error) {
	if strings.TrimSpace(s.DockpipeBin) == "" {
		return nil, fmt.Errorf("dockpipe binary not found; set DOCKPIPE_BIN or add dockpipe to PATH")
	}
	argv := []string{"scope", "--workdir", s.Workdir}
	argv = append(argv, args...)
	return exec.Command(s.DockpipeBin, argv...).Output()
}

func (s SDK) WorkflowScope() (ScopeObject, error) {
	out, err := s.scopeOutput()
	if err != nil {
		return ScopeObject{}, err
	}
	var obj ScopeObject
	if err := json.Unmarshal(out, &obj); err != nil {
		return ScopeObject{}, err
	}
	return obj, nil
}

func (s SDK) PackageScope(name string) (ScopeObject, error) {
	out, err := s.scopeOutput("--package", name)
	if err != nil {
		return ScopeObject{}, err
	}
	var obj ScopeObject
	if err := json.Unmarshal(out, &obj); err != nil {
		return ScopeObject{}, err
	}
	return obj, nil
}

func (s SDK) ScopePath(scope string, parts ...string) (string, error) {
	args := append([]string{scope}, parts...)
	out, err := s.scopeOutput(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (s SDK) PackageScopePath(name string, parts ...string) (string, error) {
	args := append([]string{"--package", name}, parts...)
	out, err := s.scopeOutput(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func fileDescriptorInt(file *os.File) (int, error) {
	fd := file.Fd()
	maxInt := int(^uint(0) >> 1)
	if fd > uintptr(maxInt) {
		return 0, fmt.Errorf("file descriptor %d overflows int", fd)
	}
	return int(fd), nil // #nosec G115 -- fd is checked against max int before conversion.
}

func promptMode() string {
	switch os.Getenv("DOCKPIPE_SDK_PROMPT_MODE") {
	case "json":
		return "json"
	case "terminal":
		return "terminal"
	}
	stdinFD, stdinErr := fileDescriptorInt(os.Stdin)
	stderrFD, stderrErr := fileDescriptorInt(os.Stderr)
	if stdinErr == nil && stderrErr == nil && term.IsTerminal(stdinFD) && term.IsTerminal(stderrFD) {
		return "terminal"
	}
	return "noninteractive"
}

func normalizeResourceEntries(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if len(candidate) >= 2 {
			first := candidate[0]
			last := candidate[len(candidate)-1]
			if (first == '"' || first == '\'') && first == last {
				candidate = strings.TrimSpace(candidate[1 : len(candidate)-1])
			}
		}
		if candidate != "" {
			out = append(out, candidate)
		}
	}
	return out
}

func resourceExists(baseDir, resourceKind, value string) bool {
	resolved := value
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(baseDir, value)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return false
	}
	if resourceKind == "directory" {
		return info.IsDir()
	}
	return true
}

func Prompt(spec PromptSpec) (string, error) {
	if strings.TrimSpace(spec.ID) == "" {
		spec.ID = fmt.Sprintf("prompt.%d", os.Getpid())
	}
	if strings.TrimSpace(spec.Message) == "" {
		spec.Message = spec.Title
	}
	if strings.TrimSpace(spec.BaseDir) == "" {
		spec.BaseDir = os.Getenv("DOCKPIPE_WORKDIR")
		if strings.TrimSpace(spec.BaseDir) == "" {
			if wd, err := os.Getwd(); err == nil {
				spec.BaseDir = wd
			}
		}
	}
	if spec.AllowAutoApprove && truthy(os.Getenv("DOCKPIPE_APPROVE_PROMPTS")) {
		if strings.TrimSpace(spec.AutoApproveValue) != "" {
			return spec.AutoApproveValue, nil
		}
		if strings.TrimSpace(spec.Default) != "" {
			return spec.Default, nil
		}
		switch spec.Type {
		case "confirm":
			return "yes", nil
		case "choice":
			if len(spec.Options) > 0 {
				return spec.Options[0], nil
			}
		case "file", "resource":
			if strings.TrimSpace(spec.Default) != "" {
				return spec.Default, nil
			}
		}
	}

	switch promptMode() {
	case "json":
		b, err := json.Marshal(spec)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(os.Stderr, "::dockpipe-prompt::%s\n", string(b))
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	case "terminal":
		if spec.Title != "" {
			fmt.Fprintln(os.Stderr, spec.Title)
		}
		switch spec.Type {
		case "confirm":
			defaultYes := truthy(spec.Default)
			suffix := " [y/N] "
			if defaultYes {
				suffix = " [Y/n] "
			}
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Fprintf(os.Stderr, "%s%s", spec.Message, suffix)
				line, err := reader.ReadString('\n')
				if err != nil && len(line) == 0 {
					return "", err
				}
				answer := strings.TrimSpace(line)
				if answer == "" {
					if defaultYes {
						return "yes", nil
					}
					return "no", nil
				}
				if truthy(answer) {
					return "yes", nil
				}
				switch strings.ToLower(answer) {
				case "n", "no", "false", "0":
					return "no", nil
				}
				fmt.Fprintln(os.Stderr, "Please answer yes or no.")
			}
		case "input":
			if spec.Default != "" {
				fmt.Fprintf(os.Stderr, "%s [%s]: ", spec.Message, spec.Default)
			} else {
				fmt.Fprintf(os.Stderr, "%s: ", spec.Message)
			}
			if spec.Sensitive {
				stdinFD, err := fileDescriptorInt(os.Stdin)
				if err != nil {
					return "", err
				}
				b, err := term.ReadPassword(stdinFD)
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return "", err
				}
				if len(b) == 0 {
					return spec.Default, nil
				}
				return string(b), nil
			}
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil && len(line) == 0 {
				return "", err
			}
			out := strings.TrimRight(line, "\r\n")
			if out == "" {
				return spec.Default, nil
			}
			return out, nil
		case "choice":
			if len(spec.Options) == 0 {
				return "", fmt.Errorf("dockpipe prompt choice requires at least one option")
			}
			fmt.Fprintln(os.Stderr, spec.Message)
			defaultIndex := 1
			for i, option := range spec.Options {
				if spec.Default != "" && option == spec.Default {
					defaultIndex = i + 1
				}
				fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, option)
			}
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Fprintf(os.Stderr, "Choose an option [%d]: ", defaultIndex)
				line, err := reader.ReadString('\n')
				if err != nil && len(line) == 0 {
					return "", err
				}
				answer := strings.TrimSpace(line)
				if answer == "" {
					return spec.Options[defaultIndex-1], nil
				}
				var selected int
				if _, err := fmt.Sscanf(answer, "%d", &selected); err == nil && selected >= 1 && selected <= len(spec.Options) {
					return spec.Options[selected-1], nil
				}
				fmt.Fprintf(os.Stderr, "Enter a number between 1 and %d.\n", len(spec.Options))
			}
		case "file":
			reader := bufio.NewReader(os.Stdin)
			for {
				if spec.FileFilter != "" {
					fmt.Fprintf(os.Stderr, "Filter: %s\n", spec.FileFilter)
				}
				if spec.Default != "" {
					fmt.Fprintf(os.Stderr, "%s [%s]: ", spec.Message, spec.Default)
				} else {
					fmt.Fprintf(os.Stderr, "%s: ", spec.Message)
				}
				line, err := reader.ReadString('\n')
				if err != nil && len(line) == 0 {
					return "", err
				}
				out := strings.TrimRight(line, "\r\n")
				if out == "" {
					out = spec.Default
				}
				if out == "" {
					return out, nil
				}
				if spec.MustExist {
					resolved := out
					if !filepath.IsAbs(resolved) {
						resolved = filepath.Join(spec.BaseDir, out)
					}
					info, err := os.Stat(resolved)
					if err != nil {
						label := "File"
						if spec.PathMode == "open-dir" {
							label = "Directory"
						}
						fmt.Fprintf(os.Stderr, "%s not found: %s\n", label, out)
						continue
					}
					if spec.PathMode == "open-dir" && !info.IsDir() {
						fmt.Fprintf(os.Stderr, "Directory not found: %s\n", out)
						continue
					}
				}
				return out, nil
			}
		case "resource":
			reader := bufio.NewReader(os.Stdin)
			filterText := spec.FileFilter
			if len(spec.Filters) > 0 {
				filterText = strings.Join(spec.Filters, ";;")
			}
			resourceSelection := spec.ResourceSelection
			if resourceSelection == "" {
				resourceSelection = "single"
			}
			resourceKind := spec.ResourceKind
			if resourceKind == "" {
				resourceKind = "file"
			}
			for {
				if filterText != "" {
					fmt.Fprintf(os.Stderr, "Filter: %s\n", filterText)
				}
				promptText := spec.Message
				if resourceSelection == "multi" {
					promptText += " (separate multiple paths with ;)"
				}
				if spec.Default != "" {
					fmt.Fprintf(os.Stderr, "%s [%s]: ", promptText, spec.Default)
				} else {
					fmt.Fprintf(os.Stderr, "%s: ", promptText)
				}
				line, err := reader.ReadString('\n')
				if err != nil && len(line) == 0 {
					return "", err
				}
				out := strings.TrimRight(line, "\r\n")
				if out == "" {
					out = spec.Default
				}
				if out == "" {
					return out, nil
				}
				if resourceSelection == "multi" {
					entries := normalizeResourceEntries(out)
					if len(entries) == 0 {
						return "[]", nil
					}
					if spec.MustExist {
						missing := ""
						for _, entry := range entries {
							if !resourceExists(spec.BaseDir, resourceKind, entry) {
								missing = entry
								break
							}
						}
						if missing != "" {
							label := "File"
							if resourceKind == "directory" {
								label = "Directory"
							}
							fmt.Fprintf(os.Stderr, "%s not found: %s\n", label, missing)
							continue
						}
					}
					b, err := json.Marshal(entries)
					if err != nil {
						return "", err
					}
					return string(b), nil
				}
				entries := normalizeResourceEntries(out)
				if len(entries) == 0 {
					return "", nil
				}
				selected := entries[0]
				if spec.MustExist && !resourceExists(spec.BaseDir, resourceKind, selected) {
					label := "File"
					if resourceKind == "directory" {
						label = "Directory"
					}
					fmt.Fprintf(os.Stderr, "%s not found: %s\n", label, selected)
					continue
				}
				return selected, nil
			}
		default:
			return "", fmt.Errorf("unsupported DockPipe prompt kind: %s", spec.Type)
		}
	default:
		if strings.TrimSpace(spec.Default) != "" {
			return spec.Default, nil
		}
		return "", fmt.Errorf("dockpipe prompt requires a terminal or DOCKPIPE_SDK_PROMPT_MODE=json")
	}
}
