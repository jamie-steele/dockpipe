package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

// TestBuildStepContainer_UsesCliArgsForLastStep uses argv after -- when the last step has no cmd in YAML.
func TestBuildStepContainer_UsesCliArgsForLastStep(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot:    repoRoot,
		wfRoot:      filepath.Join(repoRoot, "templates", "test"),
		wf:          &domain.Workflow{Isolate: "base-dev"},
		cliArgs:     []string{"echo", "from-cli"},
		opts:        &CliOpts{},
		resolver:    "",
		userIsolate: "",
	}
	envMap := map[string]string{}
	dockerEnv := map[string]string{}
	step := domain.Step{} // no cmd
	argv, runOpts, buildDir, buildCtx, _, err := buildStepContainer(o, 0, 1, step, envMap, dockerEnv, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(argv) != 2 || argv[0] != "echo" || argv[1] != "from-cli" {
		t.Fatalf("expected cli args fallback, got %v", argv)
	}
	if runOpts.Image == "" || buildDir == "" || buildCtx == "" {
		t.Fatalf("expected resolved image/build paths, got image=%q buildDir=%q buildCtx=%q", runOpts.Image, buildDir, buildCtx)
	}
}

// TestBuildStepContainer_ErrorsWhenActionMissing when act script path does not exist.
func TestBuildStepContainer_ErrorsWhenActionMissing(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	step := domain.Step{Cmd: "echo hi", Action: "scripts/does-not-exist.sh"}
	_, _, _, _, _, err := buildStepContainer(o, 0, 1, step, map[string]string{}, map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), "action script not found") {
		t.Fatalf("expected missing action error, got %v", err)
	}
}

// TestBuildStepContainer_CommitWorktreeTurnsIntoHostCommit maps bundled commit-worktree to CommitOnHost instead of in-container act.
func TestBuildStepContainer_CommitWorktreeTurnsIntoHostCommit(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	envMap := map[string]string{}
	step := domain.Step{Cmd: "echo hi", Action: "scripts/commit-worktree.sh"}
	_, runOpts, _, _, _, err := buildStepContainer(o, 0, 1, step, envMap, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !runOpts.CommitOnHost {
		t.Fatalf("expected CommitOnHost=true")
	}
	if runOpts.ActionPath != "" {
		t.Fatalf("expected ActionPath cleared for host commit, got %q", runOpts.ActionPath)
	}
	if envMap["DOCKPIPE_BRANCH_PREFIX"] == "" {
		t.Fatalf("expected branch prefix to be set for host commit path")
	}
}

// TestBuildStepContainer_ForwardsResolverEnvHintFromHost copies OPENAI_API_KEY from step env into docker ExtraEnv when profile lists DOCKPIPE_RESOLVER_ENV.
func TestBuildStepContainer_ForwardsResolverEnvHintFromHost(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	step := domain.Step{Cmd: "echo hi"}
	envMap := map[string]string{"OPENAI_API_KEY": "sk-test"}
	dockerEnv := map[string]string{}
	ra := &domain.ResolverAssignments{EnvHint: "OPENAI_API_KEY"}
	_, runOpts, _, _, _, err := buildStepContainer(o, 0, 1, step, envMap, dockerEnv, ra)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	em := domain.EnvSliceToMap(runOpts.ExtraEnv)
	if em["OPENAI_API_KEY"] != "sk-test" {
		t.Fatalf("expected OPENAI forwarded to docker env, got %#v", em)
	}
}

// TestBuildStepContainer_ForwardsMultipleResolverEnvHints copies every name in DOCKPIPE_RESOLVER_ENV (comma list) from host env into docker ExtraEnv.
func TestBuildStepContainer_ForwardsMultipleResolverEnvHints(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	step := domain.Step{Cmd: "echo hi"}
	envMap := map[string]string{"ANTHROPIC_API_KEY": "sk-ant", "CLAUDE_API_KEY": "ck-local"}
	dockerEnv := map[string]string{}
	ra := &domain.ResolverAssignments{EnvHint: "ANTHROPIC_API_KEY,CLAUDE_API_KEY"}
	_, runOpts, _, _, _, err := buildStepContainer(o, 0, 1, step, envMap, dockerEnv, ra)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	em := domain.EnvSliceToMap(runOpts.ExtraEnv)
	if em["ANTHROPIC_API_KEY"] != "sk-ant" || em["CLAUDE_API_KEY"] != "ck-local" {
		t.Fatalf("expected both resolver env hints forwarded, got %#v", em)
	}
}

// TestBuildStepContainer_ForwardsPolicyProxyEnv copies DockPipe policy proxy settings from
// the resolved workflow environment into the container env so compose exports can drive
// proxy-backed network enforcement on later steps.
func TestBuildStepContainer_ForwardsPolicyProxyEnv(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	step := domain.Step{Cmd: "echo hi"}
	envMap := map[string]string{
		"DOCKPIPE_POLICY_PROXY_URL":      "http://proxy-sidecar:8080",
		"DOCKPIPE_POLICY_PROXY_NO_PROXY": "metadata.local",
	}
	dockerEnv := map[string]string{}
	_, runOpts, _, _, _, err := buildStepContainer(o, 0, 1, step, envMap, dockerEnv, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	em := domain.EnvSliceToMap(runOpts.ExtraEnv)
	if em["DOCKPIPE_POLICY_PROXY_URL"] != "http://proxy-sidecar:8080" {
		t.Fatalf("expected policy proxy URL forwarded, got %#v", em)
	}
	if em["DOCKPIPE_POLICY_PROXY_NO_PROXY"] != "metadata.local" {
		t.Fatalf("expected policy no_proxy forwarded, got %#v", em)
	}
}

// TestBuildStepContainer_StepResolverTemplate uses DOCKPIPE_RESOLVER_TEMPLATE from a per-step resolver assignment.
func TestBuildStepContainer_StepResolverTemplate(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{},
		opts:     &CliOpts{},
	}
	step := domain.Step{Cmd: "echo hi"}
	ra := &domain.ResolverAssignments{Template: "vscode"}
	_, runOpts, _, _, _, err := buildStepContainer(o, 0, 1, step, map[string]string{}, map[string]string{}, ra)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(runOpts.Image, "vscode") {
		t.Fatalf("expected vscode image from resolver template, got %q", runOpts.Image)
	}
}

func TestBuildStepContainer_AppliesStepSecurityOverride(t *testing.T) {
	repoRoot := testRepoRoot(t)
	wfRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wfRoot, domain.RuntimeManifestDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	rm := &domain.CompiledRuntimeManifest{
		Schema:            2,
		Kind:              domain.RuntimeManifestKind,
		PolicyProfile:     "secure-default",
		PolicySources:     domain.PolicySources{EngineDefault: true, RuntimeBaseline: "dockerimage", PolicyProfile: "secure-default"},
		PolicyFingerprint: "sha256:test",
		Security: domain.CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "offline",
				Enforcement: "native",
				InternalDNS: true,
			},
			FS: domain.CompiledFilesystemPolicy{
				Root:      "readonly",
				Writes:    "workspace-only",
				TempPaths: []string{"/tmp"},
			},
			Process: domain.CompiledProcessPolicy{
				User:            "non-root",
				NoNewPrivileges: true,
				DropCaps:        []string{"ALL"},
				PIDLimit:        256,
			},
		},
	}
	b, err := marshalArtifactJSON(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.RuntimeManifestFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}

	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   wfRoot,
		wfConfig: filepath.Join(wfRoot, "config.yml"),
		wf: &domain.Workflow{
			Isolate: "base-dev",
			Security: domain.WorkflowSecurityConfig{
				Profile: "secure-default",
			},
		},
		opts: &CliOpts{},
	}
	step := domain.Step{
		Cmd: "echo hi",
		Security: domain.WorkflowSecurityConfig{
			Profile: "internet-client",
			Process: domain.WorkflowProcessConfig{
				PIDLimit: 64,
			},
		},
	}
	_, runOpts, _, _, _, err := buildStepContainer(o, 0, 1, step, map[string]string{}, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if runOpts.NetworkMode == "none" {
		t.Fatalf("expected step override to lift offline network mode, got %q", runOpts.NetworkMode)
	}
	if runOpts.PIDLimit != 64 {
		t.Fatalf("expected step pid limit override, got %d", runOpts.PIDLimit)
	}
}

