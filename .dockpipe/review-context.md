# Review context (DockPipe prep bundle)
Generated for final resolver review — do not re-enumerate the whole repo from scratch.

## Workflow flags (trust these)
- WORKFLOW_NAME=test-demo
- PREPARE_OK=1
- TESTS_PASS=1 TESTS_EXIT=0
- SCAN_PASS=1 VET_EXIT=0
- REVIEW_PREP_OK=1
- LOCAL_MODEL_STATUS=

## Go files (first 80 paths; full list in review-files.txt on disk)
.dockpipe/demo/main.go
cmd/dockpipe/main.go
embed.go
lib/dockpipe/application/build_step_container_test.go
lib/dockpipe/application/crosscut_run_test.go
lib/dockpipe/application/doctor.go
lib/dockpipe/application/flags.go
lib/dockpipe/application/flags_test.go
lib/dockpipe/application/host_bash.go
lib/dockpipe/application/host_spinner_msg.go
lib/dockpipe/application/init_structure_test.go
lib/dockpipe/application/init_workflow.go
lib/dockpipe/application/repo_root_test.go
lib/dockpipe/application/resolver_docker_env.go
lib/dockpipe/application/resolver_docker_env_test.go
lib/dockpipe/application/resolver_workflow.go
lib/dockpipe/application/resolver_workflow_test.go
lib/dockpipe/application/run.go
lib/dockpipe/application/run_steps.go
lib/dockpipe/application/run_steps_more_test.go
lib/dockpipe/application/run_steps_seams_test.go
lib/dockpipe/application/run_steps_test.go
lib/dockpipe/application/run_subcmds_test.go
lib/dockpipe/application/run_test.go
lib/dockpipe/application/runtime.go
lib/dockpipe/application/runtime_test.go
lib/dockpipe/application/strategy.go
lib/dockpipe/application/strategy_test.go
lib/dockpipe/application/subcmds.go
lib/dockpipe/application/usage.go
lib/dockpipe/application/windows.go
lib/dockpipe/application/windows_bridge.go
lib/dockpipe/application/windows_bridge_argv.go
lib/dockpipe/application/windows_bridge_argv_test.go
lib/dockpipe/application/windows_bridge_test.go
lib/dockpipe/application/windows_test.go
lib/dockpipe/application/workflow_cmd.go
lib/dockpipe/application/workflow_env.go
lib/dockpipe/application/workflow_env_more_test.go
lib/dockpipe/application/workflow_env_test.go
lib/dockpipe/application/workflow_log.go
lib/dockpipe/application/workflow_log_test.go
lib/dockpipe/application/worktree_docker_env.go
lib/dockpipe/application/worktree_docker_env_test.go
lib/dockpipe/domain/branchslug.go
lib/dockpipe/domain/branchslug_test.go
lib/dockpipe/domain/env.go
lib/dockpipe/domain/env_resolver_test.go
lib/dockpipe/domain/resolver.go
lib/dockpipe/domain/resolver_test.go
lib/dockpipe/domain/runtime_kind.go
lib/dockpipe/domain/strategy.go
lib/dockpipe/domain/strategy_test.go
lib/dockpipe/domain/workflow.go
lib/dockpipe/domain/workflow_helpers_test.go
lib/dockpipe/domain/workflow_imports.go
lib/dockpipe/domain/workflow_test.go
lib/dockpipe/infrastructure/bundled_extract.go
lib/dockpipe/infrastructure/bundled_extract_test.go
lib/dockpipe/infrastructure/commit.go
lib/dockpipe/infrastructure/config.go
lib/dockpipe/infrastructure/config_test.go
lib/dockpipe/infrastructure/docker.go
lib/dockpipe/infrastructure/docker_preflight.go
lib/dockpipe/infrastructure/docker_preflight_test.go
lib/dockpipe/infrastructure/docker_run_test.go
lib/dockpipe/infrastructure/docker_test.go
lib/dockpipe/infrastructure/docker_user.go
lib/dockpipe/infrastructure/docker_user_test.go
lib/dockpipe/infrastructure/envfile.go
lib/dockpipe/infrastructure/envfile_test.go
lib/dockpipe/infrastructure/git_remote.go
lib/dockpipe/infrastructure/git_remote_more_test.go
lib/dockpipe/infrastructure/git_remote_test.go
lib/dockpipe/infrastructure/hostpath_git.go
lib/dockpipe/infrastructure/hostpath_git_test.go
lib/dockpipe/infrastructure/isolation_profile.go
lib/dockpipe/infrastructure/isolation_profile_test.go
lib/dockpipe/infrastructure/layout.go
lib/dockpipe/infrastructure/paths.go

