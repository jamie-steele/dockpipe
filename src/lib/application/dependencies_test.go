package application

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestCheckWorkflowHostDependenciesReportsMissingRequiredDependency(t *testing.T) {
	wf := &domain.Workflow{
		Dependencies: domain.DependencySpec{
			Host: []domain.HostDependency{{
				ID:          "missing-tool",
				Command:     "dockpipe-definitely-missing-tool",
				Description: "Used by a test-only workflow",
				Install: domain.HostDependencyInstallHint{
					Windows: "winget install example.missing-tool",
					MacOS:   "brew install missing-tool",
					Linux:   "Install missing-tool from your package manager.",
				},
			}},
		},
	}

	wfRoot := t.TempDir()
	err := checkWorkflowHostDependencies(wf, wfRoot, filepath.Join(wfRoot, "config.yml"), nil)
	if err == nil {
		t.Fatal("expected missing host dependency error")
	}
	msg := err.Error()
	for _, want := range []string{
		"missing required host dependencies",
		"missing-tool",
		"dockpipe-definitely-missing-tool",
		"Used by a test-only workflow",
		installHintForMissingToolTestOS(),
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected error to contain %q, got:\n%s", want, msg)
		}
	}
}

func TestCheckWorkflowHostDependenciesIgnoresOptionalMissingDependency(t *testing.T) {
	required := false
	wf := &domain.Workflow{
		Dependencies: domain.DependencySpec{
			Host: []domain.HostDependency{{
				ID:       "optional-tool",
				Command:  "dockpipe-definitely-missing-optional-tool",
				Required: &required,
			}},
		},
	}

	if err := checkWorkflowHostDependencies(wf, t.TempDir(), filepath.Join(t.TempDir(), "config.yml"), nil); err != nil {
		t.Fatalf("optional missing dependency should not fail: %v", err)
	}
}

func TestCheckWorkflowHostDependenciesIncludesNearestPackageManifest(t *testing.T) {
	root := t.TempDir()
	wfRoot := filepath.Join(root, "workflows", "ci")
	if err := os.MkdirAll(wfRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	pm := `schema: 1
name: ci-pack
version: 1.0.0
title: CI Pack
description: CI workflows
author: DockPipe
website: https://example.com
license: Apache-2.0
kind: workflow
dependencies:
  host:
    - id: act
      command: dockpipe-definitely-missing-act
      install:
        windows: winget install nektos.act
        macos: brew install act
        linux: Install nektos/act from your package manager.
`
	if err := os.WriteFile(filepath.Join(root, "package.yml"), []byte(pm), 0o644); err != nil {
		t.Fatal(err)
	}

	err := checkWorkflowHostDependencies(&domain.Workflow{}, wfRoot, filepath.Join(wfRoot, "config.yml"), nil)
	if err == nil {
		t.Fatal("expected package-level missing dependency error")
	}
	if msg := err.Error(); !strings.Contains(msg, "act") || !strings.Contains(msg, installHintForActTestOS()) {
		t.Fatalf("expected package dependency and install hint, got:\n%s", msg)
	}
}

func TestCheckWorkflowHostDependenciesInstallsApprovedMissingDependency(t *testing.T) {
	oldLookPath := dependencyLookPathFn
	oldPowerShell := dependencyPowerShellLookupFn
	oldRunShell := dependencyRunShellFn
	t.Cleanup(func() {
		dependencyLookPathFn = oldLookPath
		dependencyPowerShellLookupFn = oldPowerShell
		dependencyRunShellFn = oldRunShell
	})
	installed := false
	dependencyLookPathFn = func(file string) (string, error) {
		if installed {
			return "/bin/" + file, nil
		}
		return "", os.ErrNotExist
	}
	dependencyPowerShellLookupFn = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	var ran string
	dependencyRunShellFn = func(command string) error {
		ran = command
		installed = true
		return nil
	}
	wf := &domain.Workflow{
		Dependencies: domain.DependencySpec{
			Host: []domain.HostDependency{{
				ID:      "missing-tool",
				Command: "dockpipe-approved-missing-tool",
				Install: domain.HostDependencyInstallHint{
					Windows: "winget install example.missing-tool",
					MacOS:   "brew install missing-tool",
					Linux:   "sudo apt-get install -y missing-tool",
				},
			}},
		},
	}

	wfRoot := t.TempDir()
	err := checkWorkflowHostDependencies(wf, wfRoot, filepath.Join(wfRoot, "config.yml"), &CliOpts{ApproveSystemChanges: true})
	if err != nil {
		t.Fatalf("expected approved install to satisfy dependency: %v", err)
	}
	if ran == "" {
		t.Fatal("expected install command to run")
	}
}

func TestResolveDependencyCommandPathFallsBackToPowerShellOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific fallback")
	}
	oldLookPath := dependencyLookPathFn
	oldPowerShell := dependencyPowerShellLookupFn
	t.Cleanup(func() {
		dependencyLookPathFn = oldLookPath
		dependencyPowerShellLookupFn = oldPowerShell
	})
	dependencyLookPathFn = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	dependencyPowerShellLookupFn = func(command string) (string, error) {
		if command != "act" {
			t.Fatalf("unexpected command %q", command)
		}
		return `C:\Users\Jamie\AppData\Local\Microsoft\WinGet\Links\act.exe`, nil
	}

	got, err := resolveDependencyCommandPath("act")
	if err != nil {
		t.Fatalf("expected PowerShell fallback to resolve command: %v", err)
	}
	if !strings.EqualFold(got, `C:\Users\Jamie\AppData\Local\Microsoft\WinGet\Links\act.exe`) {
		t.Fatalf("unexpected resolved path %q", got)
	}
}

