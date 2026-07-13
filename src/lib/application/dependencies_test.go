package application

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
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

func TestCheckWorkflowHostDependenciesApprovedInstallEmitsOperationResults(t *testing.T) {
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
	dependencyRunShellFn = func(command string) error {
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
	stderr, err := captureResultStderr(t, func() error {
		return checkWorkflowHostDependencies(wf, wfRoot, filepath.Join(wfRoot, "config.yml"), &CliOpts{ApproveSystemChanges: true})
	})
	if err != nil {
		t.Fatalf("expected approved install to satisfy dependency: %v", err)
	}
	for _, want := range []string{
		"unit=dependency.host.approval",
		"approval=approved",
		"approval_source=flag",
		"dependency=missing-tool",
		"command=dockpipe-approved-missing-tool",
		"unit=dependency.host.install",
		"status=start",
		"status=done",
		"result=installed",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCheckWorkflowHostDependenciesEmitsPreflightOperationResults(t *testing.T) {
	oldLookPath := dependencyLookPathFn
	oldPowerShell := dependencyPowerShellLookupFn
	t.Cleanup(func() {
		dependencyLookPathFn = oldLookPath
		dependencyPowerShellLookupFn = oldPowerShell
	})
	dependencyLookPathFn = func(file string) (string, error) {
		if file == "dockpipe-present-tool" {
			return "/bin/" + file, nil
		}
		return "", os.ErrNotExist
	}
	dependencyPowerShellLookupFn = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	optional := false
	wf := &domain.Workflow{
		Dependencies: domain.DependencySpec{
			Host: []domain.HostDependency{
				{ID: "present-tool", Command: "dockpipe-present-tool"},
				{ID: "optional-tool", Command: "dockpipe-optional-missing-tool", Required: &optional},
				{ID: "missing-tool", Command: "dockpipe-required-missing-tool"},
			},
		},
	}

	wfRoot := t.TempDir()
	stderr, err := captureResultStderr(t, func() error {
		return checkWorkflowHostDependencies(wf, wfRoot, filepath.Join(wfRoot, "config.yml"), nil)
	})
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	for _, want := range []string{
		"unit=dependency.host.preflight",
		"dependency=present-tool",
		"result=found",
		"dependency=optional-tool",
		"required=false",
		"skip_reason=optional_missing",
		"dependency=missing-tool",
		"status=fail",
		"result=missing",
		"installer_present=false",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCheckWorkflowHostDependenciesPreflightMirrorsOperationEvents(t *testing.T) {
	oldLookPath := dependencyLookPathFn
	oldPowerShell := dependencyPowerShellLookupFn
	t.Cleanup(func() {
		dependencyLookPathFn = oldLookPath
		dependencyPowerShellLookupFn = oldPowerShell
	})
	dependencyLookPathFn = func(file string) (string, error) {
		return "", os.ErrNotExist
	}
	dependencyPowerShellLookupFn = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	wf := &domain.Workflow{
		Dependencies: domain.DependencySpec{
			Host: []domain.HostDependency{{
				ID:      "missing-tool",
				Command: "dockpipe-event-missing-tool",
				Install: domain.HostDependencyInstallHint{
					Windows: "winget install example.missing-tool",
					MacOS:   "brew install missing-tool",
					Linux:   "sudo apt-get install -y missing-tool",
				},
			}},
		},
	}
	wfRoot := t.TempDir()
	eventLog := filepath.Join(wfRoot, "events.jsonl")
	t.Setenv(infrastructure.EnvDockpipeEventLog, eventLog)

	if _, err := captureResultStderr(t, func() error {
		return checkWorkflowHostDependencies(wf, wfRoot, filepath.Join(wfRoot, "config.yml"), nil)
	}); err == nil {
		t.Fatal("expected missing dependency error")
	}
	events, err := infrastructure.ReadOperationEvents(eventLog)
	if err != nil {
		t.Fatalf("ReadOperationEvents: %v", err)
	}
	if len(events) < 1 {
		t.Fatalf("expected mirrored preflight events, got %#v", events)
	}
	preflight := events[0]
	if preflight.Schema != infrastructure.OperationEventSchemaV1 || preflight.Type != infrastructure.OperationEventKind {
		t.Fatalf("unexpected preflight event envelope: %#v", preflight)
	}
	if preflight.Unit != "dependency.host.preflight" || preflight.Status != infrastructure.OperationStatusFail {
		t.Fatalf("unexpected preflight event status: %#v", preflight)
	}
	for key, want := range map[string]string{
		"dependency":        "missing-tool",
		"command":           "dockpipe-event-missing-tool",
		"result":            "missing",
		"required":          "true",
		"installer_present": "true",
	} {
		if got := preflight.IDs[key]; got != want {
			t.Fatalf("preflight event ID %s = %q want %q (event: %#v)", key, got, want, preflight)
		}
	}
}
func TestCheckWorkflowHostDependenciesNonInteractiveDeclineEmitsApprovalResult(t *testing.T) {
	oldLookPath := dependencyLookPathFn
	oldPowerShell := dependencyPowerShellLookupFn
	oldRunShell := dependencyRunShellFn
	t.Cleanup(func() {
		dependencyLookPathFn = oldLookPath
		dependencyPowerShellLookupFn = oldPowerShell
		dependencyRunShellFn = oldRunShell
	})
	dependencyLookPathFn = func(file string) (string, error) {
		return "", os.ErrNotExist
	}
	dependencyPowerShellLookupFn = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	dependencyRunShellFn = func(command string) error {
		t.Fatalf("installer should not run without approval: %s", command)
		return nil
	}
	wf := &domain.Workflow{
		Dependencies: domain.DependencySpec{
			Host: []domain.HostDependency{{
				ID:      "missing-tool",
				Command: "dockpipe-noninteractive-missing-tool",
				Install: domain.HostDependencyInstallHint{
					Windows: "winget install example.missing-tool",
					MacOS:   "brew install missing-tool",
					Linux:   "sudo apt-get install -y missing-tool",
				},
			}},
		},
	}

	wfRoot := t.TempDir()
	stderr, err := captureResultStderr(t, func() error {
		return checkWorkflowHostDependencies(wf, wfRoot, filepath.Join(wfRoot, "config.yml"), nil)
	})
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	if !strings.Contains(stderr, "unit=dependency.host.approval") ||
		!strings.Contains(stderr, "approval=declined") ||
		!strings.Contains(stderr, "approval_source=non_interactive") {
		t.Fatalf("expected non-interactive approval result, got:\n%s", stderr)
	}
	if strings.Contains(stderr, "unit=dependency.host.install") {
		t.Fatalf("did not expect install unit without approval, got:\n%s", stderr)
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
