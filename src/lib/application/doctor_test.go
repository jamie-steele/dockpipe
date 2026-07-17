package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func withDoctorSeams(t *testing.T) {
	t.Helper()
	oldBashLookPath := doctorBashLookPathFn
	oldDockerCheck := doctorDockerCheckFn
	oldRepoRoot := doctorRepoRootFn
	oldResolveWorkflow := doctorResolveWorkflowConfigPathFn
	oldGetwd := doctorGetwdFn
	oldLoadConfig := doctorLoadProjectConfigFn
	oldStat := doctorStatFn
	oldOpLookPath := opLookPathFn
	t.Cleanup(func() {
		doctorBashLookPathFn = oldBashLookPath
		doctorDockerCheckFn = oldDockerCheck
		doctorRepoRootFn = oldRepoRoot
		doctorResolveWorkflowConfigPathFn = oldResolveWorkflow
		doctorGetwdFn = oldGetwd
		doctorLoadProjectConfigFn = oldLoadConfig
		doctorStatFn = oldStat
		opLookPathFn = oldOpLookPath
	})
}

func TestCmdDoctorEmitsOperationResults(t *testing.T) {
	withDoctorSeams(t)
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.vault.template")
	if err := os.WriteFile(templatePath, []byte("TOKEN=op://vault/item/field\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vault := "op"
	notes := "keep secrets in 1Password references only"
	doctorBashLookPathFn = func(file string) (string, error) {
		if file != "bash" {
			t.Fatalf("unexpected lookpath target %q", file)
		}
		return "/bin/bash", nil
	}
	doctorDockerCheckFn = func(stderr *os.File) error { return nil }
	doctorRepoRootFn = func() (string, error) { return "/repo", nil }
	doctorResolveWorkflowConfigPathFn = func(root, workflow string) (string, error) {
		if root != "/repo" || workflow != "run" {
			t.Fatalf("unexpected workflow resolution args %q %q", root, workflow)
		}
		return "/repo/templates/run/config.yml", nil
	}
	doctorGetwdFn = func() (string, error) { return root, nil }
	doctorLoadProjectConfigFn = func(repoRoot string) (*domain.DockpipeProjectConfig, error) {
		if repoRoot != root {
			t.Fatalf("unexpected repo root %q", repoRoot)
		}
		return &domain.DockpipeProjectConfig{
			Secrets: domain.DockpipeSecretsConfig{
				Vault:         &vault,
				VaultTemplate: &templatePath,
				Notes:         &notes,
			},
		}, nil
	}
	opLookPathFn = func(file string) (string, error) {
		if file != "op" {
			t.Fatalf("unexpected op lookpath target %q", file)
		}
		return "/usr/bin/op", nil
	}

	stderr, err := captureResultStderr(t, func() error {
		return cmdDoctor(nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=doctor.bash",
		"status=done",
		"tool=bash",
		"unit=doctor.docker",
		"result=reachable",
		"unit=doctor.assets",
		"workflow_config=/repo/templates/run/config.yml",
		"unit=doctor.project_config",
		"vault_default=op",
		"notes=present",
		"unit=doctor.vault_template",
		"template=" + templatePath,
		"unit=doctor.vault_cli",
		"tool=op",
		"unit=doctor.summary",
		"required_checks=passed",
		"required_failures=0",
		"optional_issues=0",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdDoctorReturnsRequiredFailuresAndLogsOptionalIssues(t *testing.T) {
	withDoctorSeams(t)
	root := t.TempDir()
	doctorBashLookPathFn = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	doctorDockerCheckFn = func(stderr *os.File) error {
		return fmt.Errorf("docker daemon unreachable")
	}
	doctorRepoRootFn = func() (string, error) {
		return "", fmt.Errorf("repo root unavailable")
	}
	doctorGetwdFn = func() (string, error) { return root, nil }
	doctorLoadProjectConfigFn = func(repoRoot string) (*domain.DockpipeProjectConfig, error) {
		return nil, nil
	}

	stderr, err := captureResultStderr(t, func() error {
		return cmdDoctor(nil)
	})
	if err == nil {
		t.Fatal("expected doctor failure")
	}
	for _, want := range []string{
		"bash not found in PATH",
		"docker daemon unreachable",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
	for _, want := range []string{
		"unit=doctor.bash",
		"status=fail",
		"error=\"bash not found in PATH\"",
		"unit=doctor.docker",
		"error=\"docker daemon unreachable\"",
		"unit=doctor.assets",
		"required=false",
		"error=\"repo root unavailable\"",
		"unit=doctor.project_config",
		"result=missing",
		"unit=doctor.summary",
		"status=fail",
		"required_checks=failed",
		"required_failures=2",
		"optional_issues=1",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}