func TestCheckWorkflowHostDependenciesDoesNotInstallUnsupportedPlatform(t *testing.T) {
	oldRunShell := dependencyRunShellFn
	t.Cleanup(func() {
		dependencyRunShellFn = oldRunShell
	})
	dependencyRunShellFn = func(command string) error {
		t.Fatalf("installer should not run on unsupported platform: %s", command)
		return nil
	}
	wf := &domain.Workflow{
		Platforms: []string{unsupportedPlatformForTest()},
		Dependencies: domain.DependencySpec{
			Host: []domain.HostDependency{{
				ID:      "missing-tool",
				Command: "dockpipe-definitely-missing-platform-tool",
				Install: domain.HostDependencyInstallHint{
					Windows: "winget install example.missing-tool",
				},
			}},
		},
	}

	wfRoot := t.TempDir()
	err := checkWorkflowHostDependencies(wf, wfRoot, filepath.Join(wfRoot, "config.yml"), &CliOpts{ApproveSystemChanges: true})
	if err == nil || !strings.Contains(err.Error(), "workflow does not support host platform") {
		t.Fatalf("expected unsupported platform error, got %v", err)
	}
}

func TestCheckWorkflowHostDependenciesBlocksUnsupportedPackagePlatform(t *testing.T) {
	root := t.TempDir()
	wfRoot := filepath.Join(root, "workflows", "ci")
	if err := os.MkdirAll(wfRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	pm := `schema: 1
name: ci-pack
version: 1.0.0
title: CI Pack
description: CI workflows
author: DockPipe
website: https://example.com
license: Apache-2.0
kind: workflow
platforms: [` + unsupportedPlatformForTest() + `]
`
	if err := os.WriteFile(filepath.Join(root, "package.yml"), []byte(pm), 0o644); err != nil {
		t.Fatal(err)
	}

	err := checkWorkflowHostDependencies(&domain.Workflow{}, wfRoot, filepath.Join(wfRoot, "config.yml"), nil)
	if err == nil || !strings.Contains(err.Error(), "package does not support host platform") {
		t.Fatalf("expected unsupported package platform error, got %v", err)
	}
}

func unsupportedPlatformForTest() string {
	switch currentDependencyPlatform() {
	case "windows":
		return "deb"
	default:
		return "windows"
	}
}

func installHintForMissingToolTestOS() string {
	switch runtime.GOOS {
	case "windows":
		return "winget install"
	case "darwin":
		return "brew install"
	default:
		return "Install missing-tool"
	}
}

func installHintForActTestOS() string {
	switch runtime.GOOS {
	case "windows":
		return "winget install"
	case "darwin":
		return "brew install"
	default:
		return "Install nektos/act"
	}
}