## Signals (preview; full bounded grep in review-signals.txt on disk)
## Bounded pattern hits (grep, capped per pattern)
### exec\.Command
./lib/dockpipe/infrastructure/commit.go:18:	check := exec.Command("git", "-C", wd, "rev-parse", "--is-inside-work-tree")
./lib/dockpipe/infrastructure/commit.go:23:	st := exec.Command("git", "-C", wd, "status", "--porcelain")
./lib/dockpipe/infrastructure/commit.go:29:	br := exec.Command("git", "-C", wd, "branch", "--show-current")
./lib/dockpipe/infrastructure/commit.go:32:	add := exec.Command("git", "-C", wd, "add", "-A")
./lib/dockpipe/infrastructure/commit.go:40:	cmt := exec.Command("git", "-C", wd, "commit", "-m", msg)
./lib/dockpipe/infrastructure/commit.go:56:		b := exec.Command("git", gitArgs...)
./lib/dockpipe/infrastructure/prescript.go:50:	cmd := exec.Command(bashExe, wrapperForBash)
./lib/dockpipe/infrastructure/prescript.go:77:	cmd := exec.Command(bashExe, bashPath)
./lib/dockpipe/infrastructure/docker.go:99:	execCommandFn      = exec.Command
./lib/dockpipe/infrastructure/docker_preflight.go:72:// execCommandContextFn is exec.CommandContext for tests.
./lib/dockpipe/infrastructure/docker_preflight.go:74:	return exec.CommandContext(ctx, name, arg...)
./lib/dockpipe/infrastructure/git_remote_test.go:14:	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
./lib/dockpipe/infrastructure/git_remote_test.go:36:	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
./lib/dockpipe/infrastructure/git_remote_test.go:40:	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
./lib/dockpipe/infrastructure/docker_run_test.go:100:		return exec.Command("bash", "-c", "exit 0")
./lib/dockpipe/infrastructure/docker_run_test.go:171:			return exec.Command("bash", "-c", "exit 7")
./lib/dockpipe/infrastructure/docker_run_test.go:174:			return exec.Command("bash", "-c", "echo line1")
./lib/dockpipe/infrastructure/docker_run_test.go:176:		return exec.Command("bash", "-c", "exit 0")
./lib/dockpipe/infrastructure/docker_run_test.go:214:		return exec.Command("bash", "-c", "exit 0")
./lib/dockpipe/infrastructure/docker_run_test.go:261:			return exec.Command("bash", "-c", "exit 3")
./lib/dockpipe/infrastructure/docker_run_test.go:263:		return exec.Command("bash", "-c", "exit 0")
./lib/dockpipe/infrastructure/docker_run_test.go:342:			return exec.Command("bash", "-c", "exit 1")
./lib/dockpipe/infrastructure/docker_run_test.go:344:		return exec.Command("bash", "-c", "exit 0")
./lib/dockpipe/infrastructure/git_remote.go:25:	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
./lib/dockpipe/infrastructure/git_remote.go:58:	cmd := exec.Command("git", "-C", d, "rev-parse", rev)

### os/exec
./lib/dockpipe/infrastructure/commit.go:6:	"os/exec"
./lib/dockpipe/infrastructure/prescript.go:7:	"os/exec"
./lib/dockpipe/infrastructure/docker.go:10:	"os/exec"
./lib/dockpipe/infrastructure/docker_preflight.go:7:	"os/exec"
./lib/dockpipe/infrastructure/git_remote_test.go:5:	"os/exec"
./lib/dockpipe/infrastructure/docker_run_test.go:6:	"os/exec"
./lib/dockpipe/infrastructure/git_remote.go:5:	"os/exec"
./lib/dockpipe/infrastructure/repo_commit_prescript_resolver_test.go:5:	"os/exec"
./lib/dockpipe/application/windows.go:9:	"os/exec"
./lib/dockpipe/application/doctor.go:8:	"os/exec"
./lib/dockpipe/application/host_bash.go:8:	"os/exec"
./lib/dockpipe/application/windows_bridge.go:7:	"os/exec"
./lib/dockpipe/application/worktree_docker_env_test.go:5:	"os/exec"
./lib/dockpipe/application/run_test.go:5:	"os/exec"
./lib/dockpipe/application/run.go:6:	"os/exec"

### ioutil\.

### Deprecated:

### TODO

### FIXME

### unsafe\.

