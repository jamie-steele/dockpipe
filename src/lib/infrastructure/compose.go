package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var composeExecCommandFn = exec.CommandContext

type ComposeLifecycleOpts struct {
	Action           string
	File             string
	Project          string
	ProjectDirectory string
	Services         []string
	Env              []string
}

func RunComposeLifecycle(opts ComposeLifecycleOpts) error {
	file := strings.TrimSpace(opts.File)
	if file == "" {
		return fmt.Errorf("compose file is required")
	}
	action := strings.TrimSpace(opts.Action)
	if action == "" {
		return fmt.Errorf("compose action is required")
	}
	projectDir := strings.TrimSpace(opts.ProjectDirectory)
	if projectDir == "" {
		projectDir = filepath.Dir(file)
	}
	args := []string{"compose"}
	if project := strings.TrimSpace(opts.Project); project != "" {
		args = append(args, "-p", project)
	}
	args = append(args, "-f", file)
	if projectDir != "" {
		args = append(args, "--project-directory", projectDir)
	}
	switch action {
	case "up":
		args = append(args, "up", "-d", "--remove-orphans")
	case "down":
		args = append(args, "down")
	case "ps":
		args = append(args, "ps")
	default:
		return fmt.Errorf("unsupported compose action %q", action)
	}
	for _, service := range opts.Services {
		service = strings.TrimSpace(service)
		if service != "" {
			args = append(args, service)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := composeExecCommandFn(ctx, "docker", args...)
	cmd.Env = mergeComposeProcessEnv(opts.Env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func mergeComposeProcessEnv(overrides []string) []string {
	base := append([]string(nil), os.Environ()...)
	for _, pair := range overrides {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, _, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		prefix := key + "="
		replaced := false
		for i, existing := range base {
			if strings.HasPrefix(existing, prefix) {
				base[i] = pair
				replaced = true
				break
			}
		}
		if !replaced {
			base = append(base, pair)
		}
	}
	return base
}