func TestBuildStepContainer_PrefersCompiledStepManifest(t *testing.T) {
	repoRoot := testRepoRoot(t)
	wfRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.StepArtifactsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	workflowRM := &domain.CompiledRuntimeManifest{
		Schema:            2,
		Kind:              domain.RuntimeManifestKind,
		PolicyProfile:     "secure-default",
		PolicyFingerprint: "sha256:wf",
		Security: domain.CompiledSecurityPolicy{
			Preset: "secure-default",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "offline",
				Enforcement: "native",
				InternalDNS: true,
			},
		},
	}
	wb, _ := marshalArtifactJSON(workflowRM)
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.RuntimeManifestFileName), wb, 0o644); err != nil {
		t.Fatal(err)
	}
	stepRM := &domain.CompiledRuntimeManifest{
		Schema:            2,
		Kind:              domain.RuntimeManifestKind,
		StepID:            "fetch",
		PolicyProfile:     "internet-client",
		PolicyFingerprint: "sha256:step",
		PolicySources:     domain.PolicySources{StepOverride: true},
		Security: domain.CompiledSecurityPolicy{
			Preset: "internet-client",
			Network: domain.CompiledNetworkPolicy{
				Mode:        "internet",
				Enforcement: "native",
				InternalDNS: true,
			},
			Process: domain.CompiledProcessPolicy{
				PIDLimit: 64,
			},
		},
	}
	sb, _ := marshalArtifactJSON(stepRM)
	if err := os.WriteFile(filepath.Join(wfRoot, domain.RuntimeManifestDirName, domain.RuntimeManifestPathForStep("fetch")), sb, 0o644); err != nil {
		t.Fatal(err)
	}
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   wfRoot,
		wfConfig: filepath.Join(wfRoot, "config.yml"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	step := domain.Step{ID: "fetch", Cmd: "echo hi"}
	_, runOpts, _, _, rm, err := buildStepContainer(o, 0, 1, step, map[string]string{}, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if runOpts.NetworkMode == "none" {
		t.Fatalf("expected compiled step manifest to override workflow offline policy")
	}
	if runOpts.PIDLimit != 64 {
		t.Fatalf("expected compiled step pid limit, got %d", runOpts.PIDLimit)
	}
	if rm == nil || rm.StepID != "fetch" || rm.PolicyFingerprint != "sha256:step" {
		t.Fatalf("expected compiled step manifest, got %+v", rm)
	}
}
