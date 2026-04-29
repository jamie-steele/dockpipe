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

type PromptSpec struct {
	Type             string   `json:"type"`
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Message          string   `json:"message"`
	Default          string   `json:"default"`
	Sensitive        bool     `json:"sensitive"`
	Intent           string   `json:"intent,omitempty"`
	AutomationGroup  string   `json:"automation_group,omitempty"`
	AllowAutoApprove bool     `json:"allow_auto_approve,omitempty"`
	AutoApproveValue string   `json:"auto_approve_value,omitempty"`
	Options          []string `json:"options"`
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

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func promptMode() string {
	if os.Getenv("DOCKPIPE_SDK_PROMPT_MODE") == "json" {
		return "json"
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) {
		return "terminal"
	}
	return "noninteractive"
}

func Prompt(spec PromptSpec) (string, error) {
	if strings.TrimSpace(spec.ID) == "" {
		spec.ID = fmt.Sprintf("prompt.%d", os.Getpid())
	}
	if strings.TrimSpace(spec.Message) == "" {
		spec.Message = spec.Title
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
				b, err := term.ReadPassword(int(os.Stdin.Fd()))
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
